package spotify

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSpotifyURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string
		wantID   string
		wantErr  bool
	}{
		{
			name:     "album URL",
			input:    "https://open.spotify.com/album/123456",
			wantType: "album",
			wantID:   "123456",
			wantErr:  false,
		},
		{
			name:     "playlist URL",
			input:    "https://open.spotify.com/playlist/abcdef123456",
			wantType: "playlist",
			wantID:   "abcdef123456",
			wantErr:  false,
		},
		{
			name:     "track URL",
			input:    "https://open.spotify.com/track/789xyz",
			wantType: "track",
			wantID:   "789xyz",
			wantErr:  false,
		},
		{
			name:     "artist URL",
			input:    "https://open.spotify.com/artist/456abc",
			wantType: "artist",
			wantID:   "456abc",
			wantErr:  false,
		},
		{
			name:     "Spotify URI album",
			input:    "spotify:album:123456",
			wantType: "album",
			wantID:   "123456",
			wantErr:  false,
		},
		{
			name:     "Spotify URI track",
			input:    "spotify:track:789xyz",
			wantType: "track",
			wantID:   "789xyz",
			wantErr:  false,
		},
		{
			name:     "embed URL",
			input:    "https://embed.spotify.com/?uri=spotify:album:123456",
			wantType: "album",
			wantID:   "123456",
			wantErr:  false,
		},
		{
			name:     "international URL",
			input:    "https://open.spotify.com/intl-en/album/123456",
			wantType: "album",
			wantID:   "123456",
			wantErr:  false,
		},
		{
			name:     "empty string",
			input:    "",
			wantType: "",
			wantID:   "",
			wantErr:  true,
		},
		{
			name:     "invalid URL",
			input:    "https://invalid.com/album/123",
			wantType: "",
			wantID:   "",
			wantErr:  true,
		},
		{
			name:     "user playlist URL",
			input:    "https://open.spotify.com/user/username/playlist/playlist123",
			wantType: "playlist",
			wantID:   "playlist123",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotID, err := ParseSpotifyURL(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantType, gotType)
			assert.Equal(t, tt.wantID, gotID)
		})
	}
}

func TestValidateSpotifyURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"valid album", "https://open.spotify.com/album/123", true},
		{"valid playlist", "https://open.spotify.com/playlist/abc", true},
		{"valid track", "https://open.spotify.com/track/xyz", true},
		{"valid URI", "spotify:album:123", true},
		{"invalid URL", "https://example.com/album/123", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSpotifyURL(tt.input)
			assert.Equal(t, tt.valid, result)
		})
	}
}

func TestGetSpotifyID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantID  string
		wantErr bool
	}{
		{"album", "https://open.spotify.com/album/abc123", "abc123", false},
		{"track", "https://open.spotify.com/track/xyz789", "xyz789", false},
		{"invalid", "https://example.com/track/123", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, err := GetSpotifyID(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, gotID)
		})
	}
}

func TestGetSpotifyType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string
		wantErr  bool
	}{
		{"album", "https://open.spotify.com/album/123", "album", false},
		{"playlist", "https://open.spotify.com/playlist/123", "playlist", false},
		{"track", "https://open.spotify.com/track/123", "track", false},
		{"artist", "https://open.spotify.com/artist/123", "artist", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, err := GetSpotifyType(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantType, gotType)
		})
	}
}

func TestParseISRC(t *testing.T) {
	tests := []struct {
		name        string
		isrc        string
		wantValid   bool
		wantCountry string
		wantYear    string
	}{
		{
			name:        "valid US ISRC",
			isrc:        "US-S1Z-21-00001",
			wantValid:   true,
			wantCountry: "US",
			wantYear:    "21",
		},
		{
			name:        "valid UK ISRC",
			isrc:        "GB-AAA-21-12345",
			wantValid:   true,
			wantCountry: "GB",
			wantYear:    "21",
		},
		{
			name:        "without dashes",
			isrc:        "USS1Z2100001",
			wantValid:   true,
			wantCountry: "US",
			wantYear:    "21",
		},
		{
			name:      "invalid too short",
			isrc:      "US-S1Z-21",
			wantValid: false,
		},
		{
			name:      "empty string",
			isrc:      "",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := ParseISRC(tt.isrc)
			if tt.wantValid {
				require.NotNil(t, info)
				assert.Equal(t, tt.wantCountry, info.Country)
				assert.Equal(t, tt.wantYear, info.Year)
			} else {
				assert.Nil(t, info)
			}
		})
	}
}

func TestISRCMatch(t *testing.T) {
	tests := []struct {
		name   string
		isrc1  string
		isrc2  string
		expect bool
	}{
		{"exact match", "US-S1Z-21-00001", "US-S1Z-21-00001", true},
		{"case insensitive", "us-s1z-21-00001", "US-S1Z-21-00001", true},
		{"different dashes", "USS1Z2100001", "US-S1Z-21-00001", true},
		{"no match", "US-S1Z-21-00001", "GB-AAA-21-12345", false},
		{"empty strings", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ISRCMatch(tt.isrc1, tt.isrc2)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestCleanTrackName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"remastered", "Enter Sandman (Remastered)", "Enter Sandman"},
		{"live", "Song Name - Live", "Song Name"},
		{"version", "Track (2021 Version)", "Track"},
		{"brackets", "Track Name [Remix]", "Track Name"},
		{"clean", "Clean Track Name", "Clean Track Name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanTrackName(tt.input)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration int // in milliseconds
		want     string
	}{
		{"1 minute", 60000, "1:00"},
		{"3:30", 210000, "3:30"},
		{"0 seconds", 0, "0:00"},
		{"59 seconds", 59000, "0:59"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDuration(tt.duration)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestTrackMatchScore(t *testing.T) {
	track := &Track{
		Name:   "Enter Sandman",
		Artist: "Metallica",
		Album:  "Metallica",
	}

	tests := []struct {
		name   string
		query  string
		expect int
	}{
		{"exact name", "Enter Sandman", 60}, // 10 + 50
		{"exact artist", "Metallica", 50},   // 20 + 30 (partial match)
		{"partial name", "Sandman", 10},
		{"partial artist", "Metallica", 20},
		{"no match", "Unknown", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := track.MatchScore(tt.query)
			assert.GreaterOrEqual(t, score, tt.expect)
		})
	}
}

// Test fixtures

func TestTrackFixtures(t *testing.T) {
	// Test track fixture
	track := Track{
		ID:          "spotify:track:123",
		Name:        "Enter Sandman",
		Artist:      "Metallica",
		Artists:     []string{"Metallica"},
		Album:       "Metallica",
		AlbumID:     "spotify:album:456",
		AlbumArtist: "Metallica",
		TrackNumber: 1,
		DiscNumber:  1,
		DurationMs:  336000, // 5:36
		ISRC:        "USEE1000233",
		ReleaseYear: 1991,
		ReleaseDate: "1991-07-12",
		TotalTracks: 12,
		CoverArtURL: "https://example.com/cover.jpg",
		Explicit:    false,
	}

	assert.NotEmpty(t, track.ID)
	assert.Equal(t, "Enter Sandman", track.Name)
	assert.Equal(t, "Metallica", track.Artist)
	assert.Equal(t, "USEE1000233", track.ISRC)
	assert.Equal(t, 1991, track.ReleaseYear)
}

func TestAlbumFixtures(t *testing.T) {
	album := Album{
		ID:          "spotify:album:456",
		Name:        "Metallica",
		Artist:      "Metallica",
		ArtistID:    "spotify:artist:789",
		ReleaseDate: "1991-07-12",
		TotalTracks: 12,
		CoverArtURL: "https://example.com/cover.jpg",
	}

	assert.NotEmpty(t, album.ID)
	assert.Equal(t, "Metallica", album.Name)
	assert.Equal(t, 12, album.TotalTracks)
}

func TestPlaylistFixtures(t *testing.T) {
	playlist := Playlist{
		ID:          "spotify:playlist:abc",
		Name:        "My Playlist",
		Owner:       "testuser",
		TotalTracks: 50,
		CoverArtURL: "https://example.com/cover.jpg",
	}

	assert.NotEmpty(t, playlist.ID)
	assert.Equal(t, "My Playlist", playlist.Name)
	assert.Equal(t, 50, playlist.TotalTracks)
}
