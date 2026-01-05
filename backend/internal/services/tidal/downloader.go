package tidal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// DownloadConfig holds download configuration
type DownloadConfig struct {
	OutputDir          string
	Quality            string
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

// ThirdPartyAPIResponse is the v1 API response format
type ThirdPartyAPIResponse struct {
	OriginalTrackURL string `json:"OriginalTrackUrl"`
}

// ThirdPartyAPIResponseV2 is the v2 API response format with manifest
type ThirdPartyAPIResponseV2 struct {
	Version string `json:"version"`
	Data    struct {
		TrackID           int64  `json:"trackId"`
		AssetPresentation string `json:"assetPresentation"`
		AudioMode         string `json:"audioMode"`
		AudioQuality      string `json:"audioQuality"`
		ManifestMimeType  string `json:"manifestMimeType"`
		ManifestHash      string `json:"manifestHash"`
		Manifest          string `json:"manifest"`
		BitDepth          int    `json:"bitDepth"`
		SampleRate        int    `json:"sampleRate"`
	} `json:"data"`
}

// BTSManifest is the BTS (application/vnd.tidal.bts) manifest format
type BTSManifest struct {
	MimeType       string   `json:"mimeType"`
	Codecs         string   `json:"codecs"`
	EncryptionType string   `json:"encryptionType"`
	URLs           []string `json:"urls"`
}

// MPD represents a DASH MPD manifest
type MPD struct {
	XMLName xml.Name `xml:"MPD"`
	Period  struct {
		AdaptationSet struct {
			Representation struct {
				SegmentTemplate struct {
					Initialization string `xml:"initialization,attr"`
					Media          string `xml:"media,attr"`
					Timeline       struct {
						Segments []struct {
							Duration int `xml:"d,attr"`
							Repeat   int `xml:"r,attr"`
						} `xml:"S"`
					} `xml:"SegmentTimeline"`
				} `xml:"SegmentTemplate"`
			} `xml:"Representation"`
		} `xml:"AdaptationSet"`
	} `xml:"Period"`
}

// thirdPartyResult holds the result from a parallel third-party API request
type thirdPartyResult struct {
	apiURL      string
	downloadURL string
	isManifest  bool
	err         error
}

// GetThirdPartyAPIs returns the list of third-party API URLs (base64 decoded)
func GetThirdPartyAPIs() []string {
	// Base64 encoded API URLs for obfuscation
	encodedAPIs := []string{
		"dm9nZWwucXFkbC5zaXRl",         // vogel.qqdl.site
		"bWF1cy5xcWRsLnNpdGU=",         // maus.qqdl.site
		"aHVuZC5xcWRsLnNpdGU=",         // hund.qqdl.site
		"a2F0emUucXFkbC5zaXRl",         // katze.qqdl.site
		"d29sZi5xcWRsLnNpdGU=",         // wolf.qqdl.site
		"dGlkYWwua2lub3BsdXMub25saW5l", // tidal.kinoplus.online
		"dGlkYWwtYXBpLmJpbmltdW0ub3Jn", // tidal-api.binimum.org
		"dHJpdG9uLnNxdWlkLnd0Zg==",     // triton.squid.wtf
	}

	var apis []string
	for _, encoded := range encodedAPIs {
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			continue
		}
		apis = append(apis, "https://"+string(decoded))
	}

	return apis
}

// GetDownloadURLFromThirdParty fetches download URL from third-party APIs.
// Uses parallel requests to all APIs and returns the first success.
func (c *Client) GetDownloadURLFromThirdParty(ctx context.Context, trackID int64, quality string) (string, bool, error) {
	apis := GetThirdPartyAPIs()
	if len(apis) == 0 {
		return "", false, fmt.Errorf("no third-party APIs available")
	}

	resultChan := make(chan thirdPartyResult, len(apis))

	// Create a client with longer timeout for parallel requests
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	log.Printf("Requesting download URL from %d third-party APIs in parallel...", len(apis))

	// Start all requests in parallel
	for _, apiURL := range apis {
		go func(api string) {
			url := fmt.Sprintf("%s/track/?id=%d&quality=%s", api, trackID, quality)

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				resultChan <- thirdPartyResult{apiURL: api, err: err}
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				resultChan <- thirdPartyResult{apiURL: api, err: err}
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				resultChan <- thirdPartyResult{apiURL: api, err: fmt.Errorf("HTTP %d", resp.StatusCode)}
				return
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				resultChan <- thirdPartyResult{apiURL: api, err: err}
				return
			}

			// Try v2 format first (object with manifest)
			var v2Response ThirdPartyAPIResponseV2
			if err := json.Unmarshal(body, &v2Response); err == nil && v2Response.Data.Manifest != "" {
				resultChan <- thirdPartyResult{
					apiURL:      api,
					downloadURL: v2Response.Data.Manifest,
					isManifest:  true,
					err:         nil,
				}
				return
			}

			// Fallback to v1 format (array with OriginalTrackUrl)
			var v1Responses []ThirdPartyAPIResponse
			if err := json.Unmarshal(body, &v1Responses); err == nil {
				for _, item := range v1Responses {
					if item.OriginalTrackURL != "" {
						resultChan <- thirdPartyResult{
							apiURL:      api,
							downloadURL: item.OriginalTrackURL,
							isManifest:  false,
							err:         nil,
						}
						return
					}
				}
			}

			resultChan <- thirdPartyResult{apiURL: api, err: fmt.Errorf("no download URL or manifest in response")}
		}(apiURL)
	}

	// Collect results - return first success
	var lastError error
	var errors []string

	for i := 0; i < len(apis); i++ {
		select {
		case <-ctx.Done():
			return "", false, ctx.Err()
		case result := <-resultChan:
			if result.err == nil && result.downloadURL != "" {
				log.Printf("Got response from: %s", result.apiURL)
				return result.downloadURL, result.isManifest, nil
			}
			errMsg := result.err.Error()
			if len(errMsg) > 50 {
				errMsg = errMsg[:50] + "..."
			}
			errors = append(errors, fmt.Sprintf("%s: %s", result.apiURL, errMsg))
			lastError = result.err
		}
	}

	log.Printf("All third-party APIs failed")
	for _, e := range errors {
		log.Printf("  - %s", e)
	}

	return "", false, fmt.Errorf("all %d third-party APIs failed: %v", len(apis), lastError)
}

// ParseManifest extracts download URL from base64 encoded manifest.
// Supports both BTS (JSON with direct URLs) and DASH (XML with segments) formats.
// Returns: directURL (for BTS), or initURL + mediaURLs (for DASH)
func ParseManifest(manifestB64 string) (directURL string, initURL string, mediaURLs []string, err error) {
	// Decode base64 manifest
	manifestBytes, err := base64.StdEncoding.DecodeString(manifestB64)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	manifestStr := string(manifestBytes)

	// Check if it's BTS format (JSON) or DASH format (XML)
	if strings.HasPrefix(manifestStr, "{") {
		// BTS format - JSON with direct URLs
		var btsManifest BTSManifest
		if err := json.Unmarshal(manifestBytes, &btsManifest); err != nil {
			return "", "", nil, fmt.Errorf("failed to parse BTS manifest: %w", err)
		}

		if len(btsManifest.URLs) == 0 {
			return "", "", nil, fmt.Errorf("no URLs in BTS manifest")
		}

		log.Printf("Manifest: BTS format (%s, %s)", btsManifest.MimeType, btsManifest.Codecs)
		return btsManifest.URLs[0], "", nil, nil
	}

	// DASH format - XML with segments
	log.Printf("Manifest: DASH format")

	// Parse XML
	var mpd MPD
	if err := xml.Unmarshal(manifestBytes, &mpd); err != nil {
		return "", "", nil, fmt.Errorf("failed to parse manifest XML: %w", err)
	}

	segTemplate := mpd.Period.AdaptationSet.Representation.SegmentTemplate
	initURL = segTemplate.Initialization
	mediaTemplate := segTemplate.Media

	if initURL == "" || mediaTemplate == "" {
		// Fallback: try regex extraction
		initRe := regexp.MustCompile(`initialization="([^"]+)"`)
		mediaRe := regexp.MustCompile(`media="([^"]+)"`)

		if match := initRe.FindStringSubmatch(manifestStr); len(match) > 1 {
			initURL = match[1]
		}
		if match := mediaRe.FindStringSubmatch(manifestStr); len(match) > 1 {
			mediaTemplate = match[1]
		}
	}

	if initURL == "" {
		return "", "", nil, fmt.Errorf("no initialization URL found in manifest")
	}

	// Unescape HTML entities in URLs
	initURL = strings.ReplaceAll(initURL, "&amp;", "&")
	mediaTemplate = strings.ReplaceAll(mediaTemplate, "&amp;", "&")

	// Calculate segment count from timeline
	segmentCount := 0
	for _, seg := range segTemplate.Timeline.Segments {
		segmentCount += seg.Repeat + 1
	}

	// If no segments found via XML, try regex
	if segmentCount == 0 {
		segRe := regexp.MustCompile(`<S d="\d+"(?: r="(\d+)")?`)
		matches := segRe.FindAllStringSubmatch(manifestStr, -1)
		for _, match := range matches {
			repeat := 0
			if len(match) > 1 && match[1] != "" {
				fmt.Sscanf(match[1], "%d", &repeat)
			}
			segmentCount += repeat + 1
		}
	}

	// Generate media URLs for each segment
	for i := 1; i <= segmentCount; i++ {
		mediaURL := strings.ReplaceAll(mediaTemplate, "$Number$", fmt.Sprintf("%d", i))
		mediaURLs = append(mediaURLs, mediaURL)
	}

	return "", initURL, mediaURLs, nil
}

// DownloadFromManifest downloads audio from a manifest (supports BTS and DASH formats).
// Note: DASH format requires ffmpeg to be installed for remuxing segments to FLAC.
func (c *Client) DownloadFromManifest(ctx context.Context, manifestB64, outputPath string) error {
	directURL, initURL, mediaURLs, err := ParseManifest(manifestB64)
	if err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Create HTTP client with longer timeout for downloads
	client := &http.Client{
		Timeout: 600 * time.Second, // 10 minutes - sufficient for large files over slow connections
	}

	// If we have a direct URL (BTS format), download directly
	if directURL != "" {
		log.Printf("Downloading file from direct URL...")

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, directURL, nil)
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to download file: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("download failed with status %d", resp.StatusCode)
		}

		out, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer out.Close()

		written, err := io.Copy(out, resp.Body)
		if err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		log.Printf("Downloaded: %.2f MB", float64(written)/(1024*1024))
		return nil
	}

	// DASH format - download segments to temporary M4A file, then remux to FLAC
	log.Printf("Downloading %d DASH segments...", len(mediaURLs)+1)

	// Create temporary file for M4A segments
	tempPath := outputPath + ".m4a.tmp"
	out, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	// Download initialization segment
	log.Printf("Downloading init segment...")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, initURL, nil)
	if err != nil {
		out.Close()
		os.Remove(tempPath)
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		out.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to download init segment: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		out.Close()
		os.Remove(tempPath)
		return fmt.Errorf("init segment download failed with status %d", resp.StatusCode)
	}
	_, err = io.Copy(out, resp.Body)
	resp.Body.Close()
	if err != nil {
		out.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to write init segment: %w", err)
	}

	// Download media segments
	var totalBytes int64
	for i, mediaURL := range mediaURLs {
		select {
		case <-ctx.Done():
			out.Close()
			os.Remove(tempPath)
			return ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
		if err != nil {
			out.Close()
			os.Remove(tempPath)
			return err
		}

		resp, err := client.Do(req)
		if err != nil {
			out.Close()
			os.Remove(tempPath)
			return fmt.Errorf("failed to download segment %d: %w", i+1, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			out.Close()
			os.Remove(tempPath)
			return fmt.Errorf("segment %d download failed with status %d", i+1, resp.StatusCode)
		}
		n, err := io.Copy(out, resp.Body)
		totalBytes += n
		resp.Body.Close()
		if err != nil {
			out.Close()
			os.Remove(tempPath)
			return fmt.Errorf("failed to write segment %d: %w", i+1, err)
		}

		if (i+1)%10 == 0 || i == len(mediaURLs)-1 {
			log.Printf("Downloaded segment %d/%d (%.2f MB)", i+1, len(mediaURLs), float64(totalBytes)/(1024*1024))
		}
	}

	// Close temp file before remuxing
	out.Close()

	log.Printf("Downloaded %.2f MB total, converting to FLAC...", float64(totalBytes)/(1024*1024))

	// Remux M4A to FLAC using ffmpeg
	// DASH segments are in fMP4 container with FLAC codec, need to extract to native FLAC
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", tempPath, "-vn", "-c:a", "flac", outputPath)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// If ffmpeg fails, try to keep the M4A file for debugging
		m4aPath := strings.TrimSuffix(outputPath, ".flac") + ".m4a"
		os.Rename(tempPath, m4aPath)
		return fmt.Errorf("ffmpeg conversion failed (M4A saved as %s): %w - %s", m4aPath, err, stderr.String())
	}

	// Remove temp file
	os.Remove(tempPath)
	log.Printf("Conversion complete")

	return nil
}

// DownloadTrackFromThirdParty downloads a track using third-party APIs
func (c *Client) DownloadTrackFromThirdParty(ctx context.Context, trackID int64, cfg DownloadConfig) *DownloadResult {
	// Get track info first
	track, err := c.GetTrack(ctx, trackID)
	if err != nil {
		return &DownloadResult{Error: err}
	}

	// Get download URL from third-party APIs
	downloadURL, isManifest, err := c.GetDownloadURLFromThirdParty(ctx, trackID, cfg.Quality)
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
	if isManifest {
		// Download from manifest (BTS or DASH format)
		if err := c.DownloadFromManifest(ctx, downloadURL, filePath); err != nil {
			return &DownloadResult{Error: err}
		}
	} else {
		// Direct download
		if err := c.downloadFile(ctx, downloadURL, filePath); err != nil {
			return &DownloadResult{Error: err}
		}
	}

	// Get file size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return &DownloadResult{Error: err}
	}

	return &DownloadResult{
		Path:       filePath,
		SourceType: "tidal",
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

	// Get stream URL
	streamURL, err := c.GetStreamURL(ctx, trackID, cfg.Quality)
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
		SourceType: "tidal",
		SourceID:   fmt.Sprintf("%d", trackID),
		FileSize:   fileInfo.Size(),
	}
}

func (c *Client) downloadFile(ctx context.Context, url, filePath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
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

	_, err = io.Copy(out, resp.Body)
	return err
}

func buildFilename(track *Track, format string, includeTrackNumber bool) string {
	// Sanitize artist and title for filename
	artist := sanitizeForFilename(track.Artist.Name)
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
	track, err := c.FindByISRC(ctx, isrc)
	if err != nil {
		return &DownloadResult{Error: err}
	}

	return c.DownloadTrack(ctx, track.ID, cfg)
}

// DownloadFromISRCWithThirdParty downloads a track by ISRC using third-party APIs
func (c *Client) DownloadFromISRCWithThirdParty(ctx context.Context, isrc string, cfg DownloadConfig) *DownloadResult {
	// Search for track by ISRC
	track, err := c.FindByISRC(ctx, isrc)
	if err != nil {
		return &DownloadResult{Error: err}
	}

	return c.DownloadTrackFromThirdParty(ctx, track.ID, cfg)
}

// DownloadWithFallback attempts download with quality fallback
func (c *Client) DownloadWithFallback(ctx context.Context, trackID int64, cfg DownloadConfig) *DownloadResult {
	// Try highest quality first
	cfg.Quality = QualityHiRes
	result := c.DownloadTrack(ctx, trackID, cfg)

	if result.Error != nil {
		// Try Lossless as fallback
		cfg.Quality = QualityLossless
		result = c.DownloadTrack(ctx, trackID, cfg)
	}

	if result.Error != nil {
		// Try High as last resort
		cfg.Quality = QualityHigh
		result = c.DownloadTrack(ctx, trackID, cfg)
	}

	return result
}

// DownloadWithThirdPartyFallback attempts download using third-party APIs with quality fallback
func (c *Client) DownloadWithThirdPartyFallback(ctx context.Context, trackID int64, cfg DownloadConfig) *DownloadResult {
	// Try highest quality first
	cfg.Quality = QualityHiRes
	result := c.DownloadTrackFromThirdParty(ctx, trackID, cfg)

	if result.Error != nil {
		// Try Lossless as fallback
		cfg.Quality = QualityLossless
		result = c.DownloadTrackFromThirdParty(ctx, trackID, cfg)
	}

	if result.Error != nil {
		// Try High as last resort
		cfg.Quality = QualityHigh
		result = c.DownloadTrackFromThirdParty(ctx, trackID, cfg)
	}

	return result
}
