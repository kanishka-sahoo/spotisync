package models

import (
	"time"

	"github.com/google/uuid"
)

// BatchStatus represents the status of a batch
type BatchStatus string

const (
	BatchStatusPending    BatchStatus = "pending"
	BatchStatusProcessing BatchStatus = "processing"
	BatchStatusCompleted  BatchStatus = "completed"
	BatchStatusFailed     BatchStatus = "failed"
)

// SpotifyType represents the type of Spotify URL
type SpotifyType string

const (
	SpotifyTypeAlbum    SpotifyType = "album"
	SpotifyTypePlaylist SpotifyType = "playlist"
	SpotifyTypeArtist   SpotifyType = "artist"
)

// Batch represents a batch of download jobs from a single Spotify URL
type Batch struct {
	ID            string      `json:"id"`
	UserID        int64       `json:"user_id"`
	SpotifyURL    string      `json:"spotify_url"`
	SpotifyType   SpotifyType `json:"spotify_type"`
	Name          string      `json:"name"`
	TotalJobs     int         `json:"total_jobs"`
	CompletedJobs int         `json:"completed_jobs"`
	FailedJobs    int         `json:"failed_jobs"`
	Status        BatchStatus `json:"status"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// NewBatch creates a new batch with a generated UUID
func NewBatch(userID int64, spotifyURL, spotifyType, name string) *Batch {
	return &Batch{
		ID:          uuid.New().String(),
		UserID:      userID,
		SpotifyURL:  spotifyURL,
		SpotifyType: SpotifyType(spotifyType),
		Name:        name,
		Status:      BatchStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// BatchResponse is the public-facing batch data
type BatchResponse struct {
	ID            string      `json:"id"`
	SpotifyURL    string      `json:"spotify_url"`
	SpotifyType   SpotifyType `json:"spotify_type"`
	Name          string      `json:"name"`
	TotalJobs     int         `json:"total_jobs"`
	CompletedJobs int         `json:"completed_jobs"`
	FailedJobs    int         `json:"failed_jobs"`
	Status        BatchStatus `json:"status"`
	CreatedAt     string      `json:"created_at"`
	UpdatedAt     string      `json:"updated_at"`
}

// ToResponse converts a Batch to BatchResponse
func (b *Batch) ToResponse() *BatchResponse {
	return &BatchResponse{
		ID:            b.ID,
		SpotifyURL:    b.SpotifyURL,
		SpotifyType:   b.SpotifyType,
		Name:          b.Name,
		TotalJobs:     b.TotalJobs,
		CompletedJobs: b.CompletedJobs,
		FailedJobs:    b.FailedJobs,
		Status:        b.Status,
		CreatedAt:     b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     b.UpdatedAt.Format(time.RFC3339),
	}
}
