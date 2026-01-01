package navidrome

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// SubsonicAPIVersion is the Subsonic API version to use
	SubsonicAPIVersion = "1.16.1"
	// SubsonicClientName is the client identifier sent to Navidrome
	SubsonicClientName = "spotisync"
)

// Client provides access to Navidrome's Subsonic API
type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

// NewClient creates a new Navidrome client
func NewClient(baseURL, username, password string) *Client {
	// Ensure baseURL doesn't have trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &Client{
		baseURL:  baseURL,
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SubsonicResponse is the wrapper for all Subsonic API responses
type SubsonicResponse struct {
	SubsonicResponse struct {
		Status        string          `json:"status"`
		Version       string          `json:"version"`
		Type          string          `json:"type"`
		ServerVersion string          `json:"serverVersion"`
		Error         *SubsonicError  `json:"error,omitempty"`
		ScanStatus    *ScanStatus     `json:"scanStatus,omitempty"`
		SearchResult3 *SearchResult3  `json:"searchResult3,omitempty"`
		Playlists     *PlaylistsData  `json:"playlists,omitempty"`
		Playlist      *PlaylistDetail `json:"playlist,omitempty"`
	} `json:"subsonic-response"`
}

// SubsonicError represents an error from the Subsonic API
type SubsonicError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *SubsonicError) Error() string {
	return fmt.Sprintf("subsonic error %d: %s", e.Code, e.Message)
}

// ScanStatus represents the library scan status
type ScanStatus struct {
	Scanning bool  `json:"scanning"`
	Count    int64 `json:"count"`
}

// FlexibleISRC handles ISRC that can be either a string or array of strings.
// Navidrome may return ISRC as an array when a track has multiple ISRCs.
type FlexibleISRC string

// UnmarshalJSON implements json.Unmarshaler for FlexibleISRC
func (f *FlexibleISRC) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as a string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*f = FlexibleISRC(str)
		return nil
	}

	// Try to unmarshal as an array of strings
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		if len(arr) > 0 {
			*f = FlexibleISRC(arr[0]) // Use first ISRC
		}
		return nil
	}

	// If neither works, just set to empty
	*f = ""
	return nil
}

// Track represents a song/track from Navidrome
type Track struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Album       string       `json:"album"`
	Artist      string       `json:"artist"`
	AlbumArtist string       `json:"albumArtist,omitempty"`
	TrackNumber int          `json:"track,omitempty"`
	DiscNumber  int          `json:"discNumber,omitempty"`
	Year        int          `json:"year,omitempty"`
	Duration    int          `json:"duration,omitempty"`
	BitRate     int          `json:"bitRate,omitempty"`
	Size        int64        `json:"size,omitempty"`
	Path        string       `json:"path,omitempty"`
	ISRC        FlexibleISRC `json:"isrc,omitempty"`
	CoverArtID  string       `json:"coverArt,omitempty"`
}

// SearchResult3 represents search results from the search3 endpoint
type SearchResult3 struct {
	Artist []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"artist,omitempty"`
	Album []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Artist string `json:"artist"`
	} `json:"album,omitempty"`
	Song []Track `json:"song,omitempty"`
}

// Playlist represents a playlist from Navidrome
type Playlist struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	SongCount int       `json:"songCount"`
	Duration  int       `json:"duration"`
	Created   time.Time `json:"created"`
	Changed   time.Time `json:"changed"`
	Owner     string    `json:"owner,omitempty"`
	Public    bool      `json:"public"`
}

// PlaylistsData wraps the playlists array in the response
type PlaylistsData struct {
	Playlist []Playlist `json:"playlist"`
}

// PlaylistDetail represents a playlist with its tracks
type PlaylistDetail struct {
	Playlist
	Entry []Track `json:"entry,omitempty"`
}

// generateSalt creates a random salt for authentication
func generateSalt() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// generateToken creates the MD5 token for authentication
// token = md5(password + salt)
func generateToken(password, salt string) string {
	hash := md5.Sum([]byte(password + salt))
	return hex.EncodeToString(hash[:])
}

// buildAuthParams returns the query parameters for Subsonic authentication
func (c *Client) buildAuthParams() (url.Values, error) {
	salt, err := generateSalt()
	if err != nil {
		return nil, err
	}

	token := generateToken(c.password, salt)

	params := url.Values{}
	params.Set("u", c.username)
	params.Set("t", token)
	params.Set("s", salt)
	params.Set("v", SubsonicAPIVersion)
	params.Set("c", SubsonicClientName)
	params.Set("f", "json")

	return params, nil
}

// doRequest performs an authenticated request to the Subsonic API
func (c *Client) doRequest(ctx context.Context, endpoint string, additionalParams url.Values) (*SubsonicResponse, error) {
	authParams, err := c.buildAuthParams()
	if err != nil {
		return nil, fmt.Errorf("failed to build auth params: %w", err)
	}

	// Merge additional params
	for key, values := range additionalParams {
		for _, value := range values {
			authParams.Add(key, value)
		}
	}

	requestURL := fmt.Sprintf("%s/rest/%s?%s", c.baseURL, endpoint, authParams.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var subsonicResp SubsonicResponse
	if err := json.Unmarshal(body, &subsonicResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for Subsonic API error
	if subsonicResp.SubsonicResponse.Status != "ok" {
		if subsonicResp.SubsonicResponse.Error != nil {
			return nil, subsonicResp.SubsonicResponse.Error
		}
		return nil, fmt.Errorf("subsonic API returned status: %s", subsonicResp.SubsonicResponse.Status)
	}

	return &subsonicResp, nil
}

// Ping tests connectivity to the Navidrome server
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.doRequest(ctx, "ping.view", nil)
	if err != nil {
		log.Printf("[navidrome] ping failed: %v", err)
		return fmt.Errorf("failed to ping Navidrome: %w", err)
	}
	return nil
}

// StartScan triggers a library scan
func (c *Client) StartScan(ctx context.Context) error {
	_, err := c.doRequest(ctx, "startScan.view", nil)
	if err != nil {
		log.Printf("[navidrome] failed to start scan: %v", err)
		return fmt.Errorf("failed to start scan: %w", err)
	}
	log.Printf("[navidrome] library scan started")
	return nil
}

// GetScanStatus gets the current scan status
func (c *Client) GetScanStatus(ctx context.Context) (*ScanStatus, error) {
	resp, err := c.doRequest(ctx, "getScanStatus.view", nil)
	if err != nil {
		log.Printf("[navidrome] failed to get scan status: %v", err)
		return nil, fmt.Errorf("failed to get scan status: %w", err)
	}

	if resp.SubsonicResponse.ScanStatus == nil {
		return &ScanStatus{Scanning: false, Count: 0}, nil
	}

	return resp.SubsonicResponse.ScanStatus, nil
}

// CreatePlaylist creates a new playlist and returns its ID
func (c *Client) CreatePlaylist(ctx context.Context, name string) (string, error) {
	params := url.Values{}
	params.Set("name", name)

	resp, err := c.doRequest(ctx, "createPlaylist.view", params)
	if err != nil {
		log.Printf("[navidrome] failed to create playlist '%s': %v", name, err)
		return "", fmt.Errorf("failed to create playlist: %w", err)
	}

	// The createPlaylist endpoint returns the playlist details
	if resp.SubsonicResponse.Playlist != nil {
		log.Printf("[navidrome] created playlist '%s' with ID: %s", name, resp.SubsonicResponse.Playlist.ID)
		return resp.SubsonicResponse.Playlist.ID, nil
	}

	return "", fmt.Errorf("playlist created but no ID returned")
}

// UpdatePlaylistTracks updates a playlist with the given song IDs
// This replaces all tracks in the playlist with the provided song IDs
func (c *Client) UpdatePlaylistTracks(ctx context.Context, playlistID string, songIDs []string) error {
	if len(songIDs) == 0 {
		log.Printf("[navidrome] no songs to add to playlist %s", playlistID)
		return nil
	}

	params := url.Values{}
	params.Set("playlistId", playlistID)

	// Add each song ID as a separate parameter
	for _, songID := range songIDs {
		params.Add("songId", songID)
	}

	_, err := c.doRequest(ctx, "createPlaylist.view", params)
	if err != nil {
		log.Printf("[navidrome] failed to update playlist %s with %d songs: %v", playlistID, len(songIDs), err)
		return fmt.Errorf("failed to update playlist tracks: %w", err)
	}

	log.Printf("[navidrome] updated playlist %s with %d songs", playlistID, len(songIDs))
	return nil
}

// AddToPlaylist adds songs to an existing playlist without removing existing tracks
func (c *Client) AddToPlaylist(ctx context.Context, playlistID string, songIDs []string) error {
	if len(songIDs) == 0 {
		return nil
	}

	params := url.Values{}
	params.Set("playlistId", playlistID)

	for _, songID := range songIDs {
		params.Add("songIdToAdd", songID)
	}

	_, err := c.doRequest(ctx, "updatePlaylist.view", params)
	if err != nil {
		log.Printf("[navidrome] failed to add songs to playlist %s: %v", playlistID, err)
		return fmt.Errorf("failed to add songs to playlist: %w", err)
	}

	log.Printf("[navidrome] added %d songs to playlist %s", len(songIDs), playlistID)
	return nil
}

// SearchTrackByISRC searches for a track by ISRC
// Note: Navidrome may not index ISRC in the search, so this searches the term directly
func (c *Client) SearchTrackByISRC(ctx context.Context, isrc string) (*Track, error) {
	params := url.Values{}
	params.Set("query", isrc)
	params.Set("songCount", "10")
	params.Set("artistCount", "0")
	params.Set("albumCount", "0")

	resp, err := c.doRequest(ctx, "search3.view", params)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if resp.SubsonicResponse.SearchResult3 == nil {
		return nil, fmt.Errorf("no search results for ISRC: %s", isrc)
	}

	songs := resp.SubsonicResponse.SearchResult3.Song
	if len(songs) == 0 {
		return nil, fmt.Errorf("no tracks found for ISRC: %s", isrc)
	}

	// Try to find exact ISRC match if the track has ISRC metadata
	for i := range songs {
		if strings.EqualFold(string(songs[i].ISRC), isrc) {
			return &songs[i], nil
		}
	}

	// Return first result if no exact match
	return &songs[0], nil
}

// SearchTrackByTitle searches for a track by title and artist
func (c *Client) SearchTrackByTitle(ctx context.Context, title, artist string) (*Track, error) {
	query := title
	if artist != "" {
		query = fmt.Sprintf("%s %s", artist, title)
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("songCount", "20")
	params.Set("artistCount", "0")
	params.Set("albumCount", "0")

	resp, err := c.doRequest(ctx, "search3.view", params)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if resp.SubsonicResponse.SearchResult3 == nil {
		return nil, fmt.Errorf("no search results for: %s - %s", artist, title)
	}

	songs := resp.SubsonicResponse.SearchResult3.Song
	if len(songs) == 0 {
		return nil, fmt.Errorf("no tracks found for: %s - %s", artist, title)
	}

	// Try to find best match by comparing title and artist
	titleLower := strings.ToLower(title)
	artistLower := strings.ToLower(artist)

	for i := range songs {
		songTitleLower := strings.ToLower(songs[i].Title)
		songArtistLower := strings.ToLower(songs[i].Artist)

		// Exact title match with artist containing the search artist
		if songTitleLower == titleLower && strings.Contains(songArtistLower, artistLower) {
			return &songs[i], nil
		}
	}

	// Return first result if no better match found
	return &songs[0], nil
}

// SearchTracks performs a general search and returns matching tracks
func (c *Client) SearchTracks(ctx context.Context, query string, limit int) ([]Track, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("songCount", fmt.Sprintf("%d", limit))
	params.Set("artistCount", "0")
	params.Set("albumCount", "0")

	resp, err := c.doRequest(ctx, "search3.view", params)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if resp.SubsonicResponse.SearchResult3 == nil {
		return []Track{}, nil
	}

	return resp.SubsonicResponse.SearchResult3.Song, nil
}

// GetPlaylists gets all playlists for the authenticated user
func (c *Client) GetPlaylists(ctx context.Context) ([]Playlist, error) {
	resp, err := c.doRequest(ctx, "getPlaylists.view", nil)
	if err != nil {
		log.Printf("[navidrome] failed to get playlists: %v", err)
		return nil, fmt.Errorf("failed to get playlists: %w", err)
	}

	if resp.SubsonicResponse.Playlists == nil {
		return []Playlist{}, nil
	}

	return resp.SubsonicResponse.Playlists.Playlist, nil
}

// GetPlaylist gets a specific playlist by ID, including its tracks
func (c *Client) GetPlaylist(ctx context.Context, playlistID string) (*PlaylistDetail, error) {
	params := url.Values{}
	params.Set("id", playlistID)

	resp, err := c.doRequest(ctx, "getPlaylist.view", params)
	if err != nil {
		log.Printf("[navidrome] failed to get playlist %s: %v", playlistID, err)
		return nil, fmt.Errorf("failed to get playlist: %w", err)
	}

	if resp.SubsonicResponse.Playlist == nil {
		return nil, fmt.Errorf("playlist not found: %s", playlistID)
	}

	return resp.SubsonicResponse.Playlist, nil
}

// DeletePlaylist deletes a playlist by ID
func (c *Client) DeletePlaylist(ctx context.Context, playlistID string) error {
	params := url.Values{}
	params.Set("id", playlistID)

	_, err := c.doRequest(ctx, "deletePlaylist.view", params)
	if err != nil {
		log.Printf("[navidrome] failed to delete playlist %s: %v", playlistID, err)
		return fmt.Errorf("failed to delete playlist: %w", err)
	}

	log.Printf("[navidrome] deleted playlist %s", playlistID)
	return nil
}

// FindPlaylistByName finds a playlist by name, returns nil if not found
func (c *Client) FindPlaylistByName(ctx context.Context, name string) (*Playlist, error) {
	playlists, err := c.GetPlaylists(ctx)
	if err != nil {
		return nil, err
	}

	nameLower := strings.ToLower(name)
	for i := range playlists {
		if strings.ToLower(playlists[i].Name) == nameLower {
			return &playlists[i], nil
		}
	}

	return nil, nil
}

// CreateOrUpdatePlaylist creates a new playlist or updates an existing one with the given songs
func (c *Client) CreateOrUpdatePlaylist(ctx context.Context, name string, songIDs []string) (string, error) {
	// Check if playlist already exists
	existing, err := c.FindPlaylistByName(ctx, name)
	if err != nil {
		return "", fmt.Errorf("failed to check for existing playlist: %w", err)
	}

	if existing != nil {
		// Update existing playlist
		if err := c.UpdatePlaylistTracks(ctx, existing.ID, songIDs); err != nil {
			return "", err
		}
		return existing.ID, nil
	}

	// Create new playlist
	playlistID, err := c.CreatePlaylist(ctx, name)
	if err != nil {
		return "", err
	}

	// Add songs to the playlist
	if len(songIDs) > 0 {
		if err := c.UpdatePlaylistTracks(ctx, playlistID, songIDs); err != nil {
			return "", err
		}
	}

	return playlistID, nil
}

// WaitForScanComplete waits for an ongoing scan to complete, with timeout
func (c *Client) WaitForScanComplete(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	pollInterval := 2 * time.Second

	for time.Now().Before(deadline) {
		status, err := c.GetScanStatus(ctx)
		if err != nil {
			return err
		}

		if !status.Scanning {
			log.Printf("[navidrome] scan complete, %d items indexed", status.Count)
			return nil
		}

		log.Printf("[navidrome] scan in progress, %d items indexed so far...", status.Count)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
			continue
		}
	}

	return fmt.Errorf("scan did not complete within %v timeout", timeout)
}
