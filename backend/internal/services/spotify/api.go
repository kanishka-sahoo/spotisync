// Package spotify provides a client for interacting with the Spotify Web API.
package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// API Constants
const (
	spotifyTokenURL    = "https://accounts.spotify.com/api/token"
	spotifyAPIBaseURL  = "https://api.spotify.com/v1"
	defaultHTTPTimeout = 30 * time.Second
	tokenRefreshBuffer = 60 * time.Second // Refresh token 60 seconds before expiry
	defaultRetryAfter  = 5 * time.Second
	maxRetries         = 3
)

// Default Spotify API credentials (same as SpotiFLAC)
const (
	defaultSpotifyClientID     = "5f573c9620494bae87890c0f08a60293"
	defaultSpotifyClientSecret = "212476d9b0f3472eaa762d90b19b0ba8"
)

// Error definitions
var (
	ErrInvalidCredentials = errors.New("invalid Spotify credentials")
	ErrNotFound           = errors.New("resource not found")
	ErrRateLimited        = errors.New("rate limited by Spotify API")
	ErrAuthFailed         = errors.New("authentication failed")
	ErrInvalidURL         = errors.New("invalid Spotify URL")
)

// AlbumResult contains album metadata and all its tracks.
type AlbumResult struct {
	Name        string
	Artist      string
	ReleaseDate string
	CoverArtURL string
	Tracks      []Track
}

// DiscographyResult contains partial results for artist discography with warning
type DiscographyResult struct {
	Name        string
	Tracks      []Track
	Fetched     int
	Failed      int
	Warning     string
	TotalTracks int
}

// PlaylistResult contains playlist metadata and all its tracks.
type PlaylistResult struct {
	Name        string
	Owner       string
	CoverArtURL string
	Tracks      []Track
}

// ArtistResult contains artist metadata and all tracks from their discography.
type ArtistResult struct {
	Name   string
	Tracks []Track
}

// SpotifyClientInterface defines the contract for Spotify API operations.
// This interface allows for mocking in tests.
type SpotifyClientInterface interface {
	GetTrack(ctx context.Context, trackID string) (*Track, error)
	GetAlbumTracks(ctx context.Context, albumID string) (*AlbumResult, error)
	GetPlaylistTracks(ctx context.Context, playlistID string) (*PlaylistResult, error)
	GetArtistDiscography(ctx context.Context, artistID string, preview bool) (*DiscographyResult, error)
	GetTracksFromURL(ctx context.Context, spotifyURL string) ([]Track, string, error)
}

// Ensure APIClient implements SpotifyClientInterface
var _ SpotifyClientInterface = (*APIClient)(nil)

// APIClient handles authenticated requests to the Spotify Web API.
type APIClient struct {
	httpClient   *http.Client
	clientID     string
	clientSecret string
	cachedToken  string
	tokenExpiry  time.Time
	mu           sync.RWMutex
	isrcCache    map[string]string
	isrcCacheMu  sync.RWMutex
}

// NewAPIClient creates a new Spotify API client with the provided credentials.
// If clientID or clientSecret are empty, built-in defaults are used.
func NewAPIClient(clientID, clientSecret string) *APIClient {
	// Use defaults if not provided
	if clientID == "" {
		clientID = defaultSpotifyClientID
	}
	if clientSecret == "" {
		clientSecret = defaultSpotifyClientSecret
	}

	return &APIClient{
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
		clientID:     clientID,
		clientSecret: clientSecret,
		isrcCache:    make(map[string]string),
	}
}

// Spotify API response structures (internal use)
type accessTokenResponse struct {
	AccessToken string      `json:"access_token"`
	TokenType   string      `json:"token_type"`
	ExpiresIn   interface{} `json:"expires_in"` // Can be number or string
}

type spotifyImage struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

type spotifyArtist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type spotifyExternalIDs struct {
	ISRC string `json:"isrc"`
}

type spotifyExternalURLs struct {
	Spotify string `json:"spotify"`
}

type spotifyAlbumSimplified struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	AlbumType   string              `json:"album_type"`
	ReleaseDate string              `json:"release_date"`
	TotalTracks int                 `json:"total_tracks"`
	Images      []spotifyImage      `json:"images"`
	Artists     []spotifyArtist     `json:"artists"`
	ExternalURL spotifyExternalURLs `json:"external_urls"`
}

type spotifyTrackSimplified struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	DurationMS  int                 `json:"duration_ms"`
	TrackNumber int                 `json:"track_number"`
	DiscNumber  int                 `json:"disc_number"`
	Explicit    bool                `json:"explicit"`
	Artists     []spotifyArtist     `json:"artists"`
	ExternalURL spotifyExternalURLs `json:"external_urls"`
}

type spotifyTrackFull struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	DurationMS  int                    `json:"duration_ms"`
	TrackNumber int                    `json:"track_number"`
	DiscNumber  int                    `json:"disc_number"`
	Explicit    bool                   `json:"explicit"`
	Artists     []spotifyArtist        `json:"artists"`
	Album       spotifyAlbumSimplified `json:"album"`
	ExternalIDs spotifyExternalIDs     `json:"external_ids"`
	ExternalURL spotifyExternalURLs    `json:"external_urls"`
}

type spotifyAlbumResponse struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	AlbumType   string              `json:"album_type"`
	ReleaseDate string              `json:"release_date"`
	TotalTracks int                 `json:"total_tracks"`
	Images      []spotifyImage      `json:"images"`
	Artists     []spotifyArtist     `json:"artists"`
	ExternalURL spotifyExternalURLs `json:"external_urls"`
	Tracks      struct {
		Items []spotifyTrackSimplified `json:"items"`
		Next  string                   `json:"next"`
		Total int                      `json:"total"`
	} `json:"tracks"`
}

type spotifyPlaylistResponse struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Images      []spotifyImage      `json:"images"`
	ExternalURL spotifyExternalURLs `json:"external_urls"`
	Owner       struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"owner"`
	Tracks struct {
		Items []struct {
			Track *spotifyTrackFull `json:"track"`
		} `json:"items"`
		Next  string `json:"next"`
		Total int    `json:"total"`
	} `json:"tracks"`
}

type spotifyArtistResponse struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Genres      []string            `json:"genres"`
	Images      []spotifyImage      `json:"images"`
	ExternalURL spotifyExternalURLs `json:"external_urls"`
	Followers   struct {
		Total int `json:"total"`
	} `json:"followers"`
	Popularity int `json:"popularity"`
}

// Generic paging response for fetching paginated data
type pagingResponse[T any] struct {
	Items []T    `json:"items"`
	Next  string `json:"next"`
	Total int    `json:"total"`
}

// getAccessToken retrieves a valid access token, refreshing if necessary.
func (c *APIClient) getAccessToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	if c.cachedToken != "" && time.Now().Add(tokenRefreshBuffer).Before(c.tokenExpiry) {
		token := c.cachedToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	// Need to refresh token
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.cachedToken != "" && time.Now().Add(tokenRefreshBuffer).Before(c.tokenExpiry) {
		return c.cachedToken, nil
	}

	// Request new token
	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, spotifyTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	// Set Basic Auth header with clientID:clientSecret
	req.SetBasicAuth(c.clientID, c.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return "", ErrInvalidCredentials
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: status %d, response: %s", ErrAuthFailed, resp.StatusCode, string(body))
	}

	var tokenResp accessTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("%w: empty token received", ErrAuthFailed)
	}

	// Cache the token
	c.cachedToken = tokenResp.AccessToken

	// Parse expires_in (can be float64 or string)
	var expiresIn int
	switch v := tokenResp.ExpiresIn.(type) {
	case float64:
		expiresIn = int(v)
	case string:
		expiresIn, _ = strconv.Atoi(v)
	default:
		expiresIn = 3600 // Default to 1 hour
	}

	c.tokenExpiry = time.Now().Add(time.Duration(expiresIn) * time.Second)

	return c.cachedToken, nil
}

// doRequest performs an authenticated HTTP request with retry and rate limit handling.
func (c *APIClient) doRequest(ctx context.Context, method, endpoint string) ([]byte, error) {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		switch resp.StatusCode {
		case http.StatusOK:
			return body, nil

		case http.StatusTooManyRequests:
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			log.Printf("Rate limited by Spotify API, retrying after %v (attempt %d/%d)", retryAfter, attempt+1, maxRetries)
			if err := sleepWithContext(ctx, retryAfter); err != nil {
				return nil, err
			}
			lastErr = ErrRateLimited
			continue

		case http.StatusNotFound:
			return nil, ErrNotFound

		case http.StatusUnauthorized:
			// Token might have expired, clear cache and retry
			c.mu.Lock()
			c.cachedToken = ""
			c.tokenExpiry = time.Time{}
			c.mu.Unlock()

			token, err = c.getAccessToken(ctx)
			if err != nil {
				return nil, err
			}
			lastErr = ErrAuthFailed
			continue

		default:
			return nil, fmt.Errorf("Spotify API error: status %d, body: %s", resp.StatusCode, string(body))
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries, lastErr)
	}
	return nil, errors.New("request failed after maximum retries")
}

// getJSON fetches JSON from an endpoint and unmarshals it into dst.
func (c *APIClient) getJSON(ctx context.Context, endpoint string, dst interface{}) error {
	body, err := c.doRequest(ctx, http.MethodGet, endpoint)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dst)
}

// fetchPaging fetches all pages of a paginated endpoint and collects items.
func fetchPaging[T any](ctx context.Context, c *APIClient, startURL string, dest *[]T) error {
	nextURL := startURL
	for nextURL != "" {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var page pagingResponse[T]
		if err := c.getJSON(ctx, nextURL, &page); err != nil {
			return err
		}

		*dest = append(*dest, page.Items...)
		nextURL = stripLocaleParam(page.Next)
	}
	return nil
}

// GetTrack retrieves a single track by its Spotify ID.
func (c *APIClient) GetTrack(ctx context.Context, trackID string) (*Track, error) {
	if trackID == "" {
		return nil, errors.New("track ID cannot be empty")
	}

	endpoint := fmt.Sprintf("%s/tracks/%s", spotifyAPIBaseURL, trackID)

	var resp spotifyTrackFull
	if err := c.getJSON(ctx, endpoint, &resp); err != nil {
		return nil, fmt.Errorf("failed to get track %s: %w", trackID, err)
	}

	track := c.convertFullTrack(&resp)
	return &track, nil
}

// GetAlbumTracks retrieves all tracks from an album.
func (c *APIClient) GetAlbumTracks(ctx context.Context, albumID string) (*AlbumResult, error) {
	return c.getAlbumTracksInternal(ctx, albumID, false)
}

// GetAlbumTracksNoISRC retrieves all tracks from an album without fetching ISRCs.
func (c *APIClient) GetAlbumTracksNoISRC(ctx context.Context, albumID string) (*AlbumResult, error) {
	return c.getAlbumTracksInternal(ctx, albumID, true)
}

// getAlbumTracksInternal is the internal implementation that handles the skipISRC parameter.
func (c *APIClient) getAlbumTracksInternal(ctx context.Context, albumID string, skipISRC bool) (*AlbumResult, error) {
	if albumID == "" {
		return nil, errors.New("album ID cannot be empty")
	}

	// First, get the album metadata
	albumEndpoint := fmt.Sprintf("%s/albums/%s", spotifyAPIBaseURL, albumID)

	var albumResp spotifyAlbumResponse
	if err := c.getJSON(ctx, albumEndpoint, &albumResp); err != nil {
		return nil, fmt.Errorf("failed to get album %s: %w", albumID, err)
	}

	// Collect all tracks (handle pagination if needed)
	var simplifiedTracks []spotifyTrackSimplified
	simplifiedTracks = append(simplifiedTracks, albumResp.Tracks.Items...)

	// Fetch remaining pages if there are more tracks
	if albumResp.Tracks.Next != "" {
		if err := fetchPaging(ctx, c, albumResp.Tracks.Next, &simplifiedTracks); err != nil {
			return nil, fmt.Errorf("failed to fetch album tracks: %w", err)
		}
	}

	// Extract album metadata
	albumArtists := extractArtistNames(albumResp.Artists)
	albumArtist := ""
	if len(albumArtists) > 0 {
		albumArtist = albumArtists[0] // First artist only for directory naming
	}
	coverArtURL := getFirstImageURL(albumResp.Images)
	releaseYear := extractYear(albumResp.ReleaseDate)

	// Convert simplified tracks to full Track objects
	// Note: Simplified tracks don't have ISRC, so we need to fetch each track individually
	tracks := make([]Track, 0, len(simplifiedTracks))
	for _, st := range simplifiedTracks {
		var isrc string
		if !skipISRC {
			// Fetch ISRC for each track (with caching)
			isrc = c.fetchTrackISRC(ctx, st.ID)
		}

		artistNames := extractArtistNames(st.Artists)
		primaryArtist := ""
		if len(artistNames) > 0 {
			primaryArtist = artistNames[0]
		}

		track := Track{
			ID:           st.ID,
			Name:         st.Name,
			Artist:       primaryArtist,
			Artists:      artistNames,
			Album:        albumResp.Name,
			AlbumID:      albumResp.ID,
			AlbumArtist:  albumArtist,
			AlbumArtists: albumArtists,
			TrackNumber:  st.TrackNumber,
			DiscNumber:   st.DiscNumber,
			DurationMs:   st.DurationMS,
			ISRC:         isrc,
			ReleaseYear:  releaseYear,
			ReleaseDate:  albumResp.ReleaseDate,
			TotalTracks:  albumResp.TotalTracks,
			CoverArtURL:  coverArtURL,
			Explicit:     st.Explicit,
		}
		tracks = append(tracks, track)
	}

	return &AlbumResult{
		Name:        albumResp.Name,
		Artist:      albumArtist,
		ReleaseDate: albumResp.ReleaseDate,
		CoverArtURL: coverArtURL,
		Tracks:      tracks,
	}, nil
}

// GetPlaylistTracks retrieves all tracks from a playlist.
func (c *APIClient) GetPlaylistTracks(ctx context.Context, playlistID string) (*PlaylistResult, error) {
	if playlistID == "" {
		return nil, errors.New("playlist ID cannot be empty")
	}

	// Get playlist metadata
	playlistEndpoint := fmt.Sprintf("%s/playlists/%s", spotifyAPIBaseURL, playlistID)

	var playlistResp spotifyPlaylistResponse
	if err := c.getJSON(ctx, playlistEndpoint, &playlistResp); err != nil {
		return nil, fmt.Errorf("failed to get playlist %s: %w", playlistID, err)
	}

	// Collect tracks from initial response
	var playlistItems []struct {
		Track *spotifyTrackFull `json:"track"`
	}
	playlistItems = append(playlistItems, playlistResp.Tracks.Items...)

	// Fetch remaining pages
	if playlistResp.Tracks.Next != "" {
		tracksEndpoint := playlistResp.Tracks.Next
		for tracksEndpoint != "" {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			var page struct {
				Items []struct {
					Track *spotifyTrackFull `json:"track"`
				} `json:"items"`
				Next string `json:"next"`
			}

			if err := c.getJSON(ctx, tracksEndpoint, &page); err != nil {
				return nil, fmt.Errorf("failed to fetch playlist tracks: %w", err)
			}

			playlistItems = append(playlistItems, page.Items...)
			tracksEndpoint = stripLocaleParam(page.Next)
		}
	}

	// Convert to Track objects
	// Playlist tracks already have full track info including ISRC
	tracks := make([]Track, 0, len(playlistItems))
	for _, item := range playlistItems {
		if item.Track == nil {
			continue // Skip null tracks (can happen with local files or unavailable tracks)
		}
		track := c.convertFullTrack(item.Track)
		tracks = append(tracks, track)
	}

	coverArtURL := getFirstImageURL(playlistResp.Images)
	ownerName := playlistResp.Owner.DisplayName
	if ownerName == "" {
		ownerName = playlistResp.Owner.ID
	}

	return &PlaylistResult{
		Name:        playlistResp.Name,
		Owner:       ownerName,
		CoverArtURL: coverArtURL,
		Tracks:      tracks,
	}, nil
}

// GetArtistDiscography retrieves all tracks from all albums by an artist.
// If preview is true, it uses an optimized path that:
//   - Fetches only album metadata (no tracks) to count total tracks
//   - Returns only the first 20 tracks
//   - Skips delays for faster preview
//
// For non-preview mode (full download), it fetches all tracks with rate limiting delays.
// Returns partial results with a warning if timeout occurs for large discographies.
func (c *APIClient) GetArtistDiscography(ctx context.Context, artistID string, preview bool) (*DiscographyResult, error) {
	if artistID == "" {
		return nil, errors.New("artist ID cannot be empty")
	}

	// Get artist info
	artistEndpoint := fmt.Sprintf("%s/artists/%s", spotifyAPIBaseURL, artistID)

	var artistResp spotifyArtistResponse
	if err := c.getJSON(ctx, artistEndpoint, &artistResp); err != nil {
		return nil, fmt.Errorf("failed to get artist %s: %w", artistID, err)
	}

	// Preview mode: optimized path
	if preview {
		log.Printf("Starting preview discography fetch for artist: %s", artistID)

		// Fetch all albums (simplified, without tracks) to get album metadata and count total tracks
		albumsEndpoint := fmt.Sprintf("%s/artists/%s/albums?include_groups=album,single,compilation&limit=50", spotifyAPIBaseURL, artistID)

		var albums []spotifyAlbumSimplified
		if err := fetchPaging(ctx, c, albumsEndpoint, &albums); err != nil {
			return nil, fmt.Errorf("failed to fetch artist albums: %w", err)
		}

		// Count total tracks from album metadata
		totalTracks := 0
		for _, album := range albums {
			totalTracks += album.TotalTracks
		}
		log.Printf("Found %d albums with %d total tracks", len(albums), totalTracks)

		// Collect first 20 tracks by traversing albums in order
		var previewTracks []Track
		collected := 0
		for _, album := range albums {
			if collected >= 20 {
				break
			}

			log.Printf("Fetching tracks for album %d/%d: %s", len(previewTracks)+1, len(albums), album.Name)

			// Fetch album tracks (without ISRC for speed)
			albumResult, err := c.GetAlbumTracksNoISRC(ctx, album.ID)
			if err != nil {
				log.Printf("Warning: failed to fetch tracks for album %s: %v", album.Name, err)
				continue
			}

			// Add tracks from this album until we have 20 total
			remaining := 20 - collected
			if len(albumResult.Tracks) <= remaining {
				previewTracks = append(previewTracks, albumResult.Tracks...)
				collected += len(albumResult.Tracks)
			} else {
				previewTracks = append(previewTracks, albumResult.Tracks[:remaining]...)
				collected = 20
			}
		}

		log.Printf("Preview fetch complete: collected %d of %d tracks", collected, totalTracks)

		return &DiscographyResult{
			Name:        artistResp.Name,
			Tracks:      previewTracks,
			Fetched:     len(albums),
			TotalTracks: totalTracks,
		}, nil
	}

	// Full download mode (existing implementation with rate limiting)
	// Get all albums (albums, singles, compilations)
	albumsEndpoint := fmt.Sprintf("%s/artists/%s/albums?include_groups=album,single,compilation&limit=50", spotifyAPIBaseURL, artistID)

	var albums []spotifyAlbumSimplified
	if err := fetchPaging(ctx, c, albumsEndpoint, &albums); err != nil {
		return nil, fmt.Errorf("failed to fetch artist albums: %w", err)
	}

	// Collect all tracks from all albums with rate limiting and retries
	var allTracks []Track
	fetched := 0
	failed := 0

	for i, album := range albums {
		select {
		case <-ctx.Done():
			// Context cancelled/timeout - return partial results with warning
			warning := fmt.Sprintf("timeout after fetching %d of %d albums (failed: %d)", fetched, len(albums), failed)
			return &DiscographyResult{
				Name:        artistResp.Name,
				Tracks:      allTracks,
				Fetched:     fetched,
				Failed:      failed + (len(albums) - i),
				Warning:     warning,
				TotalTracks: len(allTracks),
			}, nil
		default:
		}

		// Add delay between each album fetch (200-500ms) to avoid hitting rate limits
		if i > 0 {
			delay := time.Duration(200+rand.Intn(300)) * time.Millisecond
			if err := sleepWithContext(ctx, delay); err != nil {
				warning := fmt.Sprintf("context cancelled after fetching %d of %d albums (failed: %d)", fetched, len(albums), failed)
				return &DiscographyResult{
					Name:        artistResp.Name,
					Tracks:      allTracks,
					Fetched:     fetched,
					Failed:      failed + (len(albums) - i),
					Warning:     warning,
					TotalTracks: len(allTracks),
				}, nil
			}
		}

		// Fetch album tracks with retry for rate limit errors
		var albumResult *AlbumResult
		var fetchErr error
		maxRetries := 3
		baseDelay := 1 * time.Second

		for attempt := 0; attempt < maxRetries; attempt++ {
			albumResult, fetchErr = c.GetAlbumTracks(ctx, album.ID)

			if fetchErr == nil {
				break
			}

			// Check if it's a rate limit error
			if errors.Is(fetchErr, ErrRateLimited) {
				delay := baseDelay * time.Duration(1<<attempt) // exponential backoff
				log.Printf("Rate limited, retrying after %v (attempt %d/%d)", delay, attempt+1, maxRetries)
				if err := sleepWithContext(ctx, delay); err != nil {
					break
				}
				continue
			}

			// For non-rate-limit errors, log and continue to next album
			log.Printf("Warning: failed to get tracks for album %s (%s): %v", album.Name, album.ID, fetchErr)
			failed++
			break
		}

		if albumResult != nil {
			allTracks = append(allTracks, albumResult.Tracks...)
			fetched++
		} else if fetchErr != nil && !errors.Is(fetchErr, ErrRateLimited) {
			// Already counted in the inner loop
		}
	}

	// Check if there were failures
	if failed > 0 {
		warning := fmt.Sprintf("failed to fetch %d of %d albums", failed, len(albums))
		return &DiscographyResult{
			Name:        artistResp.Name,
			Tracks:      allTracks,
			Fetched:     fetched,
			Failed:      failed,
			Warning:     warning,
			TotalTracks: len(allTracks),
		}, nil
	}

	return &DiscographyResult{
		Name:        artistResp.Name,
		Tracks:      allTracks,
		Fetched:     fetched,
		TotalTracks: len(allTracks),
	}, nil
}

// GetTracksFromURL parses a Spotify URL and retrieves all tracks from it.
// Returns the tracks, the name of the resource (track/album/playlist/artist name), and any error.
func (c *APIClient) GetTracksFromURL(ctx context.Context, spotifyURL string) ([]Track, string, error) {
	resourceType, resourceID, err := ParseSpotifyURL(spotifyURL)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	switch resourceType {
	case "track":
		track, err := c.GetTrack(ctx, resourceID)
		if err != nil {
			return nil, "", err
		}
		return []Track{*track}, track.Name, nil

	case "album":
		result, err := c.GetAlbumTracks(ctx, resourceID)
		if err != nil {
			return nil, "", err
		}
		return result.Tracks, result.Name, nil

	case "playlist":
		result, err := c.GetPlaylistTracks(ctx, resourceID)
		if err != nil {
			return nil, "", err
		}
		return result.Tracks, result.Name, nil

	case "artist":
		result, err := c.GetArtistDiscography(ctx, resourceID, false)
		if err != nil {
			return nil, "", err
		}
		return result.Tracks, result.Name, nil

	default:
		return nil, "", fmt.Errorf("%w: unsupported resource type: %s", ErrInvalidURL, resourceType)
	}
}

// fetchTrackISRC fetches the ISRC for a track, using cache to avoid duplicate requests.
func (c *APIClient) fetchTrackISRC(ctx context.Context, trackID string) string {
	if trackID == "" {
		return ""
	}

	// Check cache first
	c.isrcCacheMu.RLock()
	if isrc, ok := c.isrcCache[trackID]; ok {
		c.isrcCacheMu.RUnlock()
		return isrc
	}
	c.isrcCacheMu.RUnlock()

	// Fetch track to get ISRC
	endpoint := fmt.Sprintf("%s/tracks/%s", spotifyAPIBaseURL, trackID)

	var trackResp struct {
		ExternalIDs spotifyExternalIDs `json:"external_ids"`
	}

	if err := c.getJSON(ctx, endpoint, &trackResp); err != nil {
		log.Printf("Warning: failed to fetch ISRC for track %s: %v", trackID, err)
		return ""
	}

	// Cache the result
	c.isrcCacheMu.Lock()
	c.isrcCache[trackID] = trackResp.ExternalIDs.ISRC
	c.isrcCacheMu.Unlock()

	return trackResp.ExternalIDs.ISRC
}

// convertFullTrack converts a Spotify API track response to our Track type.
func (c *APIClient) convertFullTrack(st *spotifyTrackFull) Track {
	if st == nil {
		return Track{}
	}

	artistNames := extractArtistNames(st.Artists)
	primaryArtist := ""
	if len(artistNames) > 0 {
		primaryArtist = artistNames[0]
	}

	albumArtists := extractArtistNames(st.Album.Artists)
	albumArtist := ""
	if len(albumArtists) > 0 {
		albumArtist = albumArtists[0] // First artist only for directory naming
	}
	coverArtURL := getFirstImageURL(st.Album.Images)
	releaseYear := extractYear(st.Album.ReleaseDate)

	return Track{
		ID:           st.ID,
		Name:         st.Name,
		Artist:       primaryArtist,
		Artists:      artistNames,
		Album:        st.Album.Name,
		AlbumID:      st.Album.ID,
		AlbumArtist:  albumArtist,
		AlbumArtists: albumArtists,
		TrackNumber:  st.TrackNumber,
		DiscNumber:   st.DiscNumber,
		DurationMs:   st.DurationMS,
		ISRC:         st.ExternalIDs.ISRC,
		ReleaseYear:  releaseYear,
		ReleaseDate:  st.Album.ReleaseDate,
		TotalTracks:  st.Album.TotalTracks,
		CoverArtURL:  coverArtURL,
		Explicit:     st.Explicit,
	}
}

// Helper functions

// extractArtistNames extracts artist names from a slice of spotifyArtist.
func extractArtistNames(artists []spotifyArtist) []string {
	names := make([]string, 0, len(artists))
	for _, a := range artists {
		if a.Name != "" {
			names = append(names, a.Name)
		}
	}
	return names
}

// joinArtistNames joins artist names with ", " separator.
func joinArtistNames(artists []spotifyArtist) string {
	names := extractArtistNames(artists)
	return strings.Join(names, ", ")
}

// getFirstImageURL returns the URL of the first (usually largest) image.
func getFirstImageURL(images []spotifyImage) string {
	if len(images) == 0 {
		return ""
	}
	return images[0].URL
}

// extractYear extracts the year from a Spotify release date.
// Spotify release dates can be "YYYY", "YYYY-MM", or "YYYY-MM-DD".
func extractYear(releaseDate string) int {
	if releaseDate == "" {
		return 0
	}

	// Take first 4 characters as year
	if len(releaseDate) >= 4 {
		year, err := strconv.Atoi(releaseDate[:4])
		if err == nil {
			return year
		}
	}
	return 0
}

// parseRetryAfter parses the Retry-After header value and returns a duration.
func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return defaultRetryAfter
	}

	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return defaultRetryAfter
	}

	// Add 1 second buffer to be safe
	return time.Duration(seconds+1) * time.Second
}

// stripLocaleParam removes locale parameter from Spotify pagination URLs.
func stripLocaleParam(raw string) string {
	if raw == "" {
		return ""
	}
	// Remove &locale= or ?locale= parameters
	if idx := strings.Index(raw, "&locale="); idx != -1 {
		endIdx := strings.Index(raw[idx+1:], "&")
		if endIdx != -1 {
			return raw[:idx] + raw[idx+1+endIdx:]
		}
		return raw[:idx]
	}
	if idx := strings.Index(raw, "?locale="); idx != -1 {
		endIdx := strings.Index(raw[idx+1:], "&")
		if endIdx != -1 {
			return raw[:idx] + "?" + raw[idx+1+endIdx+1:]
		}
		return raw[:idx]
	}
	return raw
}

// sleepWithContext sleeps for the specified duration or until context is cancelled.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
