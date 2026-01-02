package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"spotisync/internal/api/middleware"
	"spotisync/internal/auth"
	"spotisync/internal/config"
	"spotisync/internal/db"
	"spotisync/internal/db/models"
	"spotisync/internal/services/navidrome"
	"spotisync/internal/websocket"
)

// PlaylistCreateRequest represents a request to create a playlist from a batch
type PlaylistCreateRequest struct {
	// Optional: custom name for the playlist (defaults to batch name)
	Name string `json:"name,omitempty"`
}

// PlaylistCreateResponse represents the response for playlist creation
type PlaylistCreateResponse struct {
	PlaylistID   string `json:"playlist_id"`
	PlaylistName string `json:"playlist_name"`
	TracksFound  int    `json:"tracks_found"`
	TracksFailed int    `json:"tracks_failed"`
	Message      string `json:"message"`
}

// PlaylistHandler handles playlist-related endpoints
type PlaylistHandler struct {
	db         *db.Database
	jwtManager *auth.JWTManager
	cfg        *config.Config
	hub        *websocket.Hub
}

// NewPlaylistHandler creates a new playlist handler
func NewPlaylistHandler(database *db.Database, jwtManager *auth.JWTManager, cfg *config.Config, hub *websocket.Hub) *PlaylistHandler {
	return &PlaylistHandler{
		db:         database,
		jwtManager: jwtManager,
		cfg:        cfg,
		hub:        hub,
	}
}

// CreatePlaylistFromBatch handles POST /api/v1/batches/{id}/playlist
// Creates a Navidrome playlist from a completed batch
func (h *PlaylistHandler) CreatePlaylistFromBatch(w http.ResponseWriter, r *http.Request) {
	// Get the authenticated user ID from context (already validated by AuthMiddleware)
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get batch ID from path
	batchID := GetIDFromRoute(r)
	if batchID == "" {
		http.Error(w, "batch ID is required", http.StatusBadRequest)
		return
	}

	// Get the batch
	batch, err := h.db.GetBatchByID(batchID)
	if err != nil {
		http.Error(w, "failed to get batch: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if batch == nil {
		http.Error(w, "batch not found", http.StatusNotFound)
		return
	}

	// Verify the batch belongs to the authenticated user
	if batch.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Verify the batch is a playlist type
	if batch.SpotifyType != "playlist" {
		http.Error(w, "batch is not a playlist type. Only playlist batches can be converted to Navidrome playlists.", http.StatusBadRequest)
		return
	}

	// Verify the batch has completed jobs
	if batch.CompletedJobs == 0 {
		http.Error(w, "batch has no completed jobs. Download some tracks first before creating a playlist.", http.StatusBadRequest)
		return
	}

	// Get user to check Navidrome credentials
	user, err := h.db.GetUserByID(userID)
	if err != nil {
		http.Error(w, "failed to get user: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// Check if Navidrome credentials are configured
	if user.NavidromeURL == "" || user.NavidromeUsername == "" {
		http.Error(w, "Navidrome not configured. Please set your Navidrome credentials in settings.", http.StatusBadRequest)
		return
	}

	// Parse optional request body for custom playlist name
	var req PlaylistCreateRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Ignore decode errors for optional body
			log.Printf("[playlist] warning: failed to decode request body: %v", err)
		}
	}

	// Determine playlist name
	playlistName := req.Name
	if playlistName == "" {
		playlistName = batch.Name
	}
	if playlistName == "" {
		playlistName = "Spotisync Playlist"
	}

	// Get all jobs for the batch
	jobs, err := h.db.GetJobsByBatchID(batchID)
	if err != nil {
		http.Error(w, "failed to get jobs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Prepare completed jobs for the new method
	var completedJobs []*models.Job
	for i := range jobs {
		job := jobs[i]
		if job.Status == models.JobStatusCompleted {
			completedJobs = append(completedJobs, job)
		}
	}

	if len(completedJobs) == 0 {
		http.Error(w, "no completed jobs to create playlist", http.StatusBadRequest)
		return
	}

	// Create Navidrome client
	client := navidrome.NewClient(user.NavidromeURL, user.NavidromeUsername, user.NavidromePassword)

	// Create playlist using the new method
	result := client.CreatePlaylistWithDetails(r.Context(), playlistName, completedJobs)
	if result.Error != "" {
		// Update batch playlist status to failed
		if err := h.db.UpdateBatchPlaylistStatus(batchID, models.PlaylistStatusFailed, "", result.Error, result.TracksFound, result.TracksFailed); err != nil {
			log.Printf("[playlist] failed to update batch playlist status: %v", err)
		}
		// Send WebSocket notification for failure
		if h.hub != nil {
			h.hub.BroadcastPlaylistUpdate(userID, batchID, "failed", "", result.Error, result.TracksFound, result.TracksFailed, result.TracksFound+result.TracksFailed)
		}
		http.Error(w, "failed to create playlist: "+result.Error, http.StatusInternalServerError)
		return
	}

	// Update batch playlist status to completed with the playlist ID
	if err := h.db.UpdateBatchPlaylistStatus(batchID, models.PlaylistStatusCompleted, result.PlaylistID, "", result.TracksFound, result.TracksFailed); err != nil {
		log.Printf("[playlist] failed to update batch playlist status: %v", err)
	}

	// Send WebSocket notification for success
	if h.hub != nil {
		h.hub.BroadcastPlaylistUpdate(userID, batchID, "completed", result.PlaylistID, "", result.TracksFound, result.TracksFailed, result.TracksFound+result.TracksFailed)
	}

	// Update jobs that were successfully added to the playlist
	for _, trackResult := range result.TrackResults {
		if trackResult.Found {
			if err := h.db.UpdateJobInPlaylist(trackResult.JobID, true); err != nil {
				log.Printf("[playlist] failed to update in_playlist for job %s: %v", trackResult.JobID, err)
			}
		}
	}

	// Build response message
	message := "playlist created successfully"
	if result.TracksFailed > 0 {
		message = fmt.Sprintf("playlist created with %d tracks found and %d tracks failed", result.TracksFound, result.TracksFailed)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PlaylistCreateResponse{
		PlaylistID:   result.PlaylistID,
		PlaylistName: playlistName,
		TracksFound:  result.TracksFound,
		TracksFailed: result.TracksFailed,
		Message:      message,
	})
}
