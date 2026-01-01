package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewBatch(t *testing.T) {
	batch := NewBatch(1, "https://open.spotify.com/album/123", "album", "Test Album")

	assert.NotEmpty(t, batch.ID)
	assert.Equal(t, int64(1), batch.UserID)
	assert.Equal(t, "https://open.spotify.com/album/123", batch.SpotifyURL)
	assert.Equal(t, SpotifyTypeAlbum, batch.SpotifyType)
	assert.Equal(t, "Test Album", batch.Name)
	assert.Equal(t, BatchStatusPending, batch.Status)
	assert.Equal(t, 0, batch.TotalJobs)
	assert.Equal(t, 0, batch.CompletedJobs)
	assert.Equal(t, 0, batch.FailedJobs)
	assert.False(t, batch.CreatedAt.IsZero())
	assert.False(t, batch.UpdatedAt.IsZero())
}

func TestBatch_ToResponse(t *testing.T) {
	batch := &Batch{
		ID:            "batch-123",
		UserID:        1,
		SpotifyURL:    "https://open.spotify.com/playlist/456",
		SpotifyType:   SpotifyTypePlaylist,
		Name:          "Test Playlist",
		TotalJobs:     10,
		CompletedJobs: 5,
		FailedJobs:    2,
		Status:        BatchStatusProcessing,
		CreatedAt:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC),
	}

	response := batch.ToResponse()

	assert.Equal(t, "batch-123", response.ID)
	assert.Equal(t, "https://open.spotify.com/playlist/456", response.SpotifyURL)
	assert.Equal(t, SpotifyTypePlaylist, response.SpotifyType)
	assert.Equal(t, "Test Playlist", response.Name)
	assert.Equal(t, 10, response.TotalJobs)
	assert.Equal(t, 5, response.CompletedJobs)
	assert.Equal(t, 2, response.FailedJobs)
	assert.Equal(t, BatchStatusProcessing, response.Status)
	assert.Equal(t, "2025-01-01T00:00:00Z", response.CreatedAt)
	assert.Equal(t, "2025-01-01T01:00:00Z", response.UpdatedAt)
}
