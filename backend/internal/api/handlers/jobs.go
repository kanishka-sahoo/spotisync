package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"spotisync/internal/api/middleware"
	"spotisync/internal/auth"
	"spotisync/internal/config"
	"spotisync/internal/db"
	"spotisync/internal/db/models"
	"spotisync/internal/scheduler"
	"spotisync/internal/services/spotify"
	"strings"
)

// JobRequest represents a request to create a job batch
type JobRequest struct {
	SpotifyURL string `json:"spotify_url" binding:"required"`
}

// JobBatchResponse represents a batch with jobs
type JobBatchResponse struct {
	*models.Batch
	Jobs []*models.JobResponse `json:"jobs"`
}

// JobsHandler handles job-related endpoints
type JobsHandler struct {
	db            *db.Database
	jobScheduler  *scheduler.JobScheduler
	jwtManager    *auth.JWTManager
	spotifyClient spotify.SpotifyClientInterface
}

// NewJobsHandler creates a new jobs handler
func NewJobsHandler(database *db.Database, jobScheduler *scheduler.JobScheduler, jwtManager *auth.JWTManager, cfg *config.Config) *JobsHandler {
	// Create Spotify client using credentials from config
	spotifyClient := spotify.NewAPIClient(cfg.Spotify.Username, cfg.Spotify.Password)

	return &JobsHandler{
		db:            database,
		jobScheduler:  jobScheduler,
		jwtManager:    jwtManager,
		spotifyClient: spotifyClient,
	}
}

// NewJobsHandlerWithClient creates a new jobs handler with a provided Spotify client.
// This is useful for testing with a mock client.
func NewJobsHandlerWithClient(database *db.Database, jobScheduler *scheduler.JobScheduler, jwtManager *auth.JWTManager, spotifyClient spotify.SpotifyClientInterface) *JobsHandler {
	return &JobsHandler{
		db:            database,
		jobScheduler:  jobScheduler,
		jwtManager:    jwtManager,
		spotifyClient: spotifyClient,
	}
}

// CreateBatch handles POST /api/v1/jobs
func (h *JobsHandler) CreateBatch(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate and parse Spotify URL
	spotifyType, resourceID, err := spotify.ParseSpotifyURL(req.SpotifyURL)
	if err != nil {
		http.Error(w, "invalid Spotify URL: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Fetch tracks from Spotify API using fast path for artists
	var tracks []spotify.Track
	var name string
	var totalJobs int
	if spotifyType == "artist" {
		var discographyResult *spotify.DiscographyResult
		discographyResult, err = h.spotifyClient.GetArtistDiscography(r.Context(), resourceID, true)
		if err == nil {
			tracks = discographyResult.Tracks
			name = discographyResult.Name
			totalJobs = discographyResult.TotalTracks
		} else {
			totalJobs = 0
		}
	} else {
		tracks, name, err = h.spotifyClient.GetTracksFromURL(r.Context(), req.SpotifyURL)
		if err == nil {
			totalJobs = len(tracks)
		} else {
			totalJobs = 0
		}
	}
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

	// Create batch
	batch := models.NewBatch(userID, req.SpotifyURL, spotifyType, name)
	batch.TotalJobs = totalJobs

	if err := h.db.CreateBatch(batch); err != nil {
		http.Error(w, "failed to create batch: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Track initial track IDs for deduplication
	initialTrackIDs := make(map[string]bool)

	// Create jobs for each track
	for _, track := range tracks {
		// Track this ID to avoid duplicates in background fetch
		initialTrackIDs[track.ID] = true

		// Convert spotify.Track to models.SpotifyTrack
		spotifyTrack := &models.SpotifyTrack{
			ID:           track.ID,
			Name:         track.Name,
			Artist:       track.Artist,
			Artists:      track.Artists,
			Album:        track.Album,
			AlbumID:      track.AlbumID,
			AlbumArtist:  track.AlbumArtist,
			AlbumArtists: track.AlbumArtists,
			TrackNumber:  track.TrackNumber,
			DiscNumber:   track.DiscNumber,
			DurationMs:   track.DurationMs,
			ISRC:         track.ISRC,
			ReleaseYear:  track.ReleaseYear,
			ReleaseDate:  track.ReleaseDate,
			TotalTracks:  track.TotalTracks,
			CoverArtURL:  track.CoverArtURL,
			Explicit:     track.Explicit,
		}

		job := models.NewJob(batch.ID, userID, spotifyTrack)
		if err := h.db.CreateJob(job); err != nil {
			// Log error but continue with other jobs
			continue
		}

		// Enqueue job for processing
		h.jobScheduler.Enqueue(job.ID, userID)
	}

	// Start background fetch for remaining artist tracks
	// Debug: Log values before checking background fetch condition
	log.Printf("[debug] About to check background fetch condition - spotifyType=%q, totalJobs=%d, len(tracks)=%d", spotifyType, totalJobs, len(tracks))
	if spotifyType == "artist" && totalJobs > len(tracks) {
		log.Printf("[debug] Background fetch condition passed! Spawning goroutine for batchID=%s, resourceID=%s", batch.ID, resourceID)
		go h.fetchRemainingArtistTracks(context.Background(), batch.ID, resourceID, name, userID, initialTrackIDs)
	} else {
		log.Printf("[debug] Background fetch condition failed - spotifyType=%q (expected 'artist'), totalJobs=%d, len(tracks)=%d", spotifyType, totalJobs, len(tracks))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(batch.ToResponse())
}

// fetchRemainingArtistTracks fetches and creates jobs for remaining artist tracks in the background
func (h *JobsHandler) fetchRemainingArtistTracks(ctx context.Context, batchID, artistID, name string, userID int64, initialTrackIDs map[string]bool) {
	log.Printf("[background-fetch] Goroutine STARTED! batchID=%s, artistID=%s, name=%s", batchID, artistID, name)

	// Recover from any panics to prevent crashing the server
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[background-fetch] panic while fetching remaining tracks for artist %s: %v", name, r)
		}
	}()

	log.Printf("[background-fetch] Starting background fetch for artist %s", name)

	// Fetch full discography (preview=false)
	discographyResult, err := h.spotifyClient.GetArtistDiscography(ctx, artistID, false)
	if err != nil {
		log.Printf("[background-fetch] Failed to fetch full discography for artist %s: %v", name, err)
		return
	}

	log.Printf("[background-fetch] Fetched %d total tracks from discography for artist %s", len(discographyResult.Tracks), name)

	// Create jobs for tracks not already created
	log.Printf("[background-fetch] Checking %d initial track IDs for deduplication", len(initialTrackIDs))

	additionalJobs := 0
	skippedDuplicates := 0
	for _, track := range discographyResult.Tracks {
		// Skip if we already created a job for this track
		if initialTrackIDs[track.ID] {
			skippedDuplicates++
			continue
		}

		// Convert spotify.Track to models.SpotifyTrack
		spotifyTrack := &models.SpotifyTrack{
			ID:           track.ID,
			Name:         track.Name,
			Artist:       track.Artist,
			Artists:      track.Artists,
			Album:        track.Album,
			AlbumID:      track.AlbumID,
			AlbumArtist:  track.AlbumArtist,
			AlbumArtists: track.AlbumArtists,
			TrackNumber:  track.TrackNumber,
			DiscNumber:   track.DiscNumber,
			DurationMs:   track.DurationMs,
			ISRC:         track.ISRC,
			ReleaseYear:  track.ReleaseYear,
			ReleaseDate:  track.ReleaseDate,
			TotalTracks:  track.TotalTracks,
			CoverArtURL:  track.CoverArtURL,
			Explicit:     track.Explicit,
		}

		job := models.NewJob(batchID, userID, spotifyTrack)
		if err := h.db.CreateJob(job); err != nil {
			// Log error but continue with other jobs
			log.Printf("[background-fetch] Failed to create job for track %s - %s: %v", track.Artist, track.Name, err)
			continue
		}

		// Enqueue job for processing
		h.jobScheduler.Enqueue(job.ID, userID)
		additionalJobs++
	}

	log.Printf("[background-fetch] Fetched %d total tracks, skipped %d duplicates, created %d new jobs for artist %s", len(discographyResult.Tracks), skippedDuplicates, additionalJobs, name)
}

// ListJobs handles GET /api/v1/jobs
func (h *JobsHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	jobs, err := h.db.GetJobsByUserID(userID)
	if err != nil {
		http.Error(w, "failed to get jobs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := make([]*models.JobResponse, len(jobs))
	for i, job := range jobs {
		response[i] = job.ToResponse()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetJob handles GET /api/v1/jobs/:id
func (h *JobsHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	jobID := GetIDFromRoute(r)
	if jobID == "" {
		http.Error(w, "job ID is required", http.StatusBadRequest)
		return
	}

	job, err := h.db.GetJobByID(jobID)
	if err != nil {
		http.Error(w, "failed to get job: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if job == nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	// Ensure user owns the job
	if job.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job.ToResponse())
}

// RetryJob handles POST /api/v1/jobs/:id/retry
func (h *JobsHandler) RetryJob(w http.ResponseWriter, r *http.Request) {
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

	jobID := GetIDFromRoute(r)
	if jobID == "" {
		http.Error(w, "job ID is required", http.StatusBadRequest)
		return
	}

	job, err := h.db.GetJobByID(jobID)
	if err != nil {
		http.Error(w, "failed to get job: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if job == nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	// Ensure user owns the job
	if job.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Reset job for retry
	if err := h.db.ResetJobForRetry(jobID); err != nil {
		http.Error(w, "failed to retry job: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Add job to scheduler queue
	h.jobScheduler.Enqueue(jobID, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "job retry queued"})
}

// CancelJob handles DELETE /api/v1/jobs/:id
func (h *JobsHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
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

	jobID := GetIDFromRoute(r)
	if jobID == "" {
		http.Error(w, "job ID is required", http.StatusBadRequest)
		return
	}

	job, err := h.db.GetJobByID(jobID)
	if err != nil {
		http.Error(w, "failed to get job: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if job == nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	// Ensure user owns the job
	if job.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Can only cancel pending or in_progress jobs
	if job.Status != models.JobStatusPending && job.Status != models.JobStatusInProgress {
		http.Error(w, "can only cancel pending or in-progress jobs", http.StatusBadRequest)
		return
	}

	// Mark job as failed with cancellation message
	if err := h.db.UpdateJobFailed(jobID, "cancelled by user"); err != nil {
		http.Error(w, "failed to cancel job: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "job cancelled"})
}

// GetBatchJobs handles GET /api/v1/batches/:id
func (h *JobsHandler) GetBatchJobs(w http.ResponseWriter, r *http.Request) {
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

	jobs, err := h.db.GetJobsByBatchID(batchID)
	if err != nil {
		http.Error(w, "failed to get jobs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jobResponses := make([]*models.JobResponse, len(jobs))
	for i, job := range jobs {
		jobResponses[i] = job.ToResponse()
	}

	response := JobBatchResponse{
		Batch: batch,
		Jobs:  jobResponses,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// findIndex finds the index of a character in a string
func findIndex(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
