package tidal

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
			name:    "listen.tidal.com URL",
			url:     "https://listen.tidal.com/track/441821360",
			wantID:  441821360,
			wantErr: false,
		},
		{
			name:    "tidal.com URL",
			url:     "https://tidal.com/browse/track/123456789",
			wantID:  123456789,
			wantErr: false,
		},
		{
			name:    "URL with query params",
			url:     "https://listen.tidal.com/track/441821360?albumId=456",
			wantID:  441821360,
			wantErr: false,
		},
		{
			name:    "invalid format",
			url:     "https://example.com/track/123",
			wantID:  0,
			wantErr: true,
		},
		{
			name:    "empty URL",
			url:     "",
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

func TestTrackFixtures(t *testing.T) {
	track := Track{
		ID:           441821360,
		Title:        "Enter Sandman",
		ISRC:         "USEE1000233",
		AudioQuality: "HI_RES_LOSSLESS",
		TrackNumber:  1,
		VolumeNumber: 1,
		Duration:     336000,
		Copyright:    "© 1991 Elektra",
		Explicit:     false,
		Album: struct {
			Title       string `json:"title"`
			Cover       string `json:"cover"`
			ReleaseDate string `json:"releaseDate"`
		}{
			Title:       "Metallica",
			Cover:       "album-cover-id",
			ReleaseDate: "1991-07-12",
		},
		Artists: []struct {
			Name string `json:"name"`
		}{
			{Name: "Metallica"},
		},
		Artist: struct {
			Name string `json:"name"`
		}{
			Name: "Metallica",
		},
		MediaMetadata: struct {
			Tags []string `json:"tags"`
		}{
			Tags: []string{"HIRES_LOSSLESS"},
		},
	}

	assert.Equal(t, int64(441821360), track.ID)
	assert.Equal(t, "Enter Sandman", track.Title)
	assert.Equal(t, "USEE1000233", track.ISRC)
	assert.Equal(t, "HI_RES_LOSSLESS", track.AudioQuality)
	assert.True(t, track.Hires())
}

func TestTrackHires(t *testing.T) {
	tests := []struct {
		name  string
		tags  []string
		hires bool
	}{
		{
			name:  "with hires tags",
			tags:  []string{"HIRES_LOSSLESS"},
			hires: true,
		},
		{
			name:  "with hires tag",
			tags:  []string{"HIRES"},
			hires: true,
		},
		{
			name:  "without hires tags",
			tags:  []string{"LOSSLESS"},
			hires: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			track := Track{
				MediaMetadata: struct {
					Tags []string `json:"tags"`
				}{
					Tags: tt.tags,
				},
			}
			assert.Equal(t, tt.hires, track.Hires())
		})
	}
}

func TestTrackToTrackInfo(t *testing.T) {
	track := Track{
		ID:           123,
		Title:        "Test Track",
		ISRC:         "US-S1Z-21-00001",
		AudioQuality: "HI_RES_LOSSLESS",
		Duration:     180000,
		Album: struct {
			Title       string `json:"title"`
			Cover       string `json:"cover"`
			ReleaseDate string `json:"releaseDate"`
		}{
			Title: "Test Album",
			Cover: "cover-123",
		},
		Artist: struct {
			Name string `json:"name"`
		}{
			Name: "Test Artist",
		},
		MediaMetadata: struct {
			Tags []string `json:"tags"`
		}{
			Tags: []string{"HIRES_LOSSLESS"},
		},
	}

	info := track.ToTrackInfo()

	assert.Equal(t, track.ID, info.ID)
	assert.Equal(t, track.Title, info.Title)
	assert.Equal(t, "cover-123", info.CoverURL)
	assert.Equal(t, 180, info.DurationSecs)
	assert.Equal(t, "Hi-Res", info.QualityInfo)
	assert.True(t, info.HiResCapable)
}

func TestGetAlbumArtURL(t *testing.T) {
	client := NewClient("test-id", "test-secret")

	tests := []struct {
		name    string
		coverID string
		size    int
		wantURL string
	}{
		{
			name:    "with dashes",
			coverID: "ab67616d0000b273",
			size:    1280,
			wantURL: "https://resources.tidal.com/images/ab67616d/0000b273/1280x1280.jpg",
		},
		{
			name:    "default size",
			coverID: "ab67616d0000b273",
			size:    0,
			wantURL: "https://resources.tidal.com/images/ab67616d/0000b273/1280x1280.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.GetAlbumArtURL(tt.coverID, tt.size)
			assert.Contains(t, result, "tidal.com/images")
		})
	}
}
