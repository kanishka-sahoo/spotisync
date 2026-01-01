package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"spotisync/internal/api/handlers"
	"spotisync/internal/api/middleware"
	"spotisync/internal/auth"
	"spotisync/internal/config"
	"spotisync/internal/db"
	"spotisync/internal/scheduler"
	"spotisync/internal/websocket"
)

// Server represents the API server
type Server struct {
	cfg          *config.Config
	db           *db.Database
	router       *chi.Mux
	authService  *auth.AuthService
	jobScheduler *scheduler.JobScheduler
	hub          *websocket.Hub
	httpServer   *http.Server
}

// NewServer creates a new server instance
func NewServer(cfg *config.Config) (*Server, error) {
	// Initialize database
	database, err := db.New(&cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize JWT manager
	jwtManager := auth.NewJWTManager(
		cfg.Server.SecretKey,
		cfg.Server.TokenTTL,
	)

	// Initialize auth service
	authService := auth.NewAuthService(database, jwtManager)

	// Initialize retry policy
	retryPolicy := scheduler.RetryPolicy{
		MaxRetries: cfg.Workers.RetryMax,
		Delays:     cfg.Workers.RetryDelays,
	}

	// Initialize WebSocket hub (moved up before scheduler creation)
	var hub *websocket.Hub
	if len(cfg.Server.AllowedOrigins) > 0 {
		// Use origin checking in production
		developmentMode := cfg.Server.Env != "production"
		hub = websocket.NewHubWithConfig(cfg.Server.AllowedOrigins, developmentMode)
	} else {
		// No origin checking configured
		hub = websocket.NewHub()
	}
	go hub.Run()

	// Initialize job scheduler with download orchestrator
	jobScheduler := scheduler.NewJobSchedulerWithOrchestrator(database, cfg.Workers.Count, retryPolicy, cfg, hub)

	// Initialize router
	router := chi.NewRouter()

	// Configure CORS
	allowedOrigins := cfg.Server.AllowedOrigins
	if len(allowedOrigins) == 0 {
		// Only allow wildcard in development
		if cfg.Server.Env != "production" {
			allowedOrigins = []string{"*"}
		}
	}
	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Authorization"},
		AllowCredentials: len(allowedOrigins) > 0 && allowedOrigins[0] != "*", // Can't use credentials with wildcard
		MaxAge:           300,
	})
	router.Use(corsMiddleware.Handler)

	// Health check endpoint (public, no auth required)
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Public routes (no auth required)
	router.Post("/api/v1/auth/login", handlers.NewAuthHandler(authService, database).Login)
	router.Post("/api/v1/auth/register", handlers.NewAuthHandler(authService, database).Register)

	// Protected routes (auth required)
	router.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(authService))
		r.Use(middleware.RateLimitMiddleware(cfg.RateLimit.RequestsPerMinute, cfg.RateLimit.Burst))

		// Auth routes
		r.Get("/api/v1/auth/me", handlers.NewAuthHandler(authService, database).Me)

		// Jobs routes
		r.Post("/api/v1/jobs", handlers.NewJobsHandler(database, jobScheduler, jwtManager, cfg).CreateBatch)
		r.Get("/api/v1/jobs", handlers.NewJobsHandler(database, jobScheduler, jwtManager, cfg).ListJobs)
		r.Get("/api/v1/jobs/{id}", handlers.NewJobsHandler(database, jobScheduler, jwtManager, cfg).GetJob)
		r.Post("/api/v1/jobs/{id}/retry", handlers.NewJobsHandler(database, jobScheduler, jwtManager, cfg).RetryJob)
		r.Delete("/api/v1/jobs/{id}", handlers.NewJobsHandler(database, jobScheduler, jwtManager, cfg).CancelJob)

		// Batches routes
		r.Get("/api/v1/batches", handlers.NewBatchesHandler(database, jwtManager, jobScheduler).ListBatches)
		r.Get("/api/v1/batches/{id}", handlers.NewJobsHandler(database, jobScheduler, jwtManager, cfg).GetBatchJobs)
		r.Delete("/api/v1/batches/{id}", handlers.NewBatchesHandler(database, jwtManager, jobScheduler).DeleteBatch)
		r.Post("/api/v1/batches/{id}/retry", handlers.NewBatchesHandler(database, jwtManager, jobScheduler).RetryBatch)

		// Settings routes
		r.Get("/api/v1/settings", handlers.NewSettingsHandler(database, jwtManager, cfg).GetSettings)
		r.Put("/api/v1/settings", handlers.NewSettingsHandler(database, jwtManager, cfg).UpdateSettings)
		r.Post("/api/v1/settings/test-navidrome", handlers.NewSettingsHandler(database, jwtManager, cfg).TestNavidromeConnection)
		r.Get("/api/v1/settings/storage", handlers.NewSettingsHandler(database, jwtManager, cfg).GetStorageSettings)
		r.Put("/api/v1/settings/storage", handlers.NewSettingsHandler(database, jwtManager, cfg).UpdateStorageSettings)
		r.Get("/api/v1/settings/sources", handlers.NewSettingsHandler(database, jwtManager, cfg).GetSourceSettings)
		r.Put("/api/v1/settings/sources", handlers.NewSettingsHandler(database, jwtManager, cfg).UpdateSourceSettings)
		r.Post("/api/v1/settings/test-tidal", handlers.NewSettingsHandler(database, jwtManager, cfg).TestTidalConnection)
		r.Post("/api/v1/settings/test-qobuz", handlers.NewSettingsHandler(database, jwtManager, cfg).TestQobuzConnection)

		// Preview routes
		r.Post("/api/v1/preview", handlers.NewPreviewHandler(cfg).Preview)

		// Scan routes
		r.Post("/api/v1/scan", handlers.NewScanHandler(database, cfg).TriggerScan)

		// Playlist routes
		r.Post("/api/v1/batches/{id}/playlist", handlers.NewPlaylistHandler(database, jwtManager, cfg).CreatePlaylistFromBatch)

		// WebSocket endpoint
		r.Get("/api/v1/ws", func(w http.ResponseWriter, req *http.Request) {
			// Get user ID from context (set by auth middleware)
			userID, ok := middleware.GetUserID(req.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			hub.HandleConnection(w, req, userID)
		})
	})

	return &Server{
		cfg:          cfg,
		db:           database,
		router:       router,
		authService:  authService,
		jobScheduler: jobScheduler,
		hub:          hub,
	}, nil
}

// Start starts the server
func (s *Server) Start() error {
	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start job scheduler
	s.jobScheduler.Start(ctx)

	// Configure HTTP server
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting server on %s", addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Cancel job scheduler context
	cancel()

	// Shutdown HTTP server with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
	return nil
}

// GetHub returns the WebSocket hub
func (s *Server) GetHub() *websocket.Hub {
	return s.hub
}
