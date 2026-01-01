package tidal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	tidalAuthURL = "https://auth.tidal.com/v1/oauth2/token"
	tidalAPIURL  = "https://api.tidal.com/v1"
)

// Client handles Tidal API interactions
type Client struct {
	httpClient   *http.Client
	clientID     string
	clientSecret string
	accessToken  string
	tokenExpiry  time.Time
}

// NewClient creates a new Tidal client.
// If clientID or clientSecret are empty, uses hardcoded credentials for the search API.
// These default credentials are used for searching tracks, not for authenticated downloads.
func NewClient(clientID, clientSecret string) *Client {
	// Use hardcoded credentials for search API if not provided
	// These are used for the Tidal search API (not for downloading)
	// Base64 encoded: "6BDSRdpK9hqEBTgU" and "xeuPmY7nbpZ9IIbLAcQ93shka1VNheUAqN6IcszjTG8="
	if clientID == "" {
		decoded, _ := base64.StdEncoding.DecodeString("NkJEU1JkcEs5aHFFQlRnVQ==")
		clientID = string(decoded)
	}
	if clientSecret == "" {
		decoded, _ := base64.StdEncoding.DecodeString("eGV1UG1ZN25icFo5SUliTEFjUTkzc2hrYTFWTmhlVUFxTjZJY3N6alRHOD0=")
		clientSecret = string(decoded)
	}

	return &Client{
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// Track represents Tidal track metadata
type Track struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	ISRC         string `json:"isrc"`
	AudioQuality string `json:"audioQuality"`
	TrackNumber  int    `json:"trackNumber"`
	VolumeNumber int    `json:"volumeNumber"`
	Duration     int    `json:"duration"`
	Copyright    string `json:"copyright"`
	Explicit     bool   `json:"explicit"`
	Album        struct {
		Title       string `json:"title"`
		Cover       string `json:"cover"`
		ReleaseDate string `json:"releaseDate"`
	} `json:"album"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Artist struct {
		Name string `json:"name"`
	} `json:"artist"`
	MediaMetadata struct {
		Tags []string `json:"tags"`
	} `json:"mediaMetadata"`
}

// SearchResponse represents Tidal search results
type SearchResponse struct {
	Limit              int     `json:"limit"`
	Offset             int     `json:"offset"`
	TotalNumberOfItems int     `json:"totalNumberOfItems"`
	Items              []Track `json:"items"`
}

// DownloadInfo represents track download information
type DownloadInfo struct {
	TrackID      int64  `json:"trackId"`
	ManifestURL  string `json:"manifestUrl"`
	ManifestType string `json:"manifestType"`
	Quality      string `json:"quality"`
	BitDepth     int    `json:"bitDepth"`
	SampleRate   int    `json:"sampleRate"`
}

// Quality preferences
const (
	QualityHiRes         = "HI_RES"
	QualityHiResLossless = "HI_RES_LOSSLESS"
	QualityLossless      = "LOSSLESS"
	QualityHigh          = "HIGH"
	QualityLow           = "LOW"
)

// GetAccessToken obtains an OAuth access token
func (c *Client) GetAccessToken(ctx context.Context) (string, error) {
	// Return cached token if still valid
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry.Add(-time.Minute)) {
		return c.accessToken, nil
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", c.clientID)
	data.Set("client_secret", c.clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tidalAuthURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Tidal auth failed: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	c.accessToken = result.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)

	return c.accessToken, nil
}

func (c *Client) doRequest(ctx context.Context, endpoint string) ([]byte, error) {
	token, err := c.GetAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Tidal API failed: HTTP %d - %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// SearchTracks searches for tracks on Tidal
func (c *Client) SearchTracks(ctx context.Context, query string, limit int) (*SearchResponse, error) {
	if limit <= 0 || limit > 50 {
		limit = 50
	}

	token, err := c.GetAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	searchURL := fmt.Sprintf("%s/search/tracks?query=%s&limit=%d&countryCode=US",
		tidalAPIURL, url.QueryEscape(query), limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Tidal search failed: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// SearchByISRC searches for a track by ISRC code
func (c *Client) SearchByISRC(ctx context.Context, isrc string) (*Track, error) {
	results, err := c.SearchTracks(ctx, isrc, 10)
	if err != nil {
		return nil, err
	}

	for _, track := range results.Items {
		if strings.EqualFold(track.ISRC, isrc) {
			return &track, nil
		}
	}

	return nil, fmt.Errorf("track not found for ISRC: %s", isrc)
}

// GetTrack retrieves track information by ID
func (c *Client) GetTrack(ctx context.Context, trackID int64) (*Track, error) {
	token, err := c.GetAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	trackURL := fmt.Sprintf("%s/tracks/%d?countryCode=US", tidalAPIURL, trackID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, trackURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Tidal track fetch failed: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var track Track
	if err := json.NewDecoder(resp.Body).Decode(&track); err != nil {
		return nil, err
	}

	return &track, nil
}

// FindByISRC finds a track by ISRC, searching across multiple results
func (c *Client) FindByISRC(ctx context.Context, isrc string) (*Track, error) {
	return c.SearchByISRC(ctx, isrc)
}

// GetStreamURL gets the stream URL for a track
func (c *Client) GetStreamURL(ctx context.Context, trackID int64, quality string) (string, error) {
	token, err := c.GetAccessToken(ctx)
	if err != nil {
		return "", err
	}

	streamURL := fmt.Sprintf("%s/tracks/%d/streamUrl?quality=%s&countryCode=US",
		tidalAPIURL, trackID, quality)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Tidal stream URL failed: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.URL, nil
}

// GetAlbumArtURL returns the album art URL for a cover ID
func (c *Client) GetAlbumArtURL(coverID string, size int) string {
	// Cover ID format needs dashes converted to slashes
	coverPath := strings.ReplaceAll(coverID, "-", "/")
	if size <= 0 {
		size = 1280
	}
	return fmt.Sprintf("https://resources.tidal.com/images/%s/%dx%d.jpg", coverPath, size, size)
}

// TrackInfo contains comprehensive track information
type TrackInfo struct {
	Track
	CoverURL     string
	DurationSecs int
	QualityInfo  string
	HiResCapable bool
}

// ToTrackInfo converts a Track to TrackInfo
func (t *Track) ToTrackInfo() *TrackInfo {
	durationSecs := t.Duration / 1000
	hiRes := false
	for _, tag := range t.MediaMetadata.Tags {
		if tag == "HIRES_LOSSLESS" || tag == "HIRES" {
			hiRes = true
			break
		}
	}

	qualityInfo := t.AudioQuality
	if hiRes {
		qualityInfo = "Hi-Res"
	}

	return &TrackInfo{
		Track:        *t,
		CoverURL:     t.Album.Cover,
		DurationSecs: durationSecs,
		QualityInfo:  qualityInfo,
		HiResCapable: hiRes,
	}
}

// Hires checks if track is Hi-Res
func (t *Track) Hires() bool {
	for _, tag := range t.MediaMetadata.Tags {
		if tag == "HIRES_LOSSLESS" || tag == "HIRES" {
			return true
		}
	}
	return false
}

// MatchByMetadata searches for tracks matching the given metadata
func (c *Client) MatchByMetadata(ctx context.Context, trackName, artistName string, expectedISRC string, expectedDurationMs int) (*Track, error) {
	// Build search query
	query := fmt.Sprintf("%s %s", artistName, trackName)

	results, err := c.SearchTracks(ctx, query, 20)
	if err != nil {
		return nil, err
	}

	if len(results.Items) == 0 {
		return nil, fmt.Errorf("no tracks found for query: %s", query)
	}

	// Priority 1: Match by ISRC if provided
	if expectedISRC != "" {
		for i := range results.Items {
			track := &results.Items[i]
			if strings.EqualFold(track.ISRC, expectedISRC) {
				return track, nil
			}
		}
	}

	// Priority 2: Match by duration if provided
	if expectedDurationMs > 0 {
		tolerance := 3 // 3 seconds tolerance
		var durationMatches []Track

		for _, track := range results.Items {
			durationDiff := track.Duration - expectedDurationMs
			if durationDiff < 0 {
				durationDiff = -durationDiff
			}
			if durationDiff <= tolerance*1000 {
				durationMatches = append(durationMatches, track)
			}
		}

		if len(durationMatches) > 0 {
			// Return the best quality match
			return c.selectBestTrack(durationMatches), nil
		}
	}

	// Priority 3: Return best quality track from results
	return c.selectBestTrack(results.Items), nil
}

func (c *Client) selectBestTrack(tracks []Track) *Track {
	if len(tracks) == 0 {
		return nil
	}

	best := &tracks[0]
	for i := range tracks {
		track := &tracks[i]
		// Prefer Hi-Res
		for _, tag := range track.MediaMetadata.Tags {
			if tag == "HIRES_LOSSLESS" {
				best = track
				break
			}
		}
	}
	return best
}

// GetQualityForDownload returns the best available quality
func (c *Client) GetQualityForDownload(allowHiRes bool) string {
	if allowHiRes {
		return QualityHiRes
	}
	return QualityLossless
}

// ParseTrackID extracts track ID from Tidal URL
func ParseTrackID(tidalURL string) (int64, error) {
	if tidalURL == "" {
		return 0, errors.New("empty URL")
	}

	// Validate that it's a Tidal URL
	if !strings.Contains(tidalURL, "tidal.com") && !strings.Contains(tidalURL, "listen.tidal.com") {
		return 0, fmt.Errorf("not a Tidal URL: %s", tidalURL)
	}

	// Format: https://listen.tidal.com/track/441821360
	// or: https://tidal.com/browse/track/123456789
	parts := strings.Split(tidalURL, "/track/")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid Tidal URL format: %s", tidalURL)
	}

	trackIDStr := strings.Split(parts[1], "?")[0]
	trackIDStr = strings.TrimSpace(trackIDStr)

	var trackID int64
	_, err := fmt.Sscanf(trackIDStr, "%d", &trackID)
	if err != nil {
		return 0, fmt.Errorf("failed to parse track ID: %w", err)
	}

	return trackID, nil
}
