package spotify

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Track represents a Spotify track
type Track struct {
	ID           string
	Name         string
	Artist       string
	Artists      []string
	Album        string
	AlbumID      string
	AlbumArtist  string
	AlbumArtists []string
	TrackNumber  int
	DiscNumber   int
	DurationMs   int
	ISRC         string
	ReleaseYear  int
	ReleaseDate  string
	TotalTracks  int
	CoverArtURL  string
	Explicit     bool
}

// Album represents a Spotify album
type Album struct {
	ID          string
	Name        string
	Artist      string
	ArtistID    string
	ReleaseDate string
	TotalTracks int
	CoverArtURL string
}

// Playlist represents a Spotify playlist
type Playlist struct {
	ID          string
	Name        string
	Owner       string
	TotalTracks int
	CoverArtURL string
}

// ISRCInfo contains parsed ISRC information
type ISRCInfo struct {
	Country string
	Year    string
}

var (
	spotifyURLRegex = regexp.MustCompile(`spotify:(album|playlist|track|artist):([a-zA-Z0-9]+)`)
	embedURLRegex   = regexp.MustCompile(`spotify:album:([a-zA-Z0-9]+)`)
)

// ParseSpotifyURL parses a Spotify URL and returns type and ID
func ParseSpotifyURL(input string) (string, string, error) {
	if input == "" {
		return "", "", errors.New("empty URL")
	}

	// Handle Spotify URI format
	if strings.HasPrefix(input, "spotify:") {
		matches := spotifyURLRegex.FindStringSubmatch(input)
		if len(matches) == 3 {
			return matches[1], matches[2], nil
		}
		return "", "", errors.New("invalid Spotify URI")
	}

	// Handle embed URL
	if strings.Contains(input, "embed.spotify.com") {
		matches := embedURLRegex.FindStringSubmatch(input)
		if len(matches) == 2 {
			return "album", matches[1], nil
		}
		return "", "", errors.New("invalid embed URL")
	}

	// Handle regular URL
	u, err := url.Parse(input)
	if err != nil {
		return "", "", err
	}

	// Check host
	if !strings.Contains(u.Host, "spotify.com") {
		return "", "", errors.New("not a Spotify URL")
	}

	// Parse path
	parts := strings.Split(u.Path, "/")
	if len(parts) < 2 {
		return "", "", errors.New("invalid URL path")
	}

	// Handle international URLs like /intl-en/album/...
	// These have the format /{lang-code}/album/... where lang-code starts with "intl-"
	index := 0
	if len(parts) > 3 && strings.HasPrefix(parts[1], "intl-") {
		index = 1
	}

	// Now we expect the format: /{type}/{id} or /{lang-code}/{type}/{id}
	// parts[index+1] should be the type and parts[index+2] should be the ID
	if index+2 >= len(parts) {
		return "", "", errors.New("invalid URL path")
	}

	spotifyType := parts[index+1]
	spotifyID := parts[index+2]

	// Handle user playlists
	if spotifyType == "user" && len(parts) > index+3 && parts[index+3] == "playlist" {
		spotifyType = "playlist"
		if len(parts) > index+4 {
			spotifyID = parts[index+4]
		}
	}

	validTypes := map[string]bool{
		"album":    true,
		"playlist": true,
		"track":    true,
		"artist":   true,
	}

	if !validTypes[spotifyType] {
		return "", "", errors.New("invalid Spotify type")
	}

	return spotifyType, spotifyID, nil
}

// ValidateSpotifyURL validates a Spotify URL
func ValidateSpotifyURL(input string) bool {
	_, _, err := ParseSpotifyURL(input)
	return err == nil
}

// GetSpotifyID extracts the Spotify ID from a URL
func GetSpotifyID(input string) (string, error) {
	_, id, err := ParseSpotifyURL(input)
	return id, err
}

// GetSpotifyType extracts the Spotify type from a URL
func GetSpotifyType(input string) (string, error) {
	spotifyType, _, err := ParseSpotifyURL(input)
	return spotifyType, err
}

// ParseISRC parses an ISRC code
func ParseISRC(isrc string) *ISRCInfo {
	if len(isrc) < 12 {
		return nil
	}

	// Remove dashes
	isrc = strings.ReplaceAll(isrc, "-", "")

	if len(isrc) != 12 {
		return nil
	}

	return &ISRCInfo{
		Country: isrc[0:2],
		Year:    isrc[5:7], // Year is characters 5-6 (0-indexed)
	}
}

// ISRCMatch compares two ISRC codes
func ISRCMatch(isrc1, isrc2 string) bool {
	if isrc1 == "" || isrc2 == "" {
		return false
	}

	// Normalize by removing dashes
	norm1 := strings.ReplaceAll(isrc1, "-", "")
	norm2 := strings.ReplaceAll(isrc2, "-", "")

	return strings.EqualFold(norm1, norm2)
}

// CleanTrackName removes common suffixes from track names
func CleanTrackName(name string) string {
	// Remove remastered
	name = regexp.MustCompile(`\s*\(Remastered(?: \d+)?\)`).ReplaceAllString(name, "")
	// Remove live
	name = regexp.MustCompile(`\s*-\s*Live$`).ReplaceAllString(name, "")
	// Remove version
	name = regexp.MustCompile(`\s*\(.*Version.*\)`).ReplaceAllString(name, "")
	// Remove remix
	name = regexp.MustCompile(`\s*\[.*Remix.*\]`).ReplaceAllString(name, "")

	return strings.TrimSpace(name)
}

// FormatDuration formats milliseconds to mm:ss
func FormatDuration(ms int) string {
	seconds := ms / 1000
	minutes := seconds / 60
	seconds = seconds % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// MatchScore calculates a match score for a track against a query
func (t *Track) MatchScore(query string) int {
	score := 0

	query = strings.ToLower(query)
	name := strings.ToLower(t.Name)
	artist := strings.ToLower(t.Artist)

	if query == name {
		score += 60
	} else if strings.Contains(name, query) {
		score += 10
	}

	if query == artist {
		score += 50
	} else if strings.Contains(artist, query) {
		score += 20
	}

	return score
}
