package navidrome

import (
	"context"
	"fmt"
	"log"

	"spotisync/internal/api/middleware"
	"spotisync/internal/db"
	"spotisync/internal/db/models"
)

// Syncer handles syncing playlists from Spotify to Navidrome
type Syncer struct {
	db *db.Database
}

// NewSyncer creates a new playlist syncer
func NewSyncer(db *db.Database) *Syncer {
	return &Syncer{
		db: db,
	}
}

// getClientForUser creates a Navidrome client for a specific user
// Returns nil if the user doesn't have Navidrome configured
func (s *Syncer) getClientForUser(ctx context.Context, userID int64) (*Client, error) {
	user, err := s.db.GetUserByID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if user.NavidromeURL == "" || user.NavidromeUsername == "" || user.NavidromePassword == "" {
		return nil, nil
	}

	return NewClient(user.NavidromeURL, user.NavidromeUsername, user.NavidromePassword), nil
}

// CheckPlaylist checks if a playlist exists for a batch and updates the batch if found
func (s *Syncer) CheckPlaylist(ctx context.Context, batchID string) (*Playlist, error) {
	// Get user ID from context
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, fmt.Errorf("user not authenticated")
	}

	// Get batch from database
	batch, err := s.db.GetBatchByID(batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch: %w", err)
	}
	if batch == nil {
		return nil, fmt.Errorf("batch not found")
	}

	// Verify user ownership
	if batch.UserID != userID {
		return nil, fmt.Errorf("batch does not belong to user")
	}

	// Get client for user
	client, err := s.getClientForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Navidrome client: %w", err)
	}

	// Check if Navidrome is configured for the user
	if client == nil {
		return nil, fmt.Errorf("Navidrome is not configured for this user")
	}

	// Check if playlist with batch name exists
	playlist, err := client.CheckPlaylist(ctx, batch.Name)
	if err != nil {
		log.Printf("[navidrome] failed to check playlist: %v", err)
		return nil, fmt.Errorf("failed to check playlist: %w", err)
	}

	// If playlist is found, update the batch
	if playlist != nil {
		err = s.db.UpdateBatchPlaylistStatus(batchID, models.PlaylistStatusCompleted, playlist.ID, "", 0, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to update batch: %w", err)
		}
	}

	return playlist, nil
}

// SyncPlaylist syncs a batch's completed jobs to a Navidrome playlist
func (s *Syncer) SyncPlaylist(ctx context.Context, batchID string) (*PlaylistCreationResult, error) {
	// Get user ID from context
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, fmt.Errorf("user not authenticated")
	}

	// Get batch from database
	batch, err := s.db.GetBatchByID(batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch: %w", err)
	}
	if batch == nil {
		return nil, fmt.Errorf("batch not found")
	}

	// Verify user ownership
	if batch.UserID != userID {
		return nil, fmt.Errorf("batch does not belong to user")
	}

	// Check if batch is for a playlist type
	if batch.SpotifyType != models.SpotifyTypePlaylist {
		return nil, fmt.Errorf("batch is not a playlist type")
	}

	// Get all jobs for the batch
	jobs, err := s.db.GetJobsByBatchID(batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get jobs: %w", err)
	}

	// Filter completed jobs
	var completedJobs []*models.Job
	for _, job := range jobs {
		if job.Status == models.JobStatusCompleted {
			completedJobs = append(completedJobs, job)
		}
	}

	// Update batch status to creating
	err = s.db.UpdateBatchPlaylistStatus(batchID, models.PlaylistStatusCreating, "", "", 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to update batch status: %w", err)
	}

	// Get client for user
	client, err := s.getClientForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Navidrome client: %w", err)
	}

	// Check if Navidrome is configured for the user
	if client == nil {
		result := &PlaylistCreationResult{
			PlaylistName: batch.Name,
			TotalTracks:  len(completedJobs),
			Success:      false,
			Error:        "Navidrome is not configured for this user",
		}
		return result, nil
	}

	// Create or update playlist with completed jobs
	result := client.CreatePlaylistWithDetails(ctx, batch.Name, completedJobs)

	// Update batch with results
	status := models.PlaylistStatusCompleted
	if !result.Success {
		status = models.PlaylistStatusFailed
	}
	err = s.db.UpdateBatchPlaylistStatus(batchID, status, result.PlaylistID, result.Error, result.TracksFound, result.TracksFailed)
	if err != nil {
		return nil, fmt.Errorf("failed to update batch: %w", err)
	}

	return result, nil
}
