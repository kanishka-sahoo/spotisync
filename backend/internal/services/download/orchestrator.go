package download

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"spotisync/internal/db/models"
	"spotisync/internal/services/cover"
	"spotisync/internal/services/lyrics"
	"spotisync/internal/services/metadata"
	"spotisync/internal/services/qobuz"
	"spotisync/internal/services/tidal"
	"spotisync/internal/utils"
)

// Orchestrator coordinates the download workflow for a track
type Orchestrator struct {
	tidalClient       *tidal.Client
	qobuzClient       *qobuz.Client
	coverFetcher      *cover.Fetcher
	lyricsFetcher     *lyrics.LyricsFetcher
	musicRoot         string
	tempDir           string
	useThirdPartyAPIs bool
	isrcIndex         map[string]string // ISRC -> file path cache for duplicate detection
	isrcIndexMu       sync.RWMutex      // Mutex for thread-safe access to ISRC index
	isrcIndexBuilt    bool              // Whether the ISRC index has been built
}

// OrchestratorConfig holds the configuration for creating an Orchestrator
type OrchestratorConfig struct {
	TidalClientID     string
	TidalClientSecret string
	QobuzAppID        string
	QobuzSecret       string
	MusicRoot         string
	TempDir           string
	UseThirdPartyAPIs bool // When true, use third-party APIs instead of official APIs (default: true)
}

// NewOrchestrator creates a new download orchestrator
func NewOrchestrator(cfg OrchestratorConfig) *Orchestrator {
	// Default to using third-party APIs
	useThirdParty := true
	if !cfg.UseThirdPartyAPIs && cfg.TidalClientID != "" && cfg.QobuzAppID != "" {
		// Only use official APIs if explicitly disabled AND credentials are provided
		useThirdParty = false
	}

	// Create Tidal client
	// If using third-party APIs, create client with empty credentials (they won't be used)
	// If using official APIs, only create client if credentials are provided
	var tidalClient *tidal.Client
	if useThirdParty {
		// Create client for third-party API access (credentials not needed)
		tidalClient = tidal.NewClient("", "")
	} else if cfg.TidalClientID != "" && cfg.TidalClientSecret != "" {
		tidalClient = tidal.NewClient(cfg.TidalClientID, cfg.TidalClientSecret)
	}

	// Create Qobuz client
	// Same logic as Tidal
	var qobuzClient *qobuz.Client
	if useThirdParty {
		// Create client for third-party API access (credentials not needed)
		qobuzClient = qobuz.NewClient("", "")
	} else if cfg.QobuzAppID != "" && cfg.QobuzSecret != "" {
		qobuzClient = qobuz.NewClient(cfg.QobuzAppID, cfg.QobuzSecret)
	}

	// Create cover fetcher with default config
	coverFetcher := cover.NewFetcher(cover.CoverArtConfig{})

	// Create lyrics fetcher with default config
	lyricsFetcher := lyrics.NewLyricsFetcher(lyrics.LyricsConfig{})

	return &Orchestrator{
		tidalClient:       tidalClient,
		qobuzClient:       qobuzClient,
		coverFetcher:      coverFetcher,
		lyricsFetcher:     lyricsFetcher,
		musicRoot:         cfg.MusicRoot,
		tempDir:           cfg.TempDir,
		useThirdPartyAPIs: useThirdParty,
		isrcIndex:         make(map[string]string),
	}
}

// ProgressCallback is called with download progress updates
type ProgressCallback func(progress float64, status string)

// DownloadTrack performs the complete download workflow for a job
func (o *Orchestrator) DownloadTrack(ctx context.Context, job *models.Job, progressCallback ProgressCallback) (*DownloadResult, error) {
	// Send initial progress
	if progressCallback != nil {
		progressCallback(0, "Starting download")
	}

	// Step 1: Build the output directory path
	// Use first album artist, or first track artist as fallback
	albumArtist := job.AlbumArtist
	if albumArtist == "" {
		if len(job.AlbumArtists) > 0 {
			albumArtist = job.AlbumArtists[0]
		} else if len(job.Artists) > 0 {
			albumArtist = job.Artists[0]
		} else {
			albumArtist = job.ArtistName
		}
	}

	sanitizedArtist := utils.SanitizePath(albumArtist)
	sanitizedAlbum := utils.SanitizePath(job.AlbumName)
	outputDir := filepath.Join(o.musicRoot, sanitizedArtist, sanitizedAlbum)

	// Build the final filename
	finalFilename := o.buildFilename(job)
	finalPath := filepath.Join(outputDir, finalFilename+".flac")

	// Step 2a: Check if track already exists ANYWHERE in the library with same ISRC
	if existingPath, found := o.CheckISRCInLibrary(job.ISRC); found {
		log.Printf("Track already exists in library with ISRC %s: %s", job.ISRC, existingPath)
		return &DownloadResult{
			LocalPath:     existingPath,
			Status:        models.JobStatusSkipped,
			Message:       fmt.Sprintf("Track already exists in library: %s", existingPath),
			SourceService: models.SourceTidal,
			SongStatus:    "completed",
			LyricsStatus:  "completed",
			CoverStatus:   "completed",
		}, NewSkippedError("track already exists in library")
	}

	// Step 2b: Check if file already exists at the expected path with same ISRC
	if exists, existingISRC := o.checkExistingFile(finalPath, job.ISRC); exists {
		if existingISRC == job.ISRC && job.ISRC != "" {
			log.Printf("Track already exists with same ISRC: %s", finalPath)
			return &DownloadResult{
				LocalPath:     finalPath,
				Status:        models.JobStatusSkipped,
				Message:       "Track already exists with same ISRC",
				SourceService: models.SourceTidal, // Default, will be overwritten if we re-download
				SongStatus:    "completed",
				LyricsStatus:  "completed",
				CoverStatus:   "completed",
			}, NewSkippedError("track already exists with same ISRC")
		}
	}

	// Step 3: Try to find and download the track
	if progressCallback != nil {
		progressCallback(5, "Searching for track")
	}

	var downloadResult *DownloadResult
	var downloadErr error

	// Check if any download source is configured
	if o.tidalClient == nil && o.qobuzClient == nil {
		return &DownloadResult{
			Status:     models.JobStatusFailed,
			Message:    "No download sources configured (Tidal/Qobuz credentials missing)",
			SongStatus: "failed",
		}, NewDownloadError("no download sources configured", nil)
	}

	// Try Tidal first (primary source)
	if o.tidalClient != nil {
		downloadResult, downloadErr = o.downloadFromTidal(ctx, job, progressCallback)
	}

	// If Tidal failed or wasn't configured, try Qobuz
	if (downloadErr != nil || o.tidalClient == nil) && o.qobuzClient != nil {
		if downloadErr != nil {
			log.Printf("Tidal download failed, trying Qobuz: %v", downloadErr)
		}
		if progressCallback != nil {
			progressCallback(10, "Trying Qobuz")
		}
		downloadResult, downloadErr = o.downloadFromQobuz(ctx, job, progressCallback)
	}

	// If both failed, return error
	if downloadErr != nil {
		if IsNotFoundError(downloadErr) {
			return &DownloadResult{
				Status:     models.JobStatusNotFound,
				Message:    downloadErr.Error(),
				SongStatus: "not_found",
			}, downloadErr
		}
		return &DownloadResult{
			Status:     models.JobStatusFailed,
			Message:    downloadErr.Error(),
			SongStatus: "failed",
		}, downloadErr
	}

	// Step 4: Create output directory
	if progressCallback != nil {
		progressCallback(60, "Creating output directory")
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, NewFilesystemError("failed to create output directory", err)
	}

	// Step 5: Move file to final location
	if progressCallback != nil {
		progressCallback(65, "Moving file to library")
	}

	if err := o.moveFile(downloadResult.LocalPath, finalPath); err != nil {
		return nil, NewFilesystemError("failed to move file to library", err)
	}
	downloadResult.LocalPath = finalPath
	downloadResult.SongStatus = "completed"

	// Step 6: Handle cover art (with deduplication)
	if progressCallback != nil {
		progressCallback(70, "Fetching cover art")
	}

	coverPath := o.handleCoverArt(ctx, job, outputDir)
	if coverPath != "" {
		downloadResult.CoverPath = coverPath
		downloadResult.CoverStatus = "completed"
	} else {
		downloadResult.CoverStatus = "not_found"
	}

	// Step 7: Embed metadata into FLAC
	if progressCallback != nil {
		progressCallback(80, "Embedding metadata")
	}

	if err := o.embedMetadata(job, finalPath, coverPath); err != nil {
		log.Printf("Warning: failed to embed metadata: %v", err)
		// Don't fail the download for metadata errors
	}

	// Add to ISRC index for future duplicate detection
	o.AddToISRCIndex(job.ISRC, finalPath)

	// Step 8: Fetch and save lyrics
	if progressCallback != nil {
		progressCallback(90, "Fetching lyrics")
	}

	lyricsPath := o.handleLyrics(job, outputDir, finalFilename, finalPath)
	if lyricsPath != "" {
		downloadResult.LyricsPath = lyricsPath
		downloadResult.LyricsStatus = "completed"
	} else {
		downloadResult.LyricsStatus = "not_found"
	}

	// Step 9: Get final file size
	if fi, err := os.Stat(finalPath); err == nil {
		downloadResult.FileSize = fi.Size()
	}

	// Mark as completed
	downloadResult.Status = models.JobStatusCompleted
	downloadResult.Message = "Download completed successfully"

	if progressCallback != nil {
		progressCallback(100, "Completed")
	}

	return downloadResult, nil
}

// downloadFromTidal attempts to download the track from Tidal
func (o *Orchestrator) downloadFromTidal(ctx context.Context, job *models.Job, progressCallback ProgressCallback) (*DownloadResult, error) {
	// Ensure temp directory exists
	if err := os.MkdirAll(o.tempDir, 0755); err != nil {
		return nil, NewFilesystemError("failed to create temp directory", err)
	}

	var result *tidal.DownloadResult

	// Try ISRC first
	if job.ISRC != "" {
		if progressCallback != nil {
			progressCallback(10, "Matching by ISRC on Tidal")
		}

		cfg := tidal.DownloadConfig{
			OutputDir:          o.tempDir,
			Quality:            tidal.QualityLossless,
			IncludeTrackNumber: false,
		}

		if o.useThirdPartyAPIs {
			result = o.tidalClient.DownloadFromISRCWithThirdParty(ctx, job.ISRC, cfg)
		} else {
			result = o.tidalClient.DownloadFromISRC(ctx, job.ISRC, cfg)
		}
	}

	// If ISRC failed, try metadata matching
	if result == nil || result.Error != nil {
		if progressCallback != nil {
			progressCallback(15, "Matching by metadata on Tidal")
		}

		track, err := o.tidalClient.MatchByMetadata(ctx, job.TrackName, job.ArtistName, job.ISRC, job.DurationMs)
		if err != nil {
			return nil, NewNotFoundError(fmt.Sprintf("track not found on Tidal: %v", err))
		}

		if progressCallback != nil {
			progressCallback(25, "Downloading from Tidal")
		}

		cfg := tidal.DownloadConfig{
			OutputDir:          o.tempDir,
			Quality:            tidal.QualityLossless,
			IncludeTrackNumber: false,
		}

		if o.useThirdPartyAPIs {
			result = o.tidalClient.DownloadTrackFromThirdParty(ctx, track.ID, cfg)
		} else {
			result = o.tidalClient.DownloadTrack(ctx, track.ID, cfg)
		}
	}

	if result.Error != nil {
		return nil, NewDownloadError("Tidal download failed", result.Error)
	}

	if progressCallback != nil {
		progressCallback(55, "Download complete from Tidal")
	}

	return &DownloadResult{
		LocalPath:     result.Path,
		FileSize:      result.FileSize,
		SourceService: models.SourceTidal,
		SourceID:      result.SourceID,
	}, nil
}

// downloadFromQobuz attempts to download the track from Qobuz
func (o *Orchestrator) downloadFromQobuz(ctx context.Context, job *models.Job, progressCallback ProgressCallback) (*DownloadResult, error) {
	// Ensure temp directory exists
	if err := os.MkdirAll(o.tempDir, 0755); err != nil {
		return nil, NewFilesystemError("failed to create temp directory", err)
	}

	var result *qobuz.DownloadResult

	// Try ISRC first
	if job.ISRC != "" {
		if progressCallback != nil {
			progressCallback(20, "Matching by ISRC on Qobuz")
		}

		cfg := qobuz.DownloadConfig{
			OutputDir:          o.tempDir,
			Quality:            qobuz.QualityFLAC24,
			IncludeTrackNumber: false,
		}

		if o.useThirdPartyAPIs {
			result = o.qobuzClient.DownloadFromISRCWithThirdParty(ctx, job.ISRC, cfg)
		} else {
			result = o.qobuzClient.DownloadFromISRC(ctx, job.ISRC, cfg)
		}
	}

	// If ISRC failed, try metadata matching
	if result == nil || result.Error != nil {
		if progressCallback != nil {
			progressCallback(25, "Matching by metadata on Qobuz")
		}

		track, err := o.qobuzClient.MatchByMetadata(ctx, job.TrackName, job.ArtistName, job.ISRC, job.DurationMs)
		if err != nil {
			return nil, NewNotFoundError(fmt.Sprintf("track not found on Qobuz: %v", err))
		}

		if progressCallback != nil {
			progressCallback(35, "Downloading from Qobuz")
		}

		cfg := qobuz.DownloadConfig{
			OutputDir:          o.tempDir,
			Quality:            qobuz.QualityFLAC24,
			IncludeTrackNumber: false,
		}

		if o.useThirdPartyAPIs {
			result = o.qobuzClient.DownloadTrackFromThirdParty(ctx, track.ID, cfg)
		} else {
			result = o.qobuzClient.DownloadTrack(ctx, track.ID, cfg)
		}
	}

	if result.Error != nil {
		return nil, NewDownloadError("Qobuz download failed", result.Error)
	}

	if progressCallback != nil {
		progressCallback(55, "Download complete from Qobuz")
	}

	return &DownloadResult{
		LocalPath:     result.Path,
		FileSize:      result.FileSize,
		SourceService: models.SourceQobuz,
		SourceID:      result.SourceID,
	}, nil
}

// buildFilename constructs the filename for the track
// Format: {track} {title} - {author}
func (o *Orchestrator) buildFilename(job *models.Job) string {
	trackNum := job.TrackNumber
	if trackNum <= 0 {
		trackNum = 1
	}

	// Get the primary artist (first in list, or ArtistName as fallback)
	primaryArtist := job.ArtistName
	if len(job.Artists) > 0 {
		primaryArtist = job.Artists[0]
	}

	sanitizedTitle := utils.SanitizePath(job.TrackName)
	sanitizedArtist := utils.SanitizePath(primaryArtist)

	// Format: 01 Title - Artist
	return fmt.Sprintf("%02d %s - %s", trackNum, sanitizedTitle, sanitizedArtist)
}

// checkExistingFile checks if a file exists and returns its ISRC if it's a FLAC
func (o *Orchestrator) checkExistingFile(path string, expectedISRC string) (exists bool, isrc string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, ""
	}

	// Try to read ISRC from existing FLAC file
	existingISRC, err := metadata.ReadISRCFromFile(path)
	if err != nil {
		return true, ""
	}

	return true, existingISRC
}

// moveFile moves a file from src to dst
func (o *Orchestrator) moveFile(src, dst string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	// Try rename first (fastest if on same filesystem)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Fall back to copy + delete
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	if err := os.WriteFile(dst, input, 0644); err != nil {
		return err
	}

	return os.Remove(src)
}

// handleCoverArt fetches and saves cover art (with deduplication per album)
func (o *Orchestrator) handleCoverArt(ctx context.Context, job *models.Job, outputDir string) string {
	// Get album artist for cache key
	albumArtist := job.AlbumArtist
	if albumArtist == "" {
		if len(job.AlbumArtists) > 0 {
			albumArtist = job.AlbumArtists[0]
		} else if len(job.Artists) > 0 {
			albumArtist = job.Artists[0]
		} else {
			albumArtist = job.ArtistName
		}
	}

	// Check if cover already exists for this album
	existingPath, exists := o.coverFetcher.CheckAlbumCoverExists(outputDir, albumArtist, job.AlbumName)
	if exists {
		log.Printf("Using existing cover art: %s", existingPath)
		return existingPath
	}

	// Use Spotify cover URL if available (preferred)
	if job.CoverArtURL != "" {
		coverPath := o.downloadSpotifyCover(job.CoverArtURL, outputDir)
		if coverPath != "" {
			return coverPath
		}
	}

	// Fall back to MusicBrainz
	result := o.coverFetcher.FetchFromMusicBrainz(albumArtist, job.AlbumName, "")
	if result.Error != nil {
		log.Printf("Failed to fetch cover from MusicBrainz: %v", result.Error)
		return ""
	}

	// Save the cover as cover.jpg in the album directory
	coverPath := filepath.Join(outputDir, "cover.jpg")
	if err := os.WriteFile(coverPath, result.Data, 0644); err != nil {
		log.Printf("Failed to save cover art: %v", err)
		return ""
	}

	log.Printf("Saved cover art: %s", coverPath)
	return coverPath
}

// downloadSpotifyCover downloads cover art from Spotify URL with max resolution upgrade
func (o *Orchestrator) downloadSpotifyCover(coverURL, outputDir string) string {
	// Spotify image size codes
	const spotifySize640 = "ab67616d0000b273"
	const spotifySizeMax = "ab67616d000082c1"

	// Try to upgrade to max resolution using HEAD request first
	downloadURL := coverURL
	if strings.Contains(coverURL, spotifySize640) {
		maxURL := strings.Replace(coverURL, spotifySize640, spotifySizeMax, 1)
		// Check if max resolution URL is available
		headResp, err := http.Head(maxURL)
		if err == nil && headResp.StatusCode == http.StatusOK {
			downloadURL = maxURL
			log.Printf("Using max resolution Spotify cover: %s", maxURL)
		}
		if headResp != nil {
			headResp.Body.Close()
		}
	}

	// Download the image
	resp, err := http.Get(downloadURL)
	if err != nil {
		log.Printf("Failed to download cover from Spotify: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to download cover from Spotify: HTTP %d", resp.StatusCode)
		return ""
	}

	// Read the image data
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read cover data: %v", err)
		return ""
	}

	// Save as cover.jpg
	coverPath := filepath.Join(outputDir, "cover.jpg")
	if err := os.WriteFile(coverPath, data, 0644); err != nil {
		log.Printf("Failed to save cover art: %v", err)
		return ""
	}

	log.Printf("Saved Spotify cover art: %s", coverPath)
	return coverPath
}

// embedMetadata embeds metadata into the FLAC file
func (o *Orchestrator) embedMetadata(job *models.Job, flacPath, coverPath string) error {
	// Use full artist list for metadata, joined with "; " (semicolon + space)
	artistStr := job.ArtistName
	if len(job.Artists) > 0 {
		artistStr = strings.Join(job.Artists, "; ")
	}

	// Use full album artist list for metadata
	albumArtistStr := job.AlbumArtist
	if len(job.AlbumArtists) > 0 {
		albumArtistStr = strings.Join(job.AlbumArtists, "; ")
	} else if albumArtistStr == "" {
		albumArtistStr = artistStr
	}

	meta := metadata.Metadata{
		Title:       job.TrackName,
		Artist:      artistStr,
		Album:       job.AlbumName,
		AlbumArtist: albumArtistStr,
		Date:        job.ReleaseDate,
		TrackNumber: strconv.Itoa(job.TrackNumber),
		TotalTracks: strconv.Itoa(job.TotalTracks),
		DiscNumber:  strconv.Itoa(job.DiscNumber),
		TotalDiscs:  strconv.Itoa(job.TotalDiscs),
		ISRC:        job.ISRC,
		Genre:       job.Genre,
		Copyright:   job.Copyright,
		Label:       job.Label,
		Explicit:    job.Explicit,
	}

	// If Date is empty but we have ReleaseYear, use that
	if meta.Date == "" && job.ReleaseYear > 0 {
		meta.Date = strconv.Itoa(job.ReleaseYear)
	}

	return metadata.EmbedMetadata(flacPath, meta, coverPath)
}

// handleLyrics fetches lyrics and saves as .lrc file, also embeds into FLAC
func (o *Orchestrator) handleLyrics(job *models.Job, outputDir, baseFilename string, flacPath string) string {
	// Fetch lyrics from LRCLIB
	result := o.lyricsFetcher.FetchLyricsWithFallback(job.ArtistName, job.TrackName)
	if result.Error != nil {
		log.Printf("Failed to fetch lyrics from LRCLIB: %v", result.Error)
		return ""
	}

	// Build lyrics content
	var lyricsContent string
	if len(result.Synced) > 0 {
		// Use synced lyrics (LRC format)
		var lines []string
		for _, line := range result.Synced {
			timestamp := lyrics.FormatDuration(line.Start)
			lines = append(lines, timestamp+line.Text)
		}
		lyricsContent = strings.Join(lines, "\n")
	} else if result.Unsynced != "" {
		// Use unsynced lyrics (plain text with estimated timestamps)
		lyricsContent = result.Unsynced
	}

	if lyricsContent == "" {
		return ""
	}

	// Embed lyrics into FLAC file
	if err := o.embedLyricsIntoFLAC(flacPath, lyricsContent); err != nil {
		log.Printf("Warning: failed to embed lyrics into FLAC: %v", err)
	}

	// Save lyrics file alongside the FLAC
	lyricsPath := filepath.Join(outputDir, baseFilename+".lrc")
	if err := os.WriteFile(lyricsPath, []byte(lyricsContent), 0644); err != nil {
		log.Printf("Failed to save lyrics: %v", err)
		return ""
	}

	log.Printf("Saved lyrics: %s", lyricsPath)
	return lyricsPath
}

// embedLyricsIntoFLAC embeds lyrics into an existing FLAC file
func (o *Orchestrator) embedLyricsIntoFLAC(flacPath, lyrics string) error {
	// Read existing metadata from the FLAC file
	existingMeta, err := metadata.ExtractMetadata(flacPath)
	if err != nil {
		return fmt.Errorf("failed to read existing metadata: %w", err)
	}

	// Build metadata struct with existing values plus lyrics
	meta := metadata.Metadata{
		Title:       existingMeta.Title,
		Artist:      existingMeta.Artist,
		Album:       existingMeta.Album,
		AlbumArtist: existingMeta.AlbumArtist,
		Date:        existingMeta.Date,
		TrackNumber: existingMeta.TrackNumber,
		DiscNumber:  existingMeta.DiscNumber,
		Genre:       existingMeta.Genre,
		Lyrics:      lyrics,
	}

	// Re-embed metadata with lyrics (cover art is already embedded, pass empty string)
	return metadata.EmbedMetadata(flacPath, meta, "")
}

// HasTidal returns true if Tidal client is configured
func (o *Orchestrator) HasTidal() bool {
	return o.tidalClient != nil
}

// HasQobuz returns true if Qobuz client is configured
func (o *Orchestrator) HasQobuz() bool {
	return o.qobuzClient != nil
}

// IsConfigured returns true if at least one download source is configured
func (o *Orchestrator) IsConfigured() bool {
	return o.HasTidal() || o.HasQobuz()
}

// BuildISRCIndex scans the music library and builds an ISRC index for duplicate detection.
// This should be called once at startup to populate the cache.
func (o *Orchestrator) BuildISRCIndex() {
	o.isrcIndexMu.Lock()
	defer o.isrcIndexMu.Unlock()

	log.Printf("Building ISRC index for %s...", o.musicRoot)
	o.isrcIndex = metadata.BuildISRCIndex(o.musicRoot)
	o.isrcIndexBuilt = true
	log.Printf("ISRC index built with %d entries", len(o.isrcIndex))
}

// CheckISRCInLibrary checks if an ISRC exists anywhere in the cached library index.
// Returns the existing file path and true if found, empty string and false otherwise.
func (o *Orchestrator) CheckISRCInLibrary(isrc string) (string, bool) {
	if isrc == "" {
		return "", false
	}

	o.isrcIndexMu.RLock()
	defer o.isrcIndexMu.RUnlock()

	if !o.isrcIndexBuilt {
		return "", false // Index not built yet, skip library-wide check
	}

	path, exists := o.isrcIndex[isrc]
	return path, exists
}

// AddToISRCIndex adds a newly downloaded track to the ISRC index.
// This keeps the cache up-to-date as new tracks are downloaded.
func (o *Orchestrator) AddToISRCIndex(isrc, path string) {
	if isrc == "" {
		return
	}

	o.isrcIndexMu.Lock()
	defer o.isrcIndexMu.Unlock()

	if o.isrcIndex == nil {
		o.isrcIndex = make(map[string]string)
	}
	o.isrcIndex[isrc] = path
}
