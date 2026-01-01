package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"spotisync/internal/api/middleware"
	"spotisync/internal/auth"
	"spotisync/internal/config"
	"spotisync/internal/db"
	"spotisync/internal/db/models"
	"spotisync/internal/services/navidrome"
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
	TracksTotal  int    `json:"tracks_total"`
	Message      string `json:"message"`
}

// PlaylistHandler handles playlist-related endpoints
type PlaylistHandler struct {
	db         *db.Database
	jwtManager *auth.JWTManager
	cfg        *config.Config
}

// NewPlaylistHandler creates a new playlist handler
func NewPlaylistHandler(database *db.Database, jwtManager *auth.JWTManager, cfg *config.Config) *PlaylistHandler {
	return &PlaylistHandler{
		db:         database,
		jwtManager: jwtManager,
		cfg:        cfg,
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

	// Create Navidrome client
	client := navidrome.NewClient(user.NavidromeURL, user.NavidromeUsername, user.NavidromePassword)

	// Collect Navidrome track IDs for completed jobs
	var navidromeTrackIDs []string
	tracksTotal := 0
	tracksFound := 0

	for _, job := range jobs {
		// Only process completed jobs
		if job.Status != models.JobStatusCompleted {
			continue
		}
		tracksTotal++

		// Try to find the track in Navidrome
		var track *navidrome.Track
		var searchErr error

		// First try by ISRC if available
		if job.ISRC != "" {
			track, searchErr = client.SearchTrackByISRC(r.Context(), job.ISRC)
			if searchErr != nil {
				log.Printf("[playlist] ISRC search failed for %s: %v, trying title/artist", job.ISRC, searchErr)
			}
		}

		// Fall back to title/artist search if ISRC search failed or ISRC not available
		if track == nil {
			track, searchErr = client.SearchTrackByTitle(r.Context(), job.TrackName, job.ArtistName)
			if searchErr != nil {
				log.Printf("[playlist] title/artist search failed for '%s - %s': %v", job.ArtistName, job.TrackName, searchErr)
				continue
			}
		}

		if track != nil {
			navidromeTrackIDs = append(navidromeTrackIDs, track.ID)
			tracksFound++
			log.Printf("[playlist] found track in Navidrome: %s - %s (ID: %s)", track.Artist, track.Title, track.ID)
		}
	}

	// Check if we found any tracks
	if len(navidromeTrackIDs) == 0 {
		http.Error(w, "no matching tracks found in Navidrome. Make sure Navidrome has scanned the downloaded files.", http.StatusNotFound)
		return
	}

	// Create or update the playlist in Navidrome
	playlistID, err := client.CreateOrUpdatePlaylist(r.Context(), playlistName, navidromeTrackIDs)
	if err != nil {
		http.Error(w, "failed to create playlist in Navidrome: "+err.Error(), http.StatusBadGateway)
		return
	}

	log.Printf("[playlist] created/updated Navidrome playlist '%s' (ID: %s) with %d/%d tracks", playlistName, playlistID, tracksFound, tracksTotal)

	// Build response message
	message := "playlist created successfully"
	if tracksFound < tracksTotal {
		message = "playlist created with partial tracks. Some tracks could not be found in Navidrome."
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PlaylistCreateResponse{
		PlaylistID:   playlistID,
		PlaylistName: playlistName,
		TracksFound:  tracksFound,
		TracksTotal:  tracksTotal,
		Message:      message,
	})
}
