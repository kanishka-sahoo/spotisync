package cover

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"spotisync/internal/utils"
)

// CoverArtConfig holds configuration for the cover art fetcher
type CoverArtConfig struct {
	Timeout      time.Duration
	MaxSize      int
	AllowedTypes []string
	UserAgent    string
}

// CoverResult represents the result of a cover art fetch operation
type CoverResult struct {
	Data   []byte
	Format string
	Source string
	Error  error
}

// Fetcher handles fetching cover art from various sources
type Fetcher struct {
	config CoverArtConfig
	client http.Client
}

// NewFetcher creates a new cover art fetcher with the given configuration
func NewFetcher(config CoverArtConfig) *Fetcher {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.MaxSize == 0 {
		config.MaxSize = 5 * 1024 * 1024 // 5MB default max
	}
	if config.UserAgent == "" {
		config.UserAgent = "SpotiSync/1.0"
	}
	if config.AllowedTypes == nil {
		config.AllowedTypes = []string{"image/jpeg", "image/png", "image/webp"}
	}

	return &Fetcher{
		config: config,
		client: http.Client{
			Timeout: config.Timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Reject redirects to private IP ranges
				host := req.URL.Hostname()
				if isPrivateIP(host) {
					return fmt.Errorf("redirect to private IP not allowed: %s", host)
				}
				return nil
			},
		},
	}
}

// FetchFromMusicBrainz fetches cover art from the MusicBrainz cover art archive
func (f *Fetcher) FetchFromMusicBrainz(artistName, releaseTitle, releaseID string) *CoverResult {
	// Try to fetch using release ID if available
	if releaseID != "" {
		url := fmt.Sprintf("https://coverartarchive.org/release/%s/front-500.jpg", releaseID)
		return f.fetchImage(url)
	}

	// Fallback: search MusicBrainz for the release
	searchURL := fmt.Sprintf(
		"https://musicbrainz.org/ws/2/release-group/?query=artist:%s AND release:%s&fmt=json",
		strings.ReplaceAll(artistName, " ", "+"),
		strings.ReplaceAll(releaseTitle, " ", "+"),
	)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return &CoverResult{Error: err}
	}
	req.Header.Set("User-Agent", f.config.UserAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return &CoverResult{Error: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &CoverResult{Error: fmt.Errorf("musicbrainz search failed: %d", resp.StatusCode)}
	}

	var result struct {
		Releases []struct {
			ID string `json:"id"`
		} `json:"releases"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return &CoverResult{Error: err}
	}

	if len(result.Releases) == 0 {
		return &CoverResult{Error: fmt.Errorf("no release found")}
	}

	// Fetch cover art for the first release
	coverURL := fmt.Sprintf("https://coverartarchive.org/release/%s/front-500.jpg", result.Releases[0].ID)
	return f.fetchImage(coverURL)
}

// FetchFromSpotify fetches cover art from Spotify
func (f *Fetcher) FetchFromSpotify(albumID, accessToken string) *CoverResult {
	url := fmt.Sprintf("https://api.spotify.com/v1/albums/%s/images", albumID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &CoverResult{Error: err}
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", f.config.UserAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return &CoverResult{Error: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &CoverResult{Error: fmt.Errorf("spotify API failed: %d", resp.StatusCode)}
	}

	var images []struct {
		URL    string `json:"url"`
		Height int    `json:"height"`
		Width  int    `json:"width"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&images); err != nil {
		return &CoverResult{Error: err}
	}

	if len(images) == 0 {
		return &CoverResult{Error: fmt.Errorf("no images found")}
	}

	// Try to get the largest image
	largestImage := images[0]
	for _, img := range images[1:] {
		if img.Width > largestImage.Width {
			largestImage = img
		}
	}

	return f.fetchImage(largestImage.URL)
}

// FetchFromLastFM fetches cover art from Last.fm
func (f *Fetcher) FetchFromLastFM(artistName, albumName, apiKey string) *CoverResult {
	url := fmt.Sprintf(
		"http://ws.audioscrobbler.com/2.0/?method=album.getinfo&artist=%s&album=%s&api_key=%s&format=json",
		strings.ReplaceAll(artistName, " ", "+"),
		strings.ReplaceAll(albumName, " ", "+"),
		apiKey,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &CoverResult{Error: err}
	}
	req.Header.Set("User-Agent", f.config.UserAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return &CoverResult{Error: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &CoverResult{Error: fmt.Errorf("lastfm API failed: %d", resp.StatusCode)}
	}

	var result struct {
		Album struct {
			Image []struct {
				Text string `json:"#text"`
				Size string `json:"size"`
			} `json:"image"`
		} `json:"album"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return &CoverResult{Error: err}
	}

	// Try to get extralarge image first, then large, then medium
	sizes := []string{"extralarge", "large", "medium", ""}
	for _, size := range sizes {
		for _, img := range result.Album.Image {
			if img.Size == size && img.Text != "" {
				return f.fetchImage(img.Text)
			}
		}
	}

	return &CoverResult{Error: fmt.Errorf("no image found")}
}

// fetchImage is a helper to fetch an image from a URL
func (f *Fetcher) fetchImage(url string) *CoverResult {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &CoverResult{Error: err}
	}
	req.Header.Set("User-Agent", f.config.UserAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return &CoverResult{Error: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == 404 {
			return &CoverResult{Error: fmt.Errorf("image not found")}
		}
		return &CoverResult{Error: fmt.Errorf("fetch failed: %d", resp.StatusCode)}
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		// Try to detect from first bytes
		contentType = detectContentType(resp.Body)
	}

	allowed := false
	for _, t := range f.config.AllowedTypes {
		if strings.HasPrefix(contentType, t) {
			allowed = true
			break
		}
	}

	if !allowed {
		return &CoverResult{Error: fmt.Errorf("unsupported image type: %s", contentType)}
	}

	// Limit response size
	data, err := io.ReadAll(io.LimitReader(resp.Body, int64(f.config.MaxSize)))
	if err != nil {
		return &CoverResult{Error: err}
	}

	return &CoverResult{
		Data:   data,
		Format: contentType,
		Source: url,
	}
}

// detectContentType attempts to detect content type from the first bytes
func detectContentType(r io.Reader) string {
	data := make([]byte, 4)
	_, _ = r.Read(data)

	if len(data) >= 2 {
		if data[0] == 0xFF && data[1] == 0xD8 {
			return "image/jpeg"
		}
		if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
			return "image/png"
		}
		if data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
			return "image/webp"
		}
	}

	return "application/octet-stream"
}

// isSymlinkSafe checks if the given path is safe from symlink attacks
// Returns the resolved path if safe, or an error if not
func isSymlinkSafe(path, outputDir string) (string, error) {
	// Use Lstat to check if the path is a symlink without following it
	fi, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Path doesn't exist yet, which is fine
			return "", nil
		}
		return "", fmt.Errorf("failed to check file status: %w", err)
	}

	// If it's a symlink, we need to evaluate it
	if fi.Mode()&os.ModeSymlink != 0 {
		// Evaluate the symlink to get the canonical path
		resolvedPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", fmt.Errorf("failed to resolve symlink: %w", err)
		}

		// Get absolute paths for comparison
		absResolved, err := filepath.Abs(resolvedPath)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path: %w", err)
		}

		absOutputDir, err := filepath.Abs(outputDir)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute output dir: %w", err)
		}

		// Check if the resolved path is within the output directory
		if !strings.HasPrefix(absResolved, absOutputDir+string(filepath.Separator)) {
			return "", fmt.Errorf("symlink points outside of allowed directory")
		}

		return absResolved, nil
	}

	// Not a symlink, check if the parent directory is within outputDir
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute output dir: %w", err)
	}

	if !strings.HasPrefix(absPath, absOutputDir+string(filepath.Separator)) {
		return "", fmt.Errorf("path is outside of allowed directory")
	}

	return absPath, nil
}

// isPrivateIP checks if an IP address is in a private range
func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// Check for IPv4 private ranges
	if ip.To4() != nil {
		// 10.0.0.0/8
		if ip[0] == 10 {
			return true
		}
		// 172.16.0.0/12
		if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
			return true
		}
		// 192.168.0.0/16
		if ip[0] == 192 && ip[1] == 168 {
			return true
		}
		// 127.0.0.0/8 (loopback)
		if ip[0] == 127 {
			return true
		}
	}

	// Check for IPv6 private ranges
	// ::1/128 (loopback)
	if ip.IsLoopback() {
		return true
	}

	// fc00::/7 (unique local addresses)
	if ip[0] == 0xfc || ip[0] == 0xfd {
		return true
	}

	return false
}

// checkRedirect validates that redirects don't go to private IP ranges
func checkRedirect(req *http.Request, via []*http.Request) error {
	// Check the new location (req.URL.Host)
	host := req.URL.Hostname()
	if isPrivateIP(host) {
		return fmt.Errorf("redirect to private IP not allowed: %s", host)
	}
	return nil
}

// DownloadCover downloads cover art to the output directory
func (f *Fetcher) DownloadCover(coverURL, outputDir, filename string) (string, error) {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}

	// Fetch the cover image
	result := f.fetchImage(coverURL)
	if result.Error != nil {
		return "", result.Error
	}

	// Determine file extension based on format
	ext := ".jpg"
	switch result.Format {
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	}

	// Build full filepath
	fullPath := filepath.Join(outputDir, filename+ext)

	// Check for symlink attacks before writing
	if _, err := isSymlinkSafe(fullPath, outputDir); err != nil {
		return "", fmt.Errorf("symlink attack detected: %w", err)
	}

	// Write the cover art to file
	if err := os.WriteFile(fullPath, result.Data, 0644); err != nil {
		return "", err
	}

	return fullPath, nil
}

// BuildCoverFilename builds the filename for cover art
func BuildCoverFilename(artist, album string) (string, error) {
	// Reject inputs containing ".." sequences
	if strings.Contains(artist, "..") || strings.Contains(album, "..") {
		return "", fmt.Errorf("invalid input: path traversal sequence detected")
	}

	// Use the SanitizePath function to sanitize both artist and album
	safeArtist := utils.SanitizePath(artist)
	safeAlbum := utils.SanitizePath(album)

	// Normalize the path to remove any redundant separators
	filename := fmt.Sprintf("%s - %s (Cover)", safeArtist, safeAlbum)
	return filepath.Clean(filename), nil
}

// CheckAlbumCoverExists checks if album cover already exists
func (f *Fetcher) CheckAlbumCoverExists(outputDir, artist, album string) (string, bool) {
	filename, err := BuildCoverFilename(artist, album)
	if err != nil {
		return "", false
	}

	// Check for different extensions
	extensions := []string{".jpg", ".png", ".webp"}
	for _, ext := range extensions {
		fullPath := filepath.Join(outputDir, filename+ext)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, true
		}
	}

	return "", false
}

// CoverDeduplicationResult represents the result of cover deduplication
type CoverDeduplicationResult struct {
	Path      string
	WasCopied bool
	Error     error
}

// DedupCoverArt performs deduplication of cover art files
func DedupCoverArt(sourcePath, outputDir, artist, album string) *CoverDeduplicationResult {
	// Check if cover already exists in output
	fetcher := NewFetcher(CoverArtConfig{})
	existingPath, exists := fetcher.CheckAlbumCoverExists(outputDir, artist, album)
	if exists {
		return &CoverDeduplicationResult{
			Path:      existingPath,
			WasCopied: false,
		}
	}

	// Copy cover to output
	filename, err := BuildCoverFilename(artist, album)
	if err != nil {
		return &CoverDeduplicationResult{Error: err}
	}
	destPath := filepath.Join(outputDir, filename+filepath.Ext(sourcePath))

	// Check for symlink attacks before writing
	if _, err := isSymlinkSafe(destPath, outputDir); err != nil {
		return &CoverDeduplicationResult{Error: fmt.Errorf("symlink attack detected: %w", err)}
	}

	// Read source file
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return &CoverDeduplicationResult{Error: err}
	}

	// Write to destination
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return &CoverDeduplicationResult{Error: err}
	}

	return &CoverDeduplicationResult{
		Path:      destPath,
		WasCopied: true,
	}
}
