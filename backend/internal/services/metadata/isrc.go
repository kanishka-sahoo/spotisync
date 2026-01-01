package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// ISRC format: ^[A-Z]{2}[A-Z0-9]{3}[0-9]{7}$
// Country code (2 uppercase letters) + Registrant code (3 alphanumeric) + Year (2 digits) + Serial (5 digits)
var isrcRegex = regexp.MustCompile(`^[A-Z]{2}[A-Z0-9]{3}[0-9]{7}$`)

// ValidateISRC validates the format of an ISRC code
// ISRC format: ^[A-Z]{2}[A-Z0-9]{3}[0-9]{7}$
// - First 2 characters: Country code (uppercase letters only)
// - Next 3 characters: Registrant code (uppercase letters or digits)
// - Next 2 characters: Year of registration (00-99)
// - Last 5 characters: Serial number (digits only)
// Total length: 12 characters
func ValidateISRC(isrc string) error {
	if isrc == "" {
		return nil // Empty ISRC is allowed (optional field)
	}

	// Check maximum length
	if len(isrc) > 15 {
		return fmt.Errorf("ISRC code too long (max 15 characters)")
	}

	// Check minimum length
	if len(isrc) < 12 {
		return fmt.Errorf("ISRC code too short (min 12 characters)")
	}

	// Validate format with regex
	if !isrcRegex.MatchString(isrc) {
		return fmt.Errorf("invalid ISRC format: %s (expected format: CCXXXYYNNNNN)", isrc)
	}

	// Additional validation: year should be reasonable (00-99 is technically valid per spec)
	yearStr := isrc[5:7]
	for _, c := range yearStr {
		if c < '0' || c > '9' {
			return fmt.Errorf("invalid ISRC: year must contain only digits")
		}
	}

	// Additional validation: serial should be digits only
	serialStr := isrc[7:]
	for _, c := range serialStr {
		if c < '0' || c > '9' {
			return fmt.Errorf("invalid ISRC: serial must contain only digits")
		}
	}

	return nil
}

// ISRCMetadata represents ISRC metadata for a track
type ISRCMetadata struct {
	ISRC       string
	Artist     string
	Album      string
	Title      string
	TrackNum   int
	DiscNum    int
	OutputPath string
}

// CheckISRCExists checks if a file with the given ISRC already exists in the directory
func CheckISRCExists(outputDir, isrc string) (string, bool) {
	if isrc == "" {
		return "", false
	}

	// Build ISRC index for the directory
	index := BuildISRCIndex(outputDir)

	if path, exists := index[isrc]; exists {
		return path, true
	}

	return "", false
}

// CheckISRCExistsParallel checks multiple tracks for ISRC conflicts in parallel
func CheckISRCExistsParallel(outputDir string, tracks []ISRCMetadata) map[string]string {
	if len(tracks) == 0 {
		return make(map[string]string)
	}

	// Build ISRC index once for all tracks
	index := BuildISRCIndex(outputDir)

	// Check each track's ISRC against the index
	conflicts := make(map[string]string)
	mu := sync.Mutex{}

	var wg sync.WaitGroup
	for _, track := range tracks {
		if track.ISRC == "" {
			continue
		}

		wg.Add(1)
		go func(t ISRCMetadata) {
			defer wg.Done()

			if existingPath, exists := index[t.ISRC]; exists {
				mu.Lock()
				conflicts[t.ISRC] = existingPath
				mu.Unlock()
			}
		}(track)
	}

	wg.Wait()

	return conflicts
}

// BuildISRCIndex builds an index of ISRC codes to file paths for FLAC files in a directory.
// This is exported for use by the download orchestrator's ISRC caching system.
func BuildISRCIndex(outputDir string) map[string]string {
	index := make(map[string]string)

	// Walk through all FLAC files in the directory
	filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process FLAC files
		if !strings.HasSuffix(strings.ToLower(path), ".flac") {
			return nil
		}

		// Read ISRC from the file
		isrc, err := ReadISRCFromFile(path)
		if err != nil || isrc == "" {
			return nil
		}

		// Add to index
		index[isrc] = path

		return nil
	})

	return index
}
