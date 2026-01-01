package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"spotisync/internal/config"
	"spotisync/internal/services/spotify"
)

// PreviewRequest represents a request to preview a Spotify URL
type PreviewRequest struct {
	SpotifyURL string `json:"spotify_url"`
}

// PreviewTrack represents a track in the preview response
type PreviewTrack struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Artist     string   `json:"artist"`
	Artists    []string `json:"artists"`
	Album      string   `json:"album"`
	DurationMs int      `json:"duration_ms"`
	ISRC       string   `json:"isrc"`
}

// PreviewResponse represents the response for previewing a Spotify URL
type PreviewResponse struct {
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	CoverURL    string         `json:"cover_url"`
	TotalTracks int            `json:"total_tracks"`
	Tracks      []PreviewTrack `json:"tracks"`
}

// PreviewHandler handles preview-related endpoints
type PreviewHandler struct {
	spotifyClient spotify.SpotifyClientInterface
}

// NewPreviewHandler creates a new preview handler with real Spotify client
func NewPreviewHandler(cfg *config.Config) *PreviewHandler {
	spotifyClient := spotify.NewAPIClient(cfg.Spotify.Username, cfg.Spotify.Password)
	return &PreviewHandler{
		spotifyClient: spotifyClient,
	}
}

// NewPreviewHandlerWithClient creates a new preview handler with a provided Spotify client.
// This is useful for testing with a mock client.
func NewPreviewHandlerWithClient(spotifyClient spotify.SpotifyClientInterface) *PreviewHandler {
	return &PreviewHandler{
		spotifyClient: spotifyClient,
	}
}

// Preview handles POST /api/v1/preview
// It fetches track information from a Spotify URL without creating a batch
func (h *PreviewHandler) Preview(w http.ResponseWriter, r *http.Request) {
	var req PreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.SpotifyURL == "" {
		http.Error(w, "spotify_url is required", http.StatusBadRequest)
		return
	}

	// Parse and validate Spotify URL
	resourceType, _, err := spotify.ParseSpotifyURL(req.SpotifyURL)
	if err != nil {
		http.Error(w, "invalid Spotify URL: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Fetch tracks from Spotify API
	tracks, name, err := h.spotifyClient.GetTracksFromURL(r.Context(), req.SpotifyURL)
	if err != nil {
		if errors.Is(err, spotify.ErrNotFound) {
			http.Error(w, "Spotify resource not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, spotify.ErrInvalidCredentials) {
			http.Error(w, "Spotify API credentials not configured", http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "failed to fetch Spotify data: "+err.Error(), http.StatusBadGateway)
		return
	}

	if len(tracks) == 0 {
		http.Error(w, "no tracks found in Spotify resource", http.StatusBadRequest)
		return
	}

	// Convert tracks to preview format
	previewTracks := make([]PreviewTrack, len(tracks))
	for i, track := range tracks {
		previewTracks[i] = PreviewTrack{
			ID:         track.ID,
			Name:       track.Name,
			Artist:     track.Artist,
			Artists:    track.Artists,
			Album:      track.Album,
			DurationMs: track.DurationMs,
			ISRC:       track.ISRC,
		}
	}

	// Get cover URL from first track (all tracks in album/playlist have same cover)
	coverURL := ""
	if len(tracks) > 0 {
		coverURL = tracks[0].CoverArtURL
	}

	response := PreviewResponse{
		Name:        name,
		Type:        resourceType,
		CoverURL:    coverURL,
		TotalTracks: len(tracks),
		Tracks:      previewTracks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
