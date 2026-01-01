package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"spotisync/internal/api/middleware"
	"spotisync/internal/auth"
	"spotisync/internal/config"
	"spotisync/internal/db"
	"spotisync/internal/services/navidrome"
	"spotisync/internal/services/qobuz"
	"spotisync/internal/services/tidal"
	"strings"
)

// SettingsRequest represents a request to update settings
type SettingsRequest struct {
	NavidromeURL      *string `json:"navidrome_url,omitempty"`
	NavidromeUsername *string `json:"navidrome_username,omitempty"`
	NavidromePassword *string `json:"navidrome_password,omitempty"`
}

// SettingsResponse represents the response for settings
type SettingsResponse struct {
	NavidromeURL      string `json:"navidrome_url,omitempty"`
	NavidromeUsername string `json:"navidrome_username,omitempty"`
}

// StorageSettingsRequest represents a request to update storage settings
type StorageSettingsRequest struct {
	MusicRoot string `json:"music_root"`
}

// StorageSettingsResponse represents the response for storage settings
type StorageSettingsResponse struct {
	MusicRoot string `json:"music_root"`
}

// TidalSourceSettings represents Tidal source configuration
type TidalSourceSettings struct {
	Configured   bool   `json:"configured"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"` // Only for updates, never returned
	Quality      string `json:"quality"`
}

// QobuzSourceSettings represents Qobuz source configuration
type QobuzSourceSettings struct {
	Configured bool   `json:"configured"`
	AppID      string `json:"app_id"`
	Secret     string `json:"secret,omitempty"` // Only for updates, never returned
	Quality    string `json:"quality"`
}

// SourceSettingsResponse represents the response for source settings
type SourceSettingsResponse struct {
	Tidal           TidalSourceSettings `json:"tidal"`
	Qobuz           QobuzSourceSettings `json:"qobuz"`
	PreferredSource string              `json:"preferred_source"`
}

// SourceSettingsRequest represents a request to update source settings
type SourceSettingsRequest struct {
	Tidal           *TidalSourceSettings `json:"tidal,omitempty"`
	Qobuz           *QobuzSourceSettings `json:"qobuz,omitempty"`
	PreferredSource *string              `json:"preferred_source,omitempty"`
}

// ConnectionTestResponse represents the response for connection tests
type ConnectionTestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// TestNavidromeRequest represents a request to test Navidrome connection
type TestNavidromeRequest struct {
	NavidromeURL      string `json:"navidrome_url,omitempty"`
	NavidromeUsername string `json:"navidrome_username,omitempty"`
	NavidromePassword string `json:"navidrome_password,omitempty"`
}

// ValidURLPattern matches valid URLs for self-hosted servers
// Supports: domains, IP addresses, localhost, Docker hostnames, with optional port and path
var ValidURLPattern = regexp.MustCompile(`^https?://[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*(:[0-9]+)?(/.*)?$`)

// SettingsHandler handles settings-related endpoints
type SettingsHandler struct {
	db         *db.Database
	jwtManager *auth.JWTManager
	cfg        *config.Config
}

// NewSettingsHandler creates a new settings handler
func NewSettingsHandler(database *db.Database, jwtManager *auth.JWTManager, cfg *config.Config) *SettingsHandler {
	return &SettingsHandler{
		db:         database,
		jwtManager: jwtManager,
		cfg:        cfg,
	}
}

// GetSettings handles GET /api/v1/settings
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
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
	requestedUserID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Verify the authenticated user matches the requested user
	// This prevents context manipulation attacks where someone tries to
	// access another user's settings by changing the context userID
	if authUserID != requestedUserID {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	user, err := h.db.GetUserByID(authUserID)
	if err != nil {
		http.Error(w, "failed to get user: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	response := SettingsResponse{
		NavidromeURL:      user.NavidromeURL,
		NavidromeUsername: user.NavidromeUsername,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateSettings handles PUT /api/v1/settings
func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	var req SettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate Navidrome URL if provided (pointer is not nil)
	// Empty string is allowed (to clear the URL), but invalid format returns 400
	if req.NavidromeURL != nil {
		url := *req.NavidromeURL
		// Empty string is valid (clears the URL)
		if url != "" && !ValidURLPattern.MatchString(url) {
			http.Error(w, "invalid Navidrome URL", http.StatusBadRequest)
			return
		}
	}

	// Note: Username and password can be empty strings (to clear credentials)
	// Username is required to be present but can be empty to clear

	// Update user settings with partial update support
	// If a field is nil (not provided), preserve existing value
	// If a field is not nil, use the provided value (empty string clears the field for URL)
	var navidromeURL interface{} = nil
	if req.NavidromeURL != nil {
		navidromeURL = *req.NavidromeURL
		// Empty string will be passed to the database function which will clear it to NULL
	}
	var navidromeUsername, navidromePassword string
	if req.NavidromeUsername != nil {
		navidromeUsername = *req.NavidromeUsername
	}
	if req.NavidromePassword != nil {
		navidromePassword = *req.NavidromePassword
	}

	if err := h.db.UpdateUserNavidrome(userID, navidromeURL, navidromeUsername, navidromePassword); err != nil {
		http.Error(w, "failed to update settings: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "settings updated"})
}

// TestNavidromeConnection handles POST /api/v1/settings/test-navidrome
func (h *SettingsHandler) TestNavidromeConnection(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// Try to decode request body for credentials
	var req TestNavidromeRequest
	json.NewDecoder(r.Body).Decode(&req) // Ignore error - body may be empty

	var url, username, password string

	// Use request body values if all provided, otherwise fall back to database
	if req.NavidromeURL != "" && req.NavidromeUsername != "" && req.NavidromePassword != "" {
		url = req.NavidromeURL
		username = req.NavidromeUsername
		password = req.NavidromePassword
	} else {
		// Fall back to saved values
		user, err := h.db.GetUserByID(userID)
		if err != nil {
			http.Error(w, "failed to get user: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		// Check if Navidrome is configured
		if user.NavidromeURL == "" || user.NavidromeUsername == "" {
			http.Error(w, "Navidrome not configured. Please provide credentials.", http.StatusBadRequest)
			return
		}
		url = user.NavidromeURL
		username = user.NavidromeUsername
		password = user.NavidromePassword
	}

	// Use the Navidrome client which properly implements salted MD5 token auth
	client := navidrome.NewClient(url, username, password)
	if err := client.Ping(r.Context()); err != nil {
		http.Error(w, "Navidrome connection failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "connection successful"})
}

// GetStorageSettings handles GET /api/v1/settings/storage
func (h *SettingsHandler) GetStorageSettings(w http.ResponseWriter, r *http.Request) {
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

	_, err := h.jwtManager.ValidateToken(parts[1])
	if err != nil {
		http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Return current music root from config
	response := StorageSettingsResponse{
		MusicRoot: h.cfg.Storage.MusicRoot,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateStorageSettings handles PUT /api/v1/settings/storage
func (h *SettingsHandler) UpdateStorageSettings(w http.ResponseWriter, r *http.Request) {
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

	_, err := h.jwtManager.ValidateToken(parts[1])
	if err != nil {
		http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
		return
	}

	var req StorageSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate that the path exists and is a directory
	if req.MusicRoot == "" {
		http.Error(w, "music_root is required", http.StatusBadRequest)
		return
	}

	// Check if path exists
	info, err := os.Stat(req.MusicRoot)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "music_root path does not exist", http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to check music_root path: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if it's a directory
	if !info.IsDir() {
		http.Error(w, "music_root must be a directory", http.StatusBadRequest)
		return
	}

	// Update the config in memory
	// Note: This doesn't persist across restarts - a production app would save to a config file or database
	h.cfg.Storage.MusicRoot = req.MusicRoot

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "storage settings updated"})
}

// maskCredential masks a credential string, showing only first 3 characters
func maskCredential(s string) string {
	if len(s) <= 3 {
		if s == "" {
			return ""
		}
		return "***"
	}
	return s[:3] + "***"
}

// GetSourceSettings handles GET /api/v1/settings/sources
func (h *SettingsHandler) GetSourceSettings(w http.ResponseWriter, r *http.Request) {
	// Validate JWT token
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

	_, err := h.jwtManager.ValidateToken(parts[1])
	if err != nil {
		http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Build response with masked credentials
	response := SourceSettingsResponse{
		Tidal: TidalSourceSettings{
			Configured: h.cfg.Sources.Tidal.ClientID != "" && h.cfg.Sources.Tidal.ClientSecret != "",
			ClientID:   maskCredential(h.cfg.Sources.Tidal.ClientID),
			Quality:    h.cfg.Sources.Tidal.Quality,
		},
		Qobuz: QobuzSourceSettings{
			Configured: h.cfg.Sources.Qobuz.AppID != "" && h.cfg.Sources.Qobuz.Secret != "",
			AppID:      maskCredential(h.cfg.Sources.Qobuz.AppID),
			Quality:    h.cfg.Sources.Qobuz.Quality,
		},
		PreferredSource: h.cfg.Sources.PreferredSource,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateSourceSettings handles PUT /api/v1/settings/sources
func (h *SettingsHandler) UpdateSourceSettings(w http.ResponseWriter, r *http.Request) {
	// Validate JWT token
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

	_, err := h.jwtManager.ValidateToken(parts[1])
	if err != nil {
		http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
		return
	}

	var req SourceSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Update Tidal settings if provided
	if req.Tidal != nil {
		if req.Tidal.ClientID != "" {
			h.cfg.Sources.Tidal.ClientID = req.Tidal.ClientID
		}
		if req.Tidal.ClientSecret != "" {
			h.cfg.Sources.Tidal.ClientSecret = req.Tidal.ClientSecret
		}
		if req.Tidal.Quality != "" {
			// Validate quality value
			validQualities := map[string]bool{
				"HI_RES": true, "HI_RES_LOSSLESS": true, "LOSSLESS": true, "HIGH": true, "LOW": true,
			}
			if !validQualities[req.Tidal.Quality] {
				http.Error(w, "invalid Tidal quality value", http.StatusBadRequest)
				return
			}
			h.cfg.Sources.Tidal.Quality = req.Tidal.Quality
		}
	}

	// Update Qobuz settings if provided
	if req.Qobuz != nil {
		if req.Qobuz.AppID != "" {
			h.cfg.Sources.Qobuz.AppID = req.Qobuz.AppID
		}
		if req.Qobuz.Secret != "" {
			h.cfg.Sources.Qobuz.Secret = req.Qobuz.Secret
		}
		if req.Qobuz.Quality != "" {
			// Validate quality value
			validQualities := map[string]bool{
				"FLAC24": true, "FLAC16": true, "MP3": true, "HI_RES": true,
			}
			if !validQualities[req.Qobuz.Quality] {
				http.Error(w, "invalid Qobuz quality value", http.StatusBadRequest)
				return
			}
			h.cfg.Sources.Qobuz.Quality = req.Qobuz.Quality
		}
	}

	// Update preferred source if provided
	if req.PreferredSource != nil {
		preferredSource := *req.PreferredSource
		if preferredSource != "tidal" && preferredSource != "qobuz" {
			http.Error(w, "preferred_source must be 'tidal' or 'qobuz'", http.StatusBadRequest)
			return
		}
		h.cfg.Sources.PreferredSource = preferredSource
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "source settings updated"})
}

// TestTidalConnection handles POST /api/v1/settings/test-tidal
func (h *SettingsHandler) TestTidalConnection(w http.ResponseWriter, r *http.Request) {
	// Validate JWT token
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

	_, err := h.jwtManager.ValidateToken(parts[1])
	if err != nil {
		http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Check if Tidal is configured
	if h.cfg.Sources.Tidal.ClientID == "" || h.cfg.Sources.Tidal.ClientSecret == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ConnectionTestResponse{
			Success: false,
			Message: "Tidal credentials not configured",
		})
		return
	}

	// Create Tidal client and test connection
	tidalClient := tidal.NewClient(h.cfg.Sources.Tidal.ClientID, h.cfg.Sources.Tidal.ClientSecret)

	// Try to get an access token to verify credentials
	ctx := r.Context()
	_, err = tidalClient.GetAccessToken(ctx)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ConnectionTestResponse{
			Success: false,
			Message: "Failed to authenticate with Tidal: " + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ConnectionTestResponse{
		Success: true,
		Message: "Successfully connected to Tidal",
	})
}

// TestQobuzConnection handles POST /api/v1/settings/test-qobuz
func (h *SettingsHandler) TestQobuzConnection(w http.ResponseWriter, r *http.Request) {
	// Validate JWT token
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

	_, err := h.jwtManager.ValidateToken(parts[1])
	if err != nil {
		http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Check if Qobuz is configured
	if h.cfg.Sources.Qobuz.AppID == "" || h.cfg.Sources.Qobuz.Secret == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ConnectionTestResponse{
			Success: false,
			Message: "Qobuz credentials not configured",
		})
		return
	}

	// Create Qobuz client and test connection by performing a simple search
	qobuzClient := qobuz.NewClient(h.cfg.Sources.Qobuz.AppID, h.cfg.Sources.Qobuz.Secret)

	// Try a simple search to verify credentials work
	ctx := r.Context()
	_, err = qobuzClient.SearchTracks(ctx, "test", 1)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ConnectionTestResponse{
			Success: false,
			Message: "Failed to connect to Qobuz: " + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ConnectionTestResponse{
		Success: true,
		Message: "Successfully connected to Qobuz",
	})
}
