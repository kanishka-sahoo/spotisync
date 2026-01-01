package qobuz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTrackID(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantID  int64
		wantErr bool
	}{
		{
			name:    "play.qobuz.com URL",
			url:     "https://play.qobuz.com/track/1234567890",
			wantID:  1234567890,
			wantErr: false,
		},
		{
			name:    "URL with query params",
			url:     "https://play.qobuz.com/track/1234567890?albumId=456",
			wantID:  1234567890,
			wantErr: false,
		},
		{
			name:    "invalid format",
			url:     "https://example.com/track/123",
			wantID:  0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, err := ParseTrackID(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, gotID)
		})
	}
}

func TestGetQualityLabel(t *testing.T) {
	tests := []struct {
		name        string
		qualityCode int
		wantLabel   string
	}{
		{"Hi-Res", QualityHiRes, "Hi-Res"},
		{"FLAC 24-bit", QualityFLAC24, "FLAC 24-bit"},
		{"FLAC 16-bit", QualityFLAC16, "FLAC 16-bit"},
		{"MP3 320", QualityMP3, "MP3 320"},
		{"Unknown", 999, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetQualityLabel(tt.qualityCode)
			assert.Equal(t, tt.wantLabel, result)
		})
	}
}

func TestTrackFixtures(t *testing.T) {
	track := Track{
		ID:                  1234567890,
		Title:               "Enter Sandman",
		Version:             "",
		Duration:            336000,
		TrackNumber:         1,
		MediaNumber:         1,
		ISRC:                "USEE1000233",
		Copyright:           "© 1991 Elektra",
		MaximumBitDepth:     24,
		MaximumSamplingRate: 96.0,
		Hires:               true,
		HiresStreamable:     true,
		Performer: struct {
			Name string `json:"name"`
			ID   int64  `json:"id"`
		}{
			Name: "Metallica",
			ID:   12345,
		},
		Album: struct {
			Title string `json:"title"`
			ID    string `json:"id"`
			Image struct {
				Small     string `json:"small"`
				Thumbnail string `json:"thumbnail"`
				Large     string `json:"large"`
			} `json:"image"`
			Artist struct {
				Name string `json:"name"`
				ID   int64  `json:"id"`
			} `json:"artist"`
			Label struct {
				Name string `json:"name"`
			} `json:"label"`
		}{
			Title: "Metallica",
			ID:    "album-123",
			Image: struct {
				Small     string `json:"small"`
				Thumbnail string `json:"thumbnail"`
				Large     string `json:"large"`
			}{
				Large: "https://example.com/cover.jpg",
			},
			Artist: struct {
				Name string `json:"name"`
				ID   int64  `json:"id"`
			}{
				Name: "Metallica",
				ID:   12345,
			},
		},
	}

	assert.Equal(t, int64(1234567890), track.ID)
	assert.Equal(t, "Enter Sandman", track.Title)
	assert.Equal(t, "USEE1000233", track.ISRC)
	assert.True(t, track.Hires)
	assert.True(t, track.HiresStreamable)
	assert.Equal(t, 24, track.MaximumBitDepth)
	assert.Equal(t, 96.0, track.MaximumSamplingRate)
}

func TestTrackToTrackInfo(t *testing.T) {
	track := Track{
		ID:                  123,
		Title:               "Test Track",
		Duration:            180000,
		ISRC:                "US-S1Z-21-00001",
		MaximumBitDepth:     24,
		MaximumSamplingRate: 96.0,
		Hires:               true,
		HiresStreamable:     true,
		Album: struct {
			Title string `json:"title"`
			ID    string `json:"id"`
			Image struct {
				Small     string `json:"small"`
				Thumbnail string `json:"thumbnail"`
				Large     string `json:"large"`
			} `json:"image"`
			Artist struct {
				Name string `json:"name"`
				ID   int64  `json:"id"`
			} `json:"artist"`
			Label struct {
				Name string `json:"name"`
			} `json:"label"`
		}{
			Image: struct {
				Small     string `json:"small"`
				Thumbnail string `json:"thumbnail"`
				Large     string `json:"large"`
			}{
				Large: "https://example.com/cover.jpg",
			},
		},
	}

	info := track.ToTrackInfo()

	assert.Equal(t, track.ID, info.ID)
	assert.Equal(t, track.Title, info.Title)
	assert.Equal(t, 180000, info.DurationSecs)
	assert.Equal(t, "Hi-Res", info.QualityLabel)
	assert.Equal(t, QualityHiRes, info.QualityCode)
	assert.Equal(t, "24-bit / 96.0 kHz", info.HiResInfo)
	assert.Equal(t, "https://example.com/cover.jpg", info.CoverURL)
}
