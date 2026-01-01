package matching

import "fmt"

// SourceService represents a music service
type SourceService string

const (
	SourceTidal SourceService = "tidal"
	SourceQobuz SourceService = "qobuz"
)

// MatchResult represents the result of a track match
type MatchResult struct {
	Found    bool
	NotFound bool
	Error    error
	Source   SourceService
	TrackID  string
	Quality  string
	ISRC     string
}

// TrackMetadata represents Spotify track metadata
type TrackMetadata struct {
	SpotifyID   string
	Name        string
	Artist      string
	Album       string
	AlbumArtist string
	ISRC        string
	DurationMs  int
	ReleaseYear int
	ReleaseDate string
	TrackNumber int
	TotalTracks int
	CoverArtURL string
}

// MatchStats contains matching statistics
type MatchStats struct {
	Total       int
	FoundTidal  int
	FoundQobuz  int
	NotFound    int
	Errors      int
	HiResCount  int
	SuccessRate float64
}

// CalculateStats calculates statistics from match results
func CalculateStats(results []*MatchResult) *MatchStats {
	stats := &MatchStats{
		Total: len(results),
	}

	for _, r := range results {
		if r.Error != nil {
			stats.Errors++
		} else if r.NotFound {
			stats.NotFound++
		} else if r.Found {
			if r.Source == SourceTidal {
				stats.FoundTidal++
			} else if r.Source == SourceQobuz {
				stats.FoundQobuz++
			}

			// Count Hi-Res matches
			if r.Quality == "Hi-Res" || r.Quality == "HI_RES_LOSSLESS" || r.Quality == "FLAC 24-bit" {
				stats.HiResCount++
			}
		}
	}

	// Calculate success rate (found / total - errors)
	foundCount := stats.FoundTidal + stats.FoundQobuz
	processedCount := foundCount + stats.NotFound
	if processedCount > 0 {
		stats.SuccessRate = float64(foundCount) / float64(processedCount) * 100
	}

	return stats
}

// FormatStats formats statistics for display
func FormatStats(stats *MatchStats) string {
	return fmt.Sprintf("%.1f%% success rate (%d/%d) - Tidal: %d, Qobuz: %d, Not found: %d, Errors: %d, Hi-Res: %d",
		stats.SuccessRate,
		stats.FoundTidal+stats.FoundQobuz,
		stats.Total,
		stats.FoundTidal,
		stats.FoundQobuz,
		stats.NotFound,
		stats.Errors,
		stats.HiResCount,
	)
}
