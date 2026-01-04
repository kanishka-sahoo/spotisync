package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"spotisync/internal/api/middleware"
	"spotisync/internal/auth"
	"spotisync/internal/db"
	"spotisync/internal/db/models"
	"spotisync/internal/scheduler"
	"spotisync/internal/services/navidrome"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// GetIDFromRoute extracts the ID parameter from chi route context, path value, or URL path
func GetIDFromRoute(r *http.Request) string {
	// Try chi route context first (works in production with chi router)
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if id := rc.URLParam("id"); id != "" {
			return id
		}
	}

	// Try chi's PathValue method
	if id := r.PathValue("id"); id != "" {
		return id
	}

	// Fall back to extracting from URL path (works in tests)
	// Parse the URL path to extract the ID
	// Handle patterns like /api/v1/jobs/{id}/retry or /api/v1/jobs/{id}
	path := r.URL.Path

	// Try to extract ID from patterns like /api/v1/jobs/{id}/action
	// by looking for the pattern /api/v1/jobs/ and extracting the next segment
	if strings.HasPrefix(path, "/api/v1/jobs/") {
		// Remove /api/v1/jobs/ prefix
		remainder := strings.TrimPrefix(path, "/api/v1/jobs/")
		// If there's still content, the ID is the first segment before the next /
		if idx := strings.Index(remainder, "/"); idx >= 0 {
			return remainder[:idx]
		}
		// Otherwise, the entire remainder is the ID
		if remainder != "" {
			return remainder
		}
	}

	// Try to extract ID from patterns like /api/v1/batches/{id}
	if strings.HasPrefix(path, "/api/v1/batches/") {
		remainder := strings.TrimPrefix(path, "/api/v1/batches/")
		if idx := strings.Index(remainder, "/"); idx >= 0 {
			return remainder[:idx]
		}
		if remainder != "" {
			return remainder
		}
	}

	// Fallback to extracting last segment (for simple patterns like /api/v1/jobs/{id})
	if idx := strings.LastIndex(path, "/"); idx >= 0 && idx < len(path)-1 {
		id := path[idx+1:]
		// Remove trailing slash if present
		if len(id) > 0 && id[len(id)-1] == '/' {
			id = id[:len(id)-1]
		}
		if id != "" {
			return id
		}
	}

	return ""
}

// BatchesHandler handles batch-related endpoints
type BatchesHandler struct {
	db             *db.Database
	jwtManager     *auth.JWTManager
	scheduler      *scheduler.JobScheduler
	playlistSyncer *navidrome.Syncer
}

// NewBatchesHandler creates a new batches handler
func NewBatchesHandler(database *db.Database, jwtManager *auth.JWTManager, jobScheduler *scheduler.JobScheduler, playlistSyncer *navidrome.Syncer) *BatchesHandler {
	return &BatchesHandler{
		db:             database,
		jwtManager:     jwtManager,
		scheduler:      jobScheduler,
		playlistSyncer: playlistSyncer,
	}
}

// ListBatches handles GET /api/v1/batches
func (h *BatchesHandler) ListBatches(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	batches, err := h.db.GetBatchesByUserID(userID)
	if err != nil {
		http.Error(w, "failed to get batches: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := make([]*models.BatchResponse, len(batches))
	for i, batch := range batches {
		response[i] = batch.ToResponse()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetBatch handles GET /api/v1/batches/:id
func (h *BatchesHandler) GetBatch(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	batchID := GetIDFromRoute(r)
	if batchID == "" {
		http.Error(w, "batch ID is required", http.StatusBadRequest)
		return
	}

	batch, err := h.db.GetBatchByID(batchID)
	if err != nil {
		http.Error(w, "failed to get batch: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if batch == nil {
		http.Error(w, "batch not found", http.StatusNotFound)
		return
	}

	// Ensure user owns the batch
	if batch.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(batch.ToResponse())
}

// DeleteBatch handles DELETE /api/v1/batches/:id
func (h *BatchesHandler) DeleteBatch(w http.ResponseWriter, r *http.Request) {
	// Get the authenticated user ID from JWT claims by validating the token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "missing authorization header", http.StatusUnauthorized)
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
		return
	}

	claims, err := h.jwtManager.ValidateToken(parts[1])
	if err != nil {
		http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
		return
	}
	authUserID := claims.UserID

	// Get the requested user ID from context
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Verify the authenticated user matches the requested user
	if authUserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	batchID := GetIDFromRoute(r)
	if batchID == "" {
		http.Error(w, "batch ID is required", http.StatusBadRequest)
		return
	}

	batch, err := h.db.GetBatchByID(batchID)
	if err != nil {
		http.Error(w, "failed to get batch: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if batch == nil {
		http.Error(w, "batch not found", http.StatusNotFound)
		return
	}

	// Ensure user owns the batch
	if batch.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Delete batch and its jobs
	if err := h.db.DeleteBatch(batchID); err != nil {
		http.Error(w, "failed to delete batch: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "batch deleted"})
}

// RetryBatch handles POST /api/v1/batches/:id/retry
func (h *BatchesHandler) RetryBatch(w http.ResponseWriter, r *http.Request) {
	// Get the authenticated user ID from JWT claims by validating the token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "missing authorization header", http.StatusUnauthorized)
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
		return
	}

	claims, err := h.jwtManager.ValidateToken(parts[1])
	if err != nil {
		http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
		return
	}
	authUserID := claims.UserID

	// Get the requested user ID from context
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Verify the authenticated user matches the requested user
	if authUserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	batchID := GetIDFromRoute(r)
	if batchID == "" {
		http.Error(w, "batch ID is required", http.StatusBadRequest)
		return
	}

	batch, err := h.db.GetBatchByID(batchID)
	if err != nil {
		http.Error(w, "failed to get batch: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if batch == nil {
		http.Error(w, "batch not found", http.StatusNotFound)
		return
	}

	// Ensure user owns the batch
	if batch.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Get all jobs in the batch
	jobs, err := h.db.GetJobsByBatchID(batchID)
	if err != nil {
		http.Error(w, "failed to get jobs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Retry all failed jobs
	retriedCount := 0
	for _, job := range jobs {
		if job.Status == models.JobStatusFailed {
			if err := h.db.ResetJobForRetry(job.ID); err != nil {
				continue // Skip jobs that fail to reset
			}
			h.scheduler.Enqueue(job.ID, userID)
			retriedCount++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "batch retry queued",
		"retried_count": retriedCount,
	})
}

// ResyncBatch handles POST /api/v1/batches/:id/resync
func (h *BatchesHandler) ResyncBatch(w http.ResponseWriter, r *http.Request) {
	// Get the authenticated user ID from JWT claims by validating the token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "missing authorization header", http.StatusUnauthorized)
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
		return
	}

	claims, err := h.jwtManager.ValidateToken(parts[1])
	if err != nil {
		http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
		return
	}
	authUserID := claims.UserID

	// Get the requested user ID from context
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Verify the authenticated user matches the requested user
	if authUserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	batchID := GetIDFromRoute(r)
	if batchID == "" {
		http.Error(w, "batch ID is required", http.StatusBadRequest)
		return
	}

	// Validate batch ID is a valid UUID format
	if _, err := uuid.Parse(batchID); err != nil {
		http.Error(w, "invalid batch ID format", http.StatusBadRequest)
		return
	}

	batch, err := h.db.GetBatchByID(batchID)
	if err != nil {
		http.Error(w, "failed to get batch: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if batch == nil {
		http.Error(w, "batch not found", http.StatusNotFound)
		return
	}

	// Ensure user owns the batch
	if batch.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Get all incomplete jobs in the batch
	jobs, err := h.db.GetIncompleteJobsByBatchID(batchID)
	if err != nil {
		http.Error(w, "failed to get incomplete jobs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Reset and enqueue each incomplete job
	resyncedCount := 0
	for _, job := range jobs {
		// CRITICAL: Verify job ownership before resetting and enqueueing
		if job.UserID != userID {
			log.Printf("SECURITY VIOLATION: User %d attempted to resync job %s belonging to user %d", userID, job.ID, job.UserID)
			continue // Skip jobs that don't belong to the user
		}
		if err := h.db.ResetJobForRetry(job.ID); err != nil {
			continue // Skip jobs that fail to reset
		}
		h.scheduler.Enqueue(job.ID, userID)
		resyncedCount++
	}

	// Update batch status to processing using atomic update
	// Only update if the batch is still in a state that can be resynced
	if err := h.db.UpdateBatchStatusIfMatching(batchID, models.BatchStatusProcessing, batch.CompletedJobs, batch.FailedJobs, batch.Status); err != nil {
		http.Error(w, "failed to update batch status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":        "batch resync queued",
		"resynced_count": resyncedCount,
		"batch_status":   models.BatchStatusProcessing,
	})
}

// CheckPlaylist handles POST /api/v1/batches/{id}/check-playlist
// Checks if a Navidrome playlist exists for the batch and marks it as completed if found
func (h *BatchesHandler) CheckPlaylist(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	batchID := GetIDFromRoute(r)
	if batchID == "" {
		http.Error(w, "batch ID is required", http.StatusBadRequest)
		return
	}

	// Validate batch ID is a valid UUID format
	if _, err := uuid.Parse(batchID); err != nil {
		http.Error(w, "invalid batch ID format", http.StatusBadRequest)
		return
	}

	batch, err := h.db.GetBatchByID(batchID)
	if err != nil {
		log.Printf("Error getting batch: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if batch == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Ensure user owns the batch
	if batch.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Create a background context with longer timeout for playlist operations
	// Large playlists (60+ tracks) can take several minutes to check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Pass userID through the new context since we're not using r.Context()
	ctx = context.WithValue(ctx, middleware.UserIDKey, userID)

	// Use the playlist syncer to check if playlist exists
	playlist, err := h.playlistSyncer.CheckPlaylist(ctx, batchID)
	if err != nil {
		log.Printf("Error checking playlist: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// If playlist is found, return its details
	if playlist != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"playlist_id":   playlist.ID,
			"playlist_name": playlist.Name,
			"song_count":    playlist.SongCount,
			"duration":      playlist.Duration,
			"found":         true,
			"message":       fmt.Sprintf("Found playlist '%s' with %d tracks", playlist.Name, playlist.SongCount),
		})
		return
	}

	// Playlist not found
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"found":   false,
		"message": "No matching playlist found in Navidrome",
	})
}

// SyncPlaylist handles POST /api/v1/batches/{id}/sync-playlist
// Syncs the batch's completed jobs to a Navidrome playlist
func (h *BatchesHandler) SyncPlaylist(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	batchID := GetIDFromRoute(r)
	if batchID == "" {
		http.Error(w, "batch ID is required", http.StatusBadRequest)
		return
	}

	// Validate batch ID is a valid UUID format
	if _, err := uuid.Parse(batchID); err != nil {
		http.Error(w, "invalid batch ID format", http.StatusBadRequest)
		return
	}

	batch, err := h.db.GetBatchByID(batchID)
	if err != nil {
		log.Printf("Error getting batch: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if batch == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Ensure user owns the batch
	if batch.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Validate batch is a playlist type
	if batch.SpotifyType != models.SpotifyTypePlaylist {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Create a background context with longer timeout for long-running sync operations
	// Large playlists (60+ tracks) can take several minutes to process
	// Each track requires API calls for searching and then adding to playlist
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Pass userID through the new context since we're not using r.Context()
	ctx = context.WithValue(ctx, middleware.UserIDKey, userID)

	// Use the playlist syncer to sync the playlist
	result, err := h.playlistSyncer.SyncPlaylist(ctx, batchID)
	if err != nil {
		log.Printf("Error syncing playlist: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Return the sync result
	w.Header().Set("Content-Type", "application/json")
	status := http.StatusOK
	if !result.Success {
		status = http.StatusInternalServerError
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"playlist_id":   result.PlaylistID,
		"tracks_found":  result.TracksFound,
		"tracks_failed": result.TracksFailed,
		"success":       result.Success,
		"error":         result.Error,
	})
}
