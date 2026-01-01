package lyrics

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"spotisync/internal/utils"
)

// LyricsResult represents the result of a lyrics fetch operation
type LyricsResult struct {
	Synced   []Line
	Unsynced string
	Source   string
	Error    error
}

// Line represents a line of synced lyrics with timestamp
type Line struct {
	Start int64
	Text  string
}

// LyricsFetcher handles fetching lyrics from various sources
type LyricsFetcher struct {
	client http.Client
	config LyricsConfig
}

// LyricsConfig holds configuration for the lyrics fetcher
type LyricsConfig struct {
	Timeout   time.Duration
	UserAgent string
}

// NewLyricsFetcher creates a new lyrics fetcher with the given configuration
func NewLyricsFetcher(config LyricsConfig) *LyricsFetcher {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.UserAgent == "" {
		config.UserAgent = "SpotiSync/1.0"
	}

	return &LyricsFetcher{
		client: http.Client{
			Timeout: config.Timeout,
		},
		config: config,
	}
}

// FetchFromMusixmatch fetches lyrics from Musixmatch
func (f *LyricsFetcher) FetchFromMusixmatch(trackTitle, artistName, apiKey string) *LyricsResult {
	// Musixmatch API requires a token, so this is a simplified version
	// In production, you'd need to handle the token authentication
	url := fmt.Sprintf(
		"https://api.musixmatch.com/ws/1.1/matcher.lyrics.get?apikey=%s&format=json&q_track=%s&q_artist=%s",
		apiKey,
		strings.ReplaceAll(trackTitle, " ", "+"),
		strings.ReplaceAll(artistName, " ", "+"),
	)

	return f.fetchAndParse(url, "musixmatch")
}

// FetchFromGenius fetches lyrics from Genius
func (f *LyricsFetcher) FetchFromGenius(trackTitle, artistName, accessToken string) *LyricsResult {
	// First search for the song
	searchURL := fmt.Sprintf(
		"https://api.genius.com/search?q=%s %s",
		strings.ReplaceAll(trackTitle, " ", "+"),
		strings.ReplaceAll(artistName, " ", "+"),
	)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return &LyricsResult{Error: err}
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", f.config.UserAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return &LyricsResult{Error: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &LyricsResult{Error: fmt.Errorf("genius API failed: %d", resp.StatusCode)}
	}

	var searchResult struct {
		Response struct {
			Hits []struct {
				Result struct {
					ID          int64  `json:"id"`
					Title       string `json:"title"`
					URL         string `json:"url"`
					Path        string `json:"path"`
					ArtistNames string `json:"artist_names"`
				} `json:"result"`
			} `json:"hits"`
		} `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return &LyricsResult{Error: err}
	}

	if len(searchResult.Response.Hits) == 0 {
		return &LyricsResult{Error: fmt.Errorf("no lyrics found")}
	}

	// Get the first result
	song := searchResult.Response.Hits[0].Result

	// Fetch the actual lyrics page
	lyricsURL := fmt.Sprintf("https://api.genius.com/songs/%d", song.ID)
	req, err = http.NewRequest("GET", lyricsURL, nil)
	if err != nil {
		return &LyricsResult{Error: err}
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", f.config.UserAgent)

	resp, err = f.client.Do(req)
	if err != nil {
		return &LyricsResult{Error: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &LyricsResult{Error: fmt.Errorf("genius API failed: %d", resp.StatusCode)}
	}

	var songResult struct {
		Response struct {
			Song struct {
				Lyrics string `json:"lyrics"`
			} `json:"song"`
		} `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&songResult); err != nil {
		return &LyricsResult{Error: err}
	}

	return &LyricsResult{
		Unsynced: songResult.Response.Song.Lyrics,
		Source:   "genius",
	}
}

// FetchFromOvo lumient fetches lyrics from OVOMusic
func (f *LyricsFetcher) FetchFromOVOMusic(trackTitle, artistName string) *LyricsResult {
	url := fmt.Sprintf(
		"https://ovosongapi.com/api/songs/%s - %s",
		trackTitle,
		artistName,
	)

	return f.fetchAndParse(url, "ovomusic")
}

// FetchFromAzLyrics fetches lyrics from AZLyrics
func (f *LyricsFetcher) FetchFromAzLyrics(trackTitle, artistName string) *LyricsResult {
	artistName = strings.ToLower(artistName)
	trackTitle = strings.ToLower(trackTitle)

	// Remove special characters
	artistName = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, artistName)

	trackTitle = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, trackTitle)

	url := fmt.Sprintf(
		"https://www.azlyrics.com/lyrics/%s/%s.html",
		artistName,
		trackTitle,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &LyricsResult{Error: err}
	}
	req.Header.Set("User-Agent", f.config.UserAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return &LyricsResult{Error: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &LyricsResult{Error: fmt.Errorf("azlyrics fetch failed: %d", resp.StatusCode)}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &LyricsResult{Error: err}
	}

	// AZLyrics returns HTML, so we need to extract the lyrics
	content := string(body)
	start := strings.Index(content, "<!-- Usage of azlyrics.com content by any third-party lyrics provider is prohibited by our licensing agreement. Sorry about that. -->")
	if start == -1 {
		return &LyricsResult{Error: fmt.Errorf("lyrics not found")}
	}

	end := strings.Index(content[start:], "<!-- MxM banner -->")
	if end == -1 {
		return &LyricsResult{Error: fmt.Errorf("lyrics parsing failed")}
	}

	lyrics := content[start : start+end]
	// Remove HTML tags and clean up
	lyrics = strings.ReplaceAll(lyrics, "<br>", "\n")
	lyrics = strings.Map(func(r rune) rune {
		if r == '<' {
			return -1
		}
		if r == '>' {
			return -1
		}
		return r
	}, lyrics)

	lyrics = strings.TrimSpace(lyrics)
	return &LyricsResult{
		Unsynced: lyrics,
		Source:   "azlyrics",
	}
}

// fetchAndParse is a helper to fetch and parse lyrics from a URL
func (f *LyricsFetcher) fetchAndParse(url, source string) *LyricsResult {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &LyricsResult{Error: err}
	}
	req.Header.Set("User-Agent", f.config.UserAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return &LyricsResult{Error: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &LyricsResult{Error: fmt.Errorf("%s API failed: %d", source, resp.StatusCode)}
	}

	var result struct {
		Message struct {
			Body struct {
				Lyrics struct {
					LyricsBody      string `json:"lyrics_body"`
					LyricsCopyright string `json:"lyrics_copyright"`
					LyricsID        int64  `json:"lyrics_id"`
				} `json:"lyrics"`
			} `json:"body"`
		} `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return &LyricsResult{Error: err}
	}

	return &LyricsResult{
		Unsynced: result.Message.Body.Lyrics.LyricsBody,
		Source:   source,
	}
}

// ParseSyncedLyrics parses timed lyrics (LRC format)
func ParseSyncedLyrics(lrcContent string) []Line {
	lines := strings.Split(lrcContent, "\n")
	result := make([]Line, 0, len(lines))

	for _, line := range lines {
		// LRC format: [mm:ss.xx]lyrics
		if len(line) > 10 && line[0] == '[' {
			tagEnd := strings.Index(line, "]")
			if tagEnd == -1 {
				continue
			}

			timestamp := line[1:tagEnd]
			text := line[tagEnd+1:]

			// Parse timestamp
			parts := strings.Split(timestamp, ":")
			if len(parts) != 2 {
				continue
			}

			var minutes, seconds float64
			fmt.Sscanf(parts[0], "%f", &minutes)
			fmt.Sscanf(parts[1], "%f", &seconds)

			startTime := int64(minutes*60*1000 + seconds*1000)

			result = append(result, Line{
				Start: startTime,
				Text:  strings.TrimSpace(text),
			})
		}
	}

	return result
}

// FormatDuration converts milliseconds to LRC timestamp format
func FormatDuration(ms int64) string {
	minutes := ms / 60000
	seconds := (ms % 60000) / 1000
	hundredths := (ms % 1000) / 10
	return fmt.Sprintf("[%02d:%02d.%02d]", minutes, seconds, hundredths)
}

// LRCLibResponse represents the response from LRCLIB API
type LRCLibResponse struct {
	ID           int64   `json:"id"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

// FetchFromLRCLib fetches lyrics from LRCLIB API
func (f *LyricsFetcher) FetchFromLRCLib(artistName, trackName string) *LyricsResult {
	// URL encode the parameters
	encodedArtist := url.QueryEscape(artistName)
	encodedTrack := url.QueryEscape(trackName)

	url := fmt.Sprintf(
		"https://lrclib.net/api/get?artist_name=%s&track_name=%s",
		encodedArtist,
		encodedTrack,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &LyricsResult{Error: err}
	}
	req.Header.Set("User-Agent", f.config.UserAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return &LyricsResult{Error: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return &LyricsResult{Error: fmt.Errorf("lyrics not found")}
		}
		return &LyricsResult{Error: fmt.Errorf("lrclib API failed: %d", resp.StatusCode)}
	}

	var result LRCLibResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return &LyricsResult{Error: err}
	}

	if result.Instrumental {
		return &LyricsResult{
			Unsynced: "[instrumental]",
			Source:   "lrclib",
		}
	}

	// Prefer synced lyrics if available
	if result.SyncedLyrics != "" {
		return &LyricsResult{
			Synced:   ParseSyncedLyrics(result.SyncedLyrics),
			Unsynced: result.PlainLyrics,
			Source:   "lrclib",
		}
	}

	return &LyricsResult{
		Unsynced: result.PlainLyrics,
		Source:   "lrclib",
	}
}

// BuildLyricsFilename builds the filename for lyrics file
func BuildLyricsFilename(artist, title string, discNumber, trackNumber int) (string, error) {
	// Reject inputs containing ".." sequences
	if strings.Contains(artist, "..") || strings.Contains(title, "..") {
		return "", fmt.Errorf("invalid input: path traversal sequence detected")
	}

	// Format: Artist - Title (Disc 01-Track 02).lrc
	discStr := ""
	if discNumber > 0 {
		discStr = fmt.Sprintf(" (Disc %02d", discNumber)
		if trackNumber > 0 {
			discStr += fmt.Sprintf("-Track %02d", trackNumber)
		}
		discStr += ")"
	} else if trackNumber > 0 {
		discStr = fmt.Sprintf(" (Track %02d)", trackNumber)
	}

	// Sanitize artist and title for filename using SanitizePath
	safeArtist := utils.SanitizePath(artist)
	safeTitle := utils.SanitizePath(title)

	// Normalize the path to remove any redundant separators
	filename := fmt.Sprintf("%s - %s%s.lrc", safeArtist, safeTitle, discStr)
	return filepath.Clean(filename), nil
}

// CheckLyricsExists checks if lyrics file already exists
func CheckLyricsExists(outputDir, artist, title string, discNumber, trackNumber int) bool {
	filename, err := BuildLyricsFilename(artist, title, discNumber, trackNumber)
	if err != nil {
		return false
	}
	filepath := filepath.Join(outputDir, filename)

	_, err = os.Stat(filepath)
	return err == nil
}

// plainToLRC converts plain lyrics to LRC format with estimated timestamps
func plainToLRC(plainLyrics string, durationMs int64) string {
	if plainLyrics == "" {
		return ""
	}

	lines := strings.Split(plainLyrics, "\n")
	if len(lines) == 0 {
		return ""
	}

	// Estimate time per line (total duration / number of lines)
	avgTimePerLine := durationMs / int64(len(lines))
	if avgTimePerLine == 0 {
		avgTimePerLine = 3000 // Default to 3 seconds per line
	}

	var lrcLines []string
	currentTime := int64(0)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		timestamp := FormatDuration(currentTime)
		lrcLines = append(lrcLines, timestamp+line)
		currentTime += avgTimePerLine
	}

	return strings.Join(lrcLines, "\n")
}

// FetchFromLRCLibSearch fetches lyrics using LRCLIB search API as fallback
func (f *LyricsFetcher) FetchFromLRCLibSearch(artistName, trackName string) *LyricsResult {
	query := fmt.Sprintf("%s %s", artistName, trackName)
	searchURL := fmt.Sprintf("https://lrclib.net/api/search?q=%s", url.QueryEscape(query))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return &LyricsResult{Error: err}
	}
	req.Header.Set("User-Agent", f.config.UserAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return &LyricsResult{Error: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &LyricsResult{Error: fmt.Errorf("lrclib search failed: %d", resp.StatusCode)}
	}

	var results []LRCLibResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return &LyricsResult{Error: err}
	}

	if len(results) == 0 {
		return &LyricsResult{Error: fmt.Errorf("no results found")}
	}

	// Find best match - prefer one with synced lyrics
	var best *LRCLibResponse
	for i := range results {
		if results[i].SyncedLyrics != "" {
			best = &results[i]
			break
		}
		if best == nil && results[i].PlainLyrics != "" {
			best = &results[i]
		}
	}

	if best == nil {
		best = &results[0]
	}

	// Convert to LyricsResult
	if best.Instrumental {
		return &LyricsResult{
			Unsynced: "[instrumental]",
			Source:   "lrclib-search",
		}
	}

	if best.SyncedLyrics != "" {
		return &LyricsResult{
			Synced:   ParseSyncedLyrics(best.SyncedLyrics),
			Unsynced: best.PlainLyrics,
			Source:   "lrclib-search",
		}
	}

	return &LyricsResult{
		Unsynced: best.PlainLyrics,
		Source:   "lrclib-search",
	}
}

// simplifyTrackName removes common suffixes like "(feat. X)", "(Remastered)", etc.
func simplifyTrackName(name string) string {
	// Remove content in parentheses
	if idx := strings.Index(name, "("); idx > 0 {
		name = strings.TrimSpace(name[:idx])
	}
	// Remove content after " - " (like "From the Motion Picture")
	if idx := strings.Index(name, " - "); idx > 0 {
		name = strings.TrimSpace(name[:idx])
	}
	return name
}

// FetchLyricsWithFallback tries multiple strategies to find lyrics
func (f *LyricsFetcher) FetchLyricsWithFallback(artistName, trackName string) *LyricsResult {
	// 1. Try exact match
	result := f.FetchFromLRCLib(artistName, trackName)
	if result.Error == nil && (len(result.Synced) > 0 || result.Unsynced != "") {
		return result
	}

	// 2. Try search
	result = f.FetchFromLRCLibSearch(artistName, trackName)
	if result.Error == nil && (len(result.Synced) > 0 || result.Unsynced != "") {
		return result
	}

	// 3. Try with simplified track name
	simplifiedTrack := simplifyTrackName(trackName)
	if simplifiedTrack != trackName {
		result = f.FetchFromLRCLib(artistName, simplifiedTrack)
		if result.Error == nil && (len(result.Synced) > 0 || result.Unsynced != "") {
			return result
		}

		result = f.FetchFromLRCLibSearch(artistName, simplifiedTrack)
		if result.Error == nil && (len(result.Synced) > 0 || result.Unsynced != "") {
			return result
		}
	}

	return &LyricsResult{Error: fmt.Errorf("lyrics not found after all fallbacks")}
}
