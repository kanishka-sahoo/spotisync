package download

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"spotisync/internal/db/models"
)

func TestNewOrchestrator(t *testing.T) {
	cfg := OrchestratorConfig{
		TidalClientID:     "test-tidal-id",
		TidalClientSecret: "test-tidal-secret",
		QobuzAppID:        "test-qobuz-id",
		QobuzSecret:       "test-qobuz-secret",
		MusicRoot:         "/tmp/music",
		TempDir:           "/tmp/temp",
	}

	o := NewOrchestrator(cfg)

	if o == nil {
		t.Fatal("NewOrchestrator returned nil")
	}

	if !o.HasTidal() {
		t.Error("Expected Tidal client to be configured")
	}

	if !o.HasQobuz() {
		t.Error("Expected Qobuz client to be configured")
	}

	if !o.IsConfigured() {
		t.Error("Expected orchestrator to be configured")
	}
}

func TestNewOrchestrator_ThirdPartyDefault(t *testing.T) {
	// When no credentials are provided but UseThirdPartyAPIs is not explicitly disabled,
	// third-party APIs are enabled by default and clients are created
	cfg := OrchestratorConfig{
		MusicRoot: "/tmp/music",
		TempDir:   "/tmp/temp",
	}

	o := NewOrchestrator(cfg)

	if o == nil {
		t.Fatal("NewOrchestrator returned nil")
	}

	// With third-party APIs enabled by default, clients should be created
	if !o.HasTidal() {
		t.Error("Expected Tidal client to be configured (third-party APIs enabled by default)")
	}

	if !o.HasQobuz() {
		t.Error("Expected Qobuz client to be configured (third-party APIs enabled by default)")
	}

	if !o.IsConfigured() {
		t.Error("Expected orchestrator to be configured (third-party APIs enabled by default)")
	}
}

func TestNewOrchestrator_OfficialAPIsNoCredentials(t *testing.T) {
	// When UseThirdPartyAPIs is explicitly set to false but no credentials are provided,
	// third-party APIs are still used as a fallback (since official APIs can't work without credentials)
	// The condition to use official APIs requires BOTH UseThirdPartyAPIs=false AND credentials present
	cfg := OrchestratorConfig{
		MusicRoot:         "/tmp/music",
		TempDir:           "/tmp/temp",
		UseThirdPartyAPIs: false,
	}

	o := NewOrchestrator(cfg)

	if o == nil {
		t.Fatal("NewOrchestrator returned nil")
	}

	// Without credentials, third-party APIs are still used as fallback even if UseThirdPartyAPIs is false
	// This is because official APIs require credentials to function
	if !o.HasTidal() {
		t.Error("Expected Tidal client to be configured (third-party fallback when no credentials)")
	}

	if !o.HasQobuz() {
		t.Error("Expected Qobuz client to be configured (third-party fallback when no credentials)")
	}

	if !o.IsConfigured() {
		t.Error("Expected orchestrator to be configured (third-party fallback)")
	}
}

func TestNewOrchestrator_OfficialAPIsWithCredentials(t *testing.T) {
	// When UseThirdPartyAPIs is explicitly set to false AND credentials are provided,
	// official APIs are used instead of third-party APIs
	cfg := OrchestratorConfig{
		TidalClientID:     "test-tidal-id",
		TidalClientSecret: "test-tidal-secret",
		QobuzAppID:        "test-qobuz-id",
		QobuzSecret:       "test-qobuz-secret",
		MusicRoot:         "/tmp/music",
		TempDir:           "/tmp/temp",
		UseThirdPartyAPIs: false,
	}

	o := NewOrchestrator(cfg)

	if o == nil {
		t.Fatal("NewOrchestrator returned nil")
	}

	// With credentials and UseThirdPartyAPIs=false, official APIs are used
	if !o.HasTidal() {
		t.Error("Expected Tidal client to be configured (official APIs with credentials)")
	}

	if !o.HasQobuz() {
		t.Error("Expected Qobuz client to be configured (official APIs with credentials)")
	}

	if !o.IsConfigured() {
		t.Error("Expected orchestrator to be configured")
	}
}

func TestBuildFilename(t *testing.T) {
	cfg := OrchestratorConfig{
		MusicRoot: "/tmp/music",
		TempDir:   "/tmp/temp",
	}
	o := NewOrchestrator(cfg)

	tests := []struct {
		name     string
		job      *models.Job
		expected string
	}{
		{
			name: "basic track",
			job: &models.Job{
				TrackName:   "Speak to Me",
				ArtistName:  "Pink Floyd",
				DiscNumber:  1,
				TrackNumber: 1,
			},
			expected: "01 Speak-to-Me - Pink-Floyd",
		},
		{
			name: "multi-disc album",
			job: &models.Job{
				TrackName:   "The Dark Side",
				ArtistName:  "Pink Floyd",
				DiscNumber:  2,
				TrackNumber: 5,
			},
			expected: "05 The-Dark-Side - Pink-Floyd",
		},
		{
			name: "track with special characters",
			job: &models.Job{
				TrackName:   "Money / Time",
				ArtistName:  "Pink Floyd",
				DiscNumber:  1,
				TrackNumber: 3,
			},
			expected: "03 Money-Time - Pink-Floyd",
		},
		{
			name: "zero disc number defaults to 1",
			job: &models.Job{
				TrackName:   "Test Track",
				ArtistName:  "Test Artist",
				DiscNumber:  0,
				TrackNumber: 2,
			},
			expected: "02 Test-Track - Test-Artist",
		},
		{
			name: "uses first artist from Artists slice",
			job: &models.Job{
				TrackName:   "Song Name",
				ArtistName:  "Fallback Artist",
				Artists:     []string{"Primary Artist", "Second Artist"},
				DiscNumber:  1,
				TrackNumber: 1,
			},
			expected: "01 Song-Name - Primary-Artist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := o.buildFilename(tt.job)
			if result != tt.expected {
				t.Errorf("buildFilename() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDownloadErrors(t *testing.T) {
	// Test NotFoundError
	err := NewNotFoundError("track not found")
	if !IsNotFoundError(err) {
		t.Error("Expected IsNotFoundError to return true for NotFoundError")
	}
	if IsSkippedError(err) {
		t.Error("Expected IsSkippedError to return false for NotFoundError")
	}

	// Test SkippedError
	err = NewSkippedError("file already exists")
	if !IsSkippedError(err) {
		t.Error("Expected IsSkippedError to return true for SkippedError")
	}
	if IsNotFoundError(err) {
		t.Error("Expected IsNotFoundError to return false for SkippedError")
	}

	// Test DownloadError
	err = NewDownloadError("download failed", nil)
	if IsNotFoundError(err) {
		t.Error("Expected IsNotFoundError to return false for DownloadError")
	}
	if IsSkippedError(err) {
		t.Error("Expected IsSkippedError to return false for DownloadError")
	}

	// Test error wrapping
	innerErr := NewDownloadError("inner error", nil)
	outerErr := NewDownloadError("outer error", innerErr)
	if outerErr.Unwrap() != innerErr {
		t.Error("Expected Unwrap to return inner error")
	}
}

func TestMoveFile(t *testing.T) {
	cfg := OrchestratorConfig{
		MusicRoot: "/tmp/music",
		TempDir:   "/tmp/temp",
	}
	o := NewOrchestrator(cfg)

	// Create a temp file
	tempDir := t.TempDir()
	srcPath := filepath.Join(tempDir, "source.txt")
	dstPath := filepath.Join(tempDir, "subdir", "dest.txt")

	if err := os.WriteFile(srcPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Move the file
	if err := o.moveFile(srcPath, dstPath); err != nil {
		t.Fatalf("moveFile failed: %v", err)
	}

	// Verify source no longer exists
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Error("Expected source file to be removed after move")
	}

	// Verify destination exists with correct content
	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}
	if string(content) != "test content" {
		t.Errorf("Destination file content = %q, want %q", string(content), "test content")
	}
}

func TestDownloadTrack_NoOrchestrator(t *testing.T) {
	// Create orchestrator without any download sources
	// Must explicitly disable third-party APIs to have no sources configured
	cfg := OrchestratorConfig{
		MusicRoot:         "/tmp/music",
		TempDir:           "/tmp/temp",
		UseThirdPartyAPIs: false,
	}
	o := NewOrchestrator(cfg)

	job := &models.Job{
		ID:          "test-job-1",
		TrackName:   "Test Track",
		ArtistName:  "Test Artist",
		AlbumName:   "Test Album",
		AlbumArtist: "Test Album Artist",
		ISRC:        "USTEST1234567",
		DiscNumber:  1,
		TrackNumber: 1,
	}

	ctx := context.Background()
	result, err := o.DownloadTrack(ctx, job, nil)

	// Without configured clients, download should fail
	if err == nil {
		t.Error("Expected error when no download sources are configured")
	}

	// Result should indicate not found
	if result != nil && result.Status == models.JobStatusCompleted {
		t.Error("Expected non-completed status when no download sources are configured")
	}
}

func TestCheckExistingFile(t *testing.T) {
	cfg := OrchestratorConfig{
		MusicRoot: "/tmp/music",
		TempDir:   "/tmp/temp",
	}
	o := NewOrchestrator(cfg)

	// Test non-existent file
	exists, isrc := o.checkExistingFile("/nonexistent/path.flac", "USTEST1234567")
	if exists {
		t.Error("Expected non-existent file to not exist")
	}
	if isrc != "" {
		t.Error("Expected empty ISRC for non-existent file")
	}
}

func TestCheckISRCInLibrary(t *testing.T) {
	// Create orchestrator with temp directory
	cfg := OrchestratorConfig{
		MusicRoot: t.TempDir(),
		TempDir:   t.TempDir(),
	}
	orchestrator := NewOrchestrator(cfg)

	// Test: Returns false when index not built
	path, found := orchestrator.CheckISRCInLibrary("USRC12345678")
	if found {
		t.Error("Expected false when index not built")
	}
	if path != "" {
		t.Error("Expected empty path when index not built")
	}

	// Manually add to index to test lookup
	orchestrator.AddToISRCIndex("USRC12345678", "/music/test.flac")

	// Mark index as built (need to set the flag)
	orchestrator.isrcIndexMu.Lock()
	orchestrator.isrcIndexBuilt = true
	orchestrator.isrcIndexMu.Unlock()

	// Test: Returns true for existing ISRC
	path, found = orchestrator.CheckISRCInLibrary("USRC12345678")
	if !found {
		t.Error("Expected true for existing ISRC")
	}
	if path != "/music/test.flac" {
		t.Errorf("Expected /music/test.flac, got %s", path)
	}

	// Test: Returns false for non-existent ISRC
	_, found = orchestrator.CheckISRCInLibrary("GBXXX9999999")
	if found {
		t.Error("Expected false for non-existent ISRC")
	}

	// Test: Returns false for empty ISRC
	_, found = orchestrator.CheckISRCInLibrary("")
	if found {
		t.Error("Expected false for empty ISRC")
	}
}

func TestAddToISRCIndex(t *testing.T) {
	cfg := OrchestratorConfig{
		MusicRoot: t.TempDir(),
		TempDir:   t.TempDir(),
	}
	orchestrator := NewOrchestrator(cfg)

	// Test: Add new entry
	orchestrator.AddToISRCIndex("USRC12345678", "/music/test.flac")

	// Verify it was added
	orchestrator.isrcIndexMu.RLock()
	path, exists := orchestrator.isrcIndex["USRC12345678"]
	orchestrator.isrcIndexMu.RUnlock()

	if !exists || path != "/music/test.flac" {
		t.Error("Expected entry to be added")
	}

	// Test: Empty ISRC should not be added
	orchestrator.AddToISRCIndex("", "/music/empty.flac")

	orchestrator.isrcIndexMu.RLock()
	_, exists = orchestrator.isrcIndex[""]
	orchestrator.isrcIndexMu.RUnlock()

	if exists {
		t.Error("Empty ISRC should not be added to index")
	}

	// Test: Can update existing entry
	orchestrator.AddToISRCIndex("USRC12345678", "/music/updated.flac")

	orchestrator.isrcIndexMu.RLock()
	path, exists = orchestrator.isrcIndex["USRC12345678"]
	orchestrator.isrcIndexMu.RUnlock()

	if !exists || path != "/music/updated.flac" {
		t.Errorf("Expected path to be updated to /music/updated.flac, got %s", path)
	}
}

func TestAddToISRCIndex_NilMap(t *testing.T) {
	cfg := OrchestratorConfig{
		MusicRoot: t.TempDir(),
		TempDir:   t.TempDir(),
	}
	orchestrator := NewOrchestrator(cfg)

	// Forcibly set index to nil to test initialization
	orchestrator.isrcIndexMu.Lock()
	orchestrator.isrcIndex = nil
	orchestrator.isrcIndexMu.Unlock()

	// Should not panic, should initialize map
	orchestrator.AddToISRCIndex("USRC99999999", "/music/new.flac")

	orchestrator.isrcIndexMu.RLock()
	path, exists := orchestrator.isrcIndex["USRC99999999"]
	orchestrator.isrcIndexMu.RUnlock()

	if !exists || path != "/music/new.flac" {
		t.Error("Expected entry to be added after nil map initialization")
	}
}

func TestBuildISRCIndex(t *testing.T) {
	// Create a temp directory to use as music root
	musicRoot := t.TempDir()

	cfg := OrchestratorConfig{
		MusicRoot: musicRoot,
		TempDir:   t.TempDir(),
	}
	orchestrator := NewOrchestrator(cfg)

	// Verify index is not built initially
	orchestrator.isrcIndexMu.RLock()
	initiallyBuilt := orchestrator.isrcIndexBuilt
	orchestrator.isrcIndexMu.RUnlock()

	if initiallyBuilt {
		t.Error("Expected isrcIndexBuilt to be false initially")
	}

	// Build the index (empty directory, so index should be empty)
	orchestrator.BuildISRCIndex()

	// Verify index is now marked as built
	orchestrator.isrcIndexMu.RLock()
	nowBuilt := orchestrator.isrcIndexBuilt
	indexLen := len(orchestrator.isrcIndex)
	orchestrator.isrcIndexMu.RUnlock()

	if !nowBuilt {
		t.Error("Expected isrcIndexBuilt to be true after BuildISRCIndex")
	}

	// Empty directory should result in empty index
	if indexLen != 0 {
		t.Errorf("Expected empty index for empty directory, got %d entries", indexLen)
	}

	// After building, CheckISRCInLibrary should work (return false for non-existent)
	_, found := orchestrator.CheckISRCInLibrary("NONEXISTENT123")
	if found {
		t.Error("Expected false for non-existent ISRC after building index")
	}
}
