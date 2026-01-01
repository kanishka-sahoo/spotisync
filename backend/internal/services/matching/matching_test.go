package matching

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateStats(t *testing.T) {
	results := []*MatchResult{
		{Found: true, Source: SourceTidal, Quality: "Hi-Res"},
		{Found: true, Source: SourceTidal, Quality: "LOSSLESS"},
		{Found: true, Source: SourceQobuz, Quality: "Hi-Res"},
		{Found: true, Source: SourceQobuz, Quality: "FLAC"},
		{NotFound: true},
		{Error: assert.AnError},
	}

	stats := CalculateStats(results)

	assert.Equal(t, 6, stats.Total)
	assert.Equal(t, 2, stats.FoundTidal)
	assert.Equal(t, 2, stats.FoundQobuz)
	assert.Equal(t, 1, stats.NotFound)
	assert.Equal(t, 1, stats.Errors)
	assert.Equal(t, 2, stats.HiResCount)            // Hi-Res from Tidal and Qobuz
	assert.InDelta(t, 80.0, stats.SuccessRate, 0.1) // 4 found out of 5 processed (excluding error)
}

func TestFormatStats(t *testing.T) {
	stats := &MatchStats{
		Total:       100,
		FoundTidal:  40,
		FoundQobuz:  30,
		NotFound:    20,
		Errors:      10,
		HiResCount:  25,
		SuccessRate: 70.0,
	}

	result := FormatStats(stats)
	assert.Contains(t, result, "70.0%")
	assert.Contains(t, result, "40")
	assert.Contains(t, result, "30")
	assert.Contains(t, result, "25")
	assert.Contains(t, result, "20")
}

func TestTrackMetadataFixtures(t *testing.T) {
	metadata := &TrackMetadata{
		SpotifyID:   "spotify:track:123",
		Name:        "Enter Sandman",
		Artist:      "Metallica",
		Album:       "Metallica",
		AlbumArtist: "Metallica",
		ISRC:        "USEE1000233",
		DurationMs:  336000,
		ReleaseYear: 1991,
		ReleaseDate: "1991-07-12",
		TrackNumber: 1,
		TotalTracks: 12,
		CoverArtURL: "https://example.com/cover.jpg",
	}

	assert.Equal(t, "spotify:track:123", metadata.SpotifyID)
	assert.Equal(t, "Enter Sandman", metadata.Name)
	assert.Equal(t, "Metallica", metadata.Artist)
	assert.Equal(t, "USEE1000233", metadata.ISRC)
	assert.Equal(t, 336000, metadata.DurationMs)
	assert.Equal(t, 1991, metadata.ReleaseYear)
}

func TestSourceService(t *testing.T) {
	assert.Equal(t, SourceService("tidal"), SourceTidal)
	assert.Equal(t, SourceService("qobuz"), SourceQobuz)
}

func TestMatchResultFixtures(t *testing.T) {
	// Success result
	successResult := &MatchResult{
		Found:   true,
		Source:  SourceTidal,
		TrackID: "123456",
		Quality: "Hi-Res",
		ISRC:    "USEE1000233",
	}

	assert.True(t, successResult.Found)
	assert.Equal(t, SourceTidal, successResult.Source)
	assert.Equal(t, "123456", successResult.TrackID)
	assert.Equal(t, "Hi-Res", successResult.Quality)
	assert.Nil(t, successResult.Error)
	assert.False(t, successResult.NotFound)

	// Not found result
	notFoundResult := &MatchResult{
		Found:    false,
		NotFound: true,
		Error:    nil,
	}

	assert.False(t, notFoundResult.Found)
	assert.True(t, notFoundResult.NotFound)
	assert.Nil(t, notFoundResult.Error)

	// Error result
	errorResult := &MatchResult{
		Found:    false,
		NotFound: false,
		Error:    assert.AnError,
	}

	assert.False(t, errorResult.Found)
	assert.False(t, errorResult.NotFound)
	assert.Equal(t, assert.AnError, errorResult.Error)
}
