package spotify

import (
	"context"
)

// MockSpotifyClient is a mock implementation of SpotifyClientInterface for testing.
type MockSpotifyClient struct {
	// Tracks to return from GetTracksFromURL
	TracksToReturn []Track
	// Name to return from GetTracksFromURL (album/playlist/artist name)
	NameToReturn string
	// Error to return from any method
	ErrorToReturn error
	// Track to return from GetTrack
	TrackToReturn *Track
	// AlbumResult to return from GetAlbumTracks
	AlbumResultToReturn *AlbumResult
	// PlaylistResult to return from GetPlaylistTracks
	PlaylistResultToReturn *PlaylistResult
	// DiscographyResult to return from GetArtistDiscography
	DiscographyResultToReturn *DiscographyResult
	// Record of calls made
	GetTracksFromURLCalls []string
}

// Ensure MockSpotifyClient implements SpotifyClientInterface
var _ SpotifyClientInterface = (*MockSpotifyClient)(nil)

// NewMockSpotifyClient creates a new mock Spotify client with default test data.
func NewMockSpotifyClient() *MockSpotifyClient {
	return &MockSpotifyClient{
		TracksToReturn: []Track{
			{
				ID:          "mock-track-id-1",
				Name:        "Mock Track 1",
				Artist:      "Mock Artist",
				Artists:     []string{"Mock Artist"},
				Album:       "Mock Album",
				AlbumID:     "mock-album-id",
				AlbumArtist: "Mock Artist",
				TrackNumber: 1,
				DiscNumber:  1,
				DurationMs:  180000,
				ISRC:        "USMOCK0000001",
				ReleaseYear: 2024,
				ReleaseDate: "2024-01-15",
				TotalTracks: 10,
				CoverArtURL: "https://example.com/cover.jpg",
				Explicit:    false,
			},
		},
		NameToReturn: "Mock Resource",
	}
}

// GetTrack returns a mock track or configured error.
func (m *MockSpotifyClient) GetTrack(ctx context.Context, trackID string) (*Track, error) {
	if m.ErrorToReturn != nil {
		return nil, m.ErrorToReturn
	}
	if m.TrackToReturn != nil {
		return m.TrackToReturn, nil
	}
	if len(m.TracksToReturn) > 0 {
		return &m.TracksToReturn[0], nil
	}
	return nil, ErrNotFound
}

// GetAlbumTracks returns mock album tracks or configured error.
func (m *MockSpotifyClient) GetAlbumTracks(ctx context.Context, albumID string) (*AlbumResult, error) {
	if m.ErrorToReturn != nil {
		return nil, m.ErrorToReturn
	}
	if m.AlbumResultToReturn != nil {
		return m.AlbumResultToReturn, nil
	}
	return &AlbumResult{
		Name:        m.NameToReturn,
		Artist:      "Mock Artist",
		ReleaseDate: "2024-01-15",
		CoverArtURL: "https://example.com/cover.jpg",
		Tracks:      m.TracksToReturn,
	}, nil
}

// GetPlaylistTracks returns mock playlist tracks or configured error.
func (m *MockSpotifyClient) GetPlaylistTracks(ctx context.Context, playlistID string) (*PlaylistResult, error) {
	if m.ErrorToReturn != nil {
		return nil, m.ErrorToReturn
	}
	if m.PlaylistResultToReturn != nil {
		return m.PlaylistResultToReturn, nil
	}
	return &PlaylistResult{
		Name:        m.NameToReturn,
		Owner:       "Mock Owner",
		CoverArtURL: "https://example.com/cover.jpg",
		Tracks:      m.TracksToReturn,
	}, nil
}

// GetArtistDiscography returns mock artist tracks or configured error.
func (m *MockSpotifyClient) GetArtistDiscography(ctx context.Context, artistID string, preview bool) (*DiscographyResult, error) {
	if m.ErrorToReturn != nil {
		return nil, m.ErrorToReturn
	}
	if m.DiscographyResultToReturn != nil {
		return m.DiscographyResultToReturn, nil
	}
	return &DiscographyResult{
		Name:        m.NameToReturn,
		Tracks:      m.TracksToReturn,
		Fetched:     len(m.TracksToReturn),
		TotalTracks: len(m.TracksToReturn),
		Failed:      0,
	}, nil
}

// GetTracksFromURL returns mock tracks and name or configured error.
func (m *MockSpotifyClient) GetTracksFromURL(ctx context.Context, spotifyURL string) ([]Track, string, error) {
	m.GetTracksFromURLCalls = append(m.GetTracksFromURLCalls, spotifyURL)
	if m.ErrorToReturn != nil {
		return nil, "", m.ErrorToReturn
	}
	return m.TracksToReturn, m.NameToReturn, nil
}
