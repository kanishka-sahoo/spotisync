package qobuz

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DownloadConfig holds download configuration
type DownloadConfig struct {
	OutputDir          string
	Quality            int
	FilenameFormat     string
	IncludeTrackNumber bool
}

// DownloadResult contains download result information
type DownloadResult struct {
	Path       string
	SourceType string
	SourceID   string
	FileSize   int64
	Error      error
}

// ThirdPartyStreamResponse represents the response from third-party Qobuz APIs
type ThirdPartyStreamResponse struct {
	URL string `json:"url"`
}

// GetThirdPartyAPIs returns the list of third-party Qobuz API URLs (base64 decoded)
func GetThirdPartyAPIs() []string {
	// Base64 encoded API URLs for obfuscation
	// Primary: https://dab.yeet.su/api/stream
	// Fallback: https://dabmusic.xyz/api/stream
	encodedAPIs := []string{
		"aHR0cHM6Ly9kYWIueWVldC5zdS9hcGkvc3RyZWFt",     // https://dab.yeet.su/api/stream
		"aHR0cHM6Ly9kYWJtdXNpYy54eXovYXBpL3N0cmVhbQ==", // https://dabmusic.xyz/api/stream
	}

	var apis []string
	for _, encoded := range encodedAPIs {
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			continue
		}
		apis = append(apis, string(decoded))
	}

	return apis
}

// GetDownloadURLFromThirdParty fetches download URL from third-party Qobuz APIs.
// Tries the primary API first, then falls back to secondary.
// Quality codes: 5 (MP3), 6 (FLAC16), 7 (FLAC24), 27 (Hi-Res)
func (c *Client) GetDownloadURLFromThirdParty(ctx context.Context, trackID int64, quality int) (string, error) {
	apis := GetThirdPartyAPIs()
	if len(apis) == 0 {
		return "", fmt.Errorf("no third-party APIs available")
	}

	// Create a client with appropriate timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	var lastError error

	for i, apiURL := range apis {
		// Build request URL: {apiURL}?trackId={trackID}&quality={quality}
		requestURL := fmt.Sprintf("%s?trackId=%d&quality=%d", apiURL, trackID, quality)

		if i == 0 {
			log.Printf("Requesting Qobuz download URL from primary API...")
		} else {
			log.Printf("Primary API failed, trying fallback API...")
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			lastError = err
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			lastError = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastError = fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastError = err
			continue
		}

		if len(body) == 0 {
			lastError = fmt.Errorf("API returned empty response")
			continue
		}

		var streamResp ThirdPartyStreamResponse
		if err := json.Unmarshal(body, &streamResp); err != nil {
			lastError = fmt.Errorf("failed to decode response: %w", err)
			continue
		}

		if streamResp.URL == "" {
			lastError = fmt.Errorf("no download URL in response")
			continue
		}

		log.Printf("Got Qobuz download URL from API")
		return streamResp.URL, nil
	}

	return "", fmt.Errorf("all Qobuz third-party APIs failed: %v", lastError)
}

// DownloadTrackFromThirdParty downloads a track using third-party APIs
func (c *Client) DownloadTrackFromThirdParty(ctx context.Context, trackID int64, cfg DownloadConfig) *DownloadResult {
	// Get track info first
	track, err := c.GetTrack(ctx, trackID)
	if err != nil {
		return &DownloadResult{Error: err}
	}

	// Get download URL from third-party APIs
	downloadURL, err := c.GetDownloadURLFromThirdParty(ctx, trackID, cfg.Quality)
	if err != nil {
		return &DownloadResult{Error: err}
	}

	// Build filename
	filename := buildFilename(track, cfg.FilenameFormat, cfg.IncludeTrackNumber)
	filePath := filepath.Join(cfg.OutputDir, filename)

	// Create output directory
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return &DownloadResult{Error: err}
	}

	// Download file with extended timeout for large Hi-Res files
	if err := c.downloadFileWithTimeout(ctx, downloadURL, filePath, 5*time.Minute); err != nil {
		return &DownloadResult{Error: err}
	}

	// Get file size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return &DownloadResult{Error: err}
	}

	return &DownloadResult{
		Path:       filePath,
		SourceType: "qobuz",
		SourceID:   fmt.Sprintf("%d", trackID),
		FileSize:   fileInfo.Size(),
	}
}

// DownloadTrack downloads a track to the specified output directory
func (c *Client) DownloadTrack(ctx context.Context, trackID int64, cfg DownloadConfig) *DownloadResult {
	// Get track info
	track, err := c.GetTrack(ctx, trackID)
	if err != nil {
		return &DownloadResult{Error: err}
	}

	// Get stream URL with specified quality
	streamURL, _, err := c.GetBestStreamURL(ctx, trackID)
	if err != nil {
		return &DownloadResult{Error: err}
	}

	// Build filename
	filename := buildFilename(track, cfg.FilenameFormat, cfg.IncludeTrackNumber)
	filePath := filepath.Join(cfg.OutputDir, filename)

	// Create output directory
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return &DownloadResult{Error: err}
	}

	// Download file
	if err := c.downloadFile(ctx, streamURL, filePath); err != nil {
		return &DownloadResult{Error: err}
	}

	// Get file size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return &DownloadResult{Error: err}
	}

	return &DownloadResult{
		Path:       filePath,
		SourceType: "qobuz",
		SourceID:   fmt.Sprintf("%d", trackID),
		FileSize:   fileInfo.Size(),
	}
}

func (c *Client) downloadFile(ctx context.Context, url, filePath string) error {
	return c.downloadFileWithTimeout(ctx, url, filePath, 2*time.Minute)
}

func (c *Client) downloadFileWithTimeout(ctx context.Context, url, filePath string, timeout time.Duration) error {
	// Create a client with the specified timeout for large files
	client := &http.Client{
		Timeout: timeout,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	log.Printf("Downloaded: %.2f MB", float64(written)/(1024*1024))
	return nil
}

func buildFilename(track *Track, format string, includeTrackNumber bool) string {
	// Sanitize artist and title for filename
	artist := sanitizeForFilename(track.Performer.Name)
	if artist == "" {
		artist = sanitizeForFilename(track.Album.Artist.Name)
	}
	title := sanitizeForFilename(track.Title)

	var filename string

	if includeTrackNumber {
		filename = fmt.Sprintf("%02d - %s - %s", track.TrackNumber, artist, title)
	} else {
		filename = fmt.Sprintf("%s - %s", artist, title)
	}

	return filename + ".flac"
}

func sanitizeForFilename(name string) string {
	if name == "" {
		return "Unknown"
	}

	// Remove or replace characters not allowed in filenames
	chars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range chars {
		name = strings.ReplaceAll(name, char, "-")
	}
	// Replace multiple spaces with single space
	for strings.Contains(name, "  ") {
		name = strings.ReplaceAll(name, "  ", " ")
	}
	return strings.TrimSpace(name)
}

// DownloadFromISRC downloads a track by ISRC
func (c *Client) DownloadFromISRC(ctx context.Context, isrc string, cfg DownloadConfig) *DownloadResult {
	// Search for track by ISRC
	track, err := c.SearchByISRC(ctx, isrc)
	if err != nil {
		return &DownloadResult{Error: err}
	}

	return c.DownloadTrack(ctx, track.ID, cfg)
}

// DownloadFromISRCWithThirdParty downloads a track by ISRC using third-party APIs
func (c *Client) DownloadFromISRCWithThirdParty(ctx context.Context, isrc string, cfg DownloadConfig) *DownloadResult {
	// Search for track by ISRC
	track, err := c.SearchByISRC(ctx, isrc)
	if err != nil {
		return &DownloadResult{Error: err}
	}

	return c.DownloadTrackFromThirdParty(ctx, track.ID, cfg)
}

// DownloadWithFallback attempts download with quality fallback
func (c *Client) DownloadWithFallback(ctx context.Context, trackID int64, cfg DownloadConfig) *DownloadResult {
	// Try highest quality first (Hi-Res)
	cfg.Quality = QualityHiRes
	result := c.DownloadTrack(ctx, trackID, cfg)

	if result.Error != nil {
		// Try FLAC 24-bit as fallback
		cfg.Quality = QualityFLAC24
		result = c.DownloadTrack(ctx, trackID, cfg)
	}

	if result.Error != nil {
		// Try FLAC 16-bit as last resort
		cfg.Quality = QualityFLAC16
		result = c.DownloadTrack(ctx, trackID, cfg)
	}

	return result
}

// DownloadWithThirdPartyFallback attempts download using third-party APIs with quality fallback
func (c *Client) DownloadWithThirdPartyFallback(ctx context.Context, trackID int64, cfg DownloadConfig) *DownloadResult {
	// Try highest quality first (Hi-Res - 27)
	cfg.Quality = QualityHiRes
	result := c.DownloadTrackFromThirdParty(ctx, trackID, cfg)

	if result.Error != nil {
		// Try FLAC 24-bit as fallback (7)
		cfg.Quality = QualityFLAC24
		result = c.DownloadTrackFromThirdParty(ctx, trackID, cfg)
	}

	if result.Error != nil {
		// Try FLAC 16-bit as last resort (6)
		cfg.Quality = QualityFLAC16
		result = c.DownloadTrackFromThirdParty(ctx, trackID, cfg)
	}

	return result
}

// GetAlbumArtURL returns the album art URL
func (c *Client) GetAlbumArtURL(imageURL string) string {
	return imageURL
}
