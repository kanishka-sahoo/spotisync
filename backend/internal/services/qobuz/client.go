package qobuz

import (
	"context"
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
	qobuzAPIURL   = "https://www.qobuz.com/api.json/0.2"
	qobuzAPIURLv1 = "https://www.qobuz.com/api.json/0.2"
)

// Client handles Qobuz API interactions
type Client struct {
	httpClient *http.Client
	appID      string
	secret     string
}

// Track represents Qobuz track metadata
type Track struct {
	ID                  int64   `json:"id"`
	Title               string  `json:"title"`
	Version             string  `json:"version"`
	Duration            int     `json:"duration"`
	TrackNumber         int     `json:"track_number"`
	MediaNumber         int     `json:"media_number"`
	ISRC                string  `json:"isrc"`
	Copyright           string  `json:"copyright"`
	MaximumBitDepth     int     `json:"maximum_bit_depth"`
	MaximumSamplingRate float64 `json:"maximum_sampling_rate"`
	Hires               bool    `json:"hires"`
	HiresStreamable     bool    `json:"hires_streamable"`
	ReleaseDateOriginal string  `json:"release_date_original"`
	Performer           struct {
		Name string `json:"name"`
		ID   int64  `json:"id"`
	} `json:"performer"`
	Album struct {
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
	} `json:"album"`
}

// SearchResponse represents Qobuz search results
type SearchResponse struct {
	Query  string `json:"query"`
	Tracks struct {
		Limit  int     `json:"limit"`
		Offset int     `json:"offset"`
		Total  int     `json:"total"`
		Items  []Track `json:"items"`
	} `json:"tracks"`
}

// StreamResponse represents stream URL response
type StreamResponse struct {
	URL string `json:"url"`
}

// Quality codes
const (
	QualityMP3    = 5
	QualityFLAC16 = 6
	QualityFLAC24 = 7
	QualityHiRes  = 27
)

// NewClient creates a new Qobuz client.
// If appID is empty, uses a hardcoded app_id for the public search API.
// This default app_id is used for searching tracks on the public Qobuz API.
func NewClient(appID, secret string) *Client {
	// Use hardcoded app_id for public search API if not provided
	// This is used for the Qobuz search API
	if appID == "" {
		appID = "798273057"
	}

	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		appID:      appID,
		secret:     secret,
	}
}

// doRequest makes an API request to Qobuz
func (c *Client) doRequest(ctx context.Context, endpoint string) ([]byte, error) {
	// Add authentication parameters
	if strings.Contains(endpoint, "?") {
		endpoint += "&"
	} else {
		endpoint += "?"
	}
	endpoint += fmt.Sprintf("app_id=%s", c.appID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Qobuz API failed: HTTP %d - %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// SearchTracks searches for tracks on Qobuz
func (c *Client) SearchTracks(ctx context.Context, query string, limit int) (*SearchResponse, error) {
	if limit <= 0 || limit > 50 {
		limit = 50
	}

	searchURL := fmt.Sprintf("%s/track/search?query=%s&limit=%d",
		qobuzAPIURL, url.QueryEscape(query), limit)

	data, err := c.doRequest(ctx, searchURL)
	if err != nil {
		return nil, err
	}

	var result SearchResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	return &result, nil
}

// SearchByISRC searches for a track by ISRC code
func (c *Client) SearchByISRC(ctx context.Context, isrc string) (*Track, error) {
	results, err := c.SearchTracks(ctx, isrc, 10)
	if err != nil {
		return nil, err
	}

	for i := range results.Tracks.Items {
		track := &results.Tracks.Items[i]
		if strings.EqualFold(track.ISRC, isrc) {
			return track, nil
		}
	}

	return nil, fmt.Errorf("track not found for ISRC: %s", isrc)
}

// GetTrack retrieves track information by ID
func (c *Client) GetTrack(ctx context.Context, trackID int64) (*Track, error) {
	trackURL := fmt.Sprintf("%s/track/get?track_id=%d", qobuzAPIURL, trackID)

	data, err := c.doRequest(ctx, trackURL)
	if err != nil {
		return nil, err
	}

	var result struct {
		Track Track `json:"track"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to decode track response: %w", err)
	}

	return &result.Track, nil
}

// GetStreamURL gets the stream URL for a track with specified quality
func (c *Client) GetStreamURL(ctx context.Context, trackID int64, quality int) (string, error) {
	streamURL := fmt.Sprintf("%s/track/getFileUrl?track_id=%d&format_id=%d",
		qobuzAPIURL, trackID, quality)

	data, err := c.doRequest(ctx, streamURL)
	if err != nil {
		return "", err
	}

	var result struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("failed to decode stream response: %w", err)
	}

	if result.URL == "" {
		return "", fmt.Errorf("no stream URL available for track %d", trackID)
	}

	return result.URL, nil
}

// GetBestStreamURL gets the best available quality stream URL
func (c *Client) GetBestStreamURL(ctx context.Context, trackID int64) (string, int, error) {
	// Try Hi-Res first (quality 27)
	url, err := c.GetStreamURL(ctx, trackID, QualityHiRes)
	if err == nil && url != "" {
		return url, QualityHiRes, nil
	}

	// Try FLAC 24-bit (quality 7)
	url, err = c.GetStreamURL(ctx, trackID, QualityFLAC24)
	if err == nil && url != "" {
		return url, QualityFLAC24, nil
	}

	// Try FLAC 16-bit (quality 6)
	url, err = c.GetStreamURL(ctx, trackID, QualityFLAC16)
	if err == nil && url != "" {
		return url, QualityFLAC16, nil
	}

	// Fallback to MP3 320 (quality 5)
	url, err = c.GetStreamURL(ctx, trackID, QualityMP3)
	if err != nil {
		return "", 0, fmt.Errorf("no stream URL available for track %d: %w", trackID, err)
	}

	return url, QualityMP3, nil
}

// TrackInfo contains comprehensive track information
type TrackInfo struct {
	Track
	DurationSecs int
	QualityLabel string
	QualityCode  int
	HiResInfo    string
	CoverURL     string
}

// ToTrackInfo converts a Track to TrackInfo with additional details
func (t *Track) ToTrackInfo() *TrackInfo {
	durationSecs := t.Duration

	qualityLabel := "Standard"
	qualityCode := QualityMP3
	hiResInfo := ""

	if t.Hires {
		qualityLabel = "Hi-Res"
		qualityCode = QualityHiRes
		if t.MaximumBitDepth > 0 && t.MaximumSamplingRate > 0 {
			hiResInfo = fmt.Sprintf("%d-bit / %.1f kHz", t.MaximumBitDepth, t.MaximumSamplingRate)
		}
	} else if t.MaximumBitDepth >= 24 {
		qualityLabel = "FLAC 24-bit"
		qualityCode = QualityFLAC24
	} else {
		qualityLabel = "FLAC 16-bit"
		qualityCode = QualityFLAC16
	}

	return &TrackInfo{
		Track:        *t,
		DurationSecs: durationSecs,
		QualityLabel: qualityLabel,
		QualityCode:  qualityCode,
		HiResInfo:    hiResInfo,
		CoverURL:     t.Album.Image.Large,
	}
}

// MatchByMetadata searches for tracks matching the given metadata
func (c *Client) MatchByMetadata(ctx context.Context, trackName, artistName string, expectedISRC string, expectedDurationMs int) (*Track, error) {
	// Build search query
	query := fmt.Sprintf("%s %s", artistName, trackName)

	results, err := c.SearchTracks(ctx, query, 20)
	if err != nil {
		return nil, err
	}

	if len(results.Tracks.Items) == 0 {
		return nil, fmt.Errorf("no tracks found for query: %s", query)
	}

	// Priority 1: Match by ISRC if provided
	if expectedISRC != "" {
		for i := range results.Tracks.Items {
			track := &results.Tracks.Items[i]
			if strings.EqualFold(track.ISRC, expectedISRC) {
				return track, nil
			}
		}
	}

	// Priority 2: Match by duration if provided
	if expectedDurationMs > 0 {
		tolerance := 3 // 3 seconds tolerance
		var durationMatches []Track

		for _, track := range results.Tracks.Items {
			durationDiff := track.Duration - expectedDurationMs
			if durationDiff < 0 {
				durationDiff = -durationDiff
			}
			if durationDiff <= tolerance*1000 {
				durationMatches = append(durationMatches, track)
			}
		}

		if len(durationMatches) > 0 {
			return c.selectBestTrack(durationMatches), nil
		}
	}

	// Priority 3: Return best quality track from results
	return c.selectBestTrack(results.Tracks.Items), nil
}

func (c *Client) selectBestTrack(tracks []Track) *Track {
	if len(tracks) == 0 {
		return nil
	}

	best := &tracks[0]
	for i := range tracks {
		track := &tracks[i]
		// Prefer Hi-Res streamable tracks
		if track.Hires && track.HiresStreamable {
			best = track
			break
		}
		// Prefer higher bit depth
		if track.MaximumBitDepth > best.MaximumBitDepth {
			best = track
		}
	}
	return best
}

// ParseTrackID extracts track ID from Qobuz URL
func ParseTrackID(qobuzURL string) (int64, error) {
	if qobuzURL == "" {
		return 0, errors.New("empty URL")
	}

	// Validate that it's a Qobuz URL
	if !strings.Contains(qobuzURL, "qobuz.com") {
		return 0, fmt.Errorf("not a Qobuz URL: %s", qobuzURL)
	}

	// Format: https://play.qobuz.com/track/1234567890
	parts := strings.Split(qobuzURL, "/track/")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid Qobuz URL format: %s", qobuzURL)
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

// GetQualityLabel returns a human-readable label for quality code
func GetQualityLabel(qualityCode int) string {
	switch qualityCode {
	case QualityHiRes:
		return "Hi-Res"
	case QualityFLAC24:
		return "FLAC 24-bit"
	case QualityFLAC16:
		return "FLAC 16-bit"
	case QualityMP3:
		return "MP3 320"
	default:
		return "Unknown"
	}
}
