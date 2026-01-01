package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewJob(t *testing.T) {
	track := &SpotifyTrack{
		ID:          "spotify-track-123",
		Name:        "Enter Sandman",
		Artist:      "Metallica",
		Album:       "Metallica",
		AlbumArtist: "Metallica",
		TrackNumber: 1,
		TotalTracks: 12,
		ISRC:        "USEE1000233",
		ReleaseYear: 1991,
	}

	job := NewJob("batch-123", 1, track)

	assert.NotEmpty(t, job.ID)
	assert.Equal(t, "batch-123", job.BatchID)
	assert.Equal(t, int64(1), job.UserID)
	assert.Equal(t, "spotify-track-123", job.SpotifyTrackID)
	assert.Equal(t, "Enter Sandman", job.TrackName)
	assert.Equal(t, "Metallica", job.ArtistName)
	assert.Equal(t, "Metallica", job.AlbumName)
	assert.Equal(t, "Metallica", job.AlbumArtist)
	assert.Equal(t, 1, job.TrackNumber)
	assert.Equal(t, 12, job.TotalTracks)
	assert.Equal(t, "USEE1000233", job.ISRC)
	assert.Equal(t, 1991, job.ReleaseYear)
	assert.Equal(t, JobStatusPending, job.Status)
	assert.Equal(t, 0, job.RetryCount)
	assert.Equal(t, float64(0), job.Progress)
}

func TestJob_ToResponse(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-time.Minute)
	completedAt := now

	job := &Job{
		ID:              "job-123",
		BatchID:         "batch-123",
		UserID:          1,
		SpotifyTrackID:  "spotify-track-123",
		ISRC:            "USEE1000233",
		TrackName:       "Enter Sandman",
		ArtistName:      "Metallica",
		AlbumName:       "Metallica",
		AlbumArtist:     "Metallica",
		TrackNumber:     1,
		TotalTracks:     12,
		DurationMs:      300000,
		ReleaseYear:     1991,
		Status:          JobStatusCompleted,
		SourceService:   SourceTidal,
		LocalPath:       "/music/Metallica/Metallica/01 - Metallica - Enter Sandman.flac",
		CoverPath:       "/music/Metallica/Metallica/01 - Metallica - Enter Sandman.jpg",
		LyricsPath:      "/music/Metallica/Metallica/01 - Metallica - Enter Sandman.lrc",
		FileSize:        35600000,
		RetryCount:      0,
		Progress:        100,
		DownloadSpeed:   0,
		BytesDownloaded: 35600000,
		BytesTotal:      35600000,
		StartedAt:       &startedAt,
		CompletedAt:     &completedAt,
		CreatedAt:       now.Add(-5 * time.Minute),
		UpdatedAt:       now,
	}

	response := job.ToResponse()

	assert.Equal(t, "job-123", response.ID)
	assert.Equal(t, "batch-123", response.BatchID)
	assert.Equal(t, "spotify-track-123", response.SpotifyTrackID)
	assert.Equal(t, "USEE1000233", response.ISRC)
	assert.Equal(t, "Enter Sandman", response.TrackName)
	assert.Equal(t, "Metallica", response.ArtistName)
	assert.Equal(t, "Metallica", response.AlbumName)
	assert.Equal(t, "Metallica", response.AlbumArtist)
	assert.Equal(t, 1, response.TrackNumber)
	assert.Equal(t, 12, response.TotalTracks)
	assert.Equal(t, 300000, response.DurationMs)
	assert.Equal(t, 1991, response.ReleaseYear)
	assert.Equal(t, JobStatusCompleted, response.Status)
	assert.Equal(t, SourceTidal, response.SourceService)
	assert.Equal(t, "/music/Metallica/Metallica/01 - Metallica - Enter Sandman.flac", response.LocalPath)
	assert.Equal(t, "/music/Metallica/Metallica/01 - Metallica - Enter Sandman.jpg", response.CoverPath)
	assert.Equal(t, "/music/Metallica/Metallica/01 - Metallica - Enter Sandman.lrc", response.LyricsPath)
	assert.Equal(t, int64(35600000), response.FileSize)
	assert.Equal(t, 0, response.RetryCount)
	assert.Equal(t, float64(100), response.Progress)
	assert.Equal(t, int64(35600000), response.BytesDownloaded)
	assert.Equal(t, int64(35600000), response.BytesTotal)
	assert.Equal(t, startedAt.Format(time.RFC3339), response.StartedAt)
	assert.Equal(t, completedAt.Format(time.RFC3339), response.CompletedAt)
}
