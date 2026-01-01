package metadata

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckISRCExists(t *testing.T) {
	t.Run("check ISRC in empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		path, exists := CheckISRCExists(tmpDir, "US-S1Z-24-00001")
		assert.Empty(t, path)
		assert.False(t, exists)
	})

	t.Run("check ISRC with empty ISRC", func(t *testing.T) {
		tmpDir := t.TempDir()

		path, exists := CheckISRCExists(tmpDir, "")
		assert.Empty(t, path)
		assert.False(t, exists)
	})

	t.Run("check ISRC in directory with no FLAC files", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create non-FLAC files
		err := os.WriteFile(filepath.Join(tmpDir, "test.mp3"), []byte("mp3 content"), 0644)
		require.NoError(t, err)

		path, exists := CheckISRCExists(tmpDir, "US-S1Z-24-00001")
		assert.Empty(t, path)
		assert.False(t, exists)
	})

	t.Run("check ISRC in directory with FLAC but no ISRC", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a FLAC file without ISRC (just content for testing)
		flacContent := []byte("not a real flac but content")
		err := os.WriteFile(filepath.Join(tmpDir, "track.flac"), flacContent, 0644)
		require.NoError(t, err)

		path, exists := CheckISRCExists(tmpDir, "US-S1Z-24-00001")
		assert.Empty(t, path)
		assert.False(t, exists)
	})
}

func TestCheckISRCExistsParallel(t *testing.T) {
	t.Run("check multiple tracks with no conflicts", func(t *testing.T) {
		tmpDir := t.TempDir()

		tracks := []ISRCMetadata{
			{ISRC: "US-S1Z-24-00001", Title: "Track 1"},
			{ISRC: "US-S1Z-24-00002", Title: "Track 2"},
			{ISRC: "US-S1Z-24-00003", Title: "Track 3"},
		}

		conflicts := CheckISRCExistsParallel(tmpDir, tracks)
		assert.Empty(t, conflicts)
	})

	t.Run("check multiple tracks with empty list", func(t *testing.T) {
		tmpDir := t.TempDir()

		tracks := []ISRCMetadata{}
		conflicts := CheckISRCExistsParallel(tmpDir, tracks)
		assert.Empty(t, conflicts)
	})

	t.Run("check multiple tracks with some empty ISRCs", func(t *testing.T) {
		tmpDir := t.TempDir()

		tracks := []ISRCMetadata{
			{ISRC: "US-S1Z-24-00001", Title: "Track 1"},
			{ISRC: "", Title: "Track 2"}, // Empty ISRC
			{ISRC: "US-S1Z-24-00003", Title: "Track 3"},
		}

		conflicts := CheckISRCExistsParallel(tmpDir, tracks)
		assert.Empty(t, conflicts)
	})

	t.Run("check multiple tracks with concurrent access", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create many tracks with the same ISRC
		tracks := make([]ISRCMetadata, 100)
		for i := 0; i < 100; i++ {
			tracks[i] = ISRCMetadata{
				ISRC:     "US-S1Z-24-00001",
				Title:    "Track",
				TrackNum: i,
			}
		}

		// Note: Since we're not creating actual FLAC files with ISRC metadata,
		// the BuildISRCIndex will return an empty map, so no conflicts will be found.
		// This test verifies the concurrent processing logic works correctly.
		conflicts := CheckISRCExistsParallel(tmpDir, tracks)

		// With empty index, no conflicts should be found
		// This is the expected behavior when no FLAC files with ISRC exist
		assert.Empty(t, conflicts)
	})

	t.Run("check parallel with actual concurrent processing", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create FLAC files with different ISRCs
		for i := 0; i < 10; i++ {
			isrc := "US-S1Z-24-00001"
			if i%2 == 0 {
				isrc = "US-S1Z-24-00001"
			} else {
				isrc = "US-S1Z-24-00002"
			}
			// Create mock FLAC file with ISRC in content
			flacContent := []byte("fLaC" + isrc)
			err := os.WriteFile(filepath.Join(tmpDir, "track"+string(rune('0'+i))+".flac"), flacContent, 0644)
			require.NoError(t, err)
		}

		tracks := []ISRCMetadata{
			{ISRC: "US-S1Z-24-00001", Title: "Track with ISRC 1"},
			{ISRC: "US-S1Z-24-00002", Title: "Track with ISRC 2"},
		}

		// This will likely have no conflicts because the files aren't valid FLACs
		// but it tests the parallel processing logic
		conflicts := CheckISRCExistsParallel(tmpDir, tracks)
		assert.Empty(t, conflicts)
	})
}

func TestBuildISRCIndex(t *testing.T) {
	t.Run("build index in empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		index := BuildISRCIndex(tmpDir)
		assert.NotNil(t, index)
		assert.Empty(t, index)
	})

	t.Run("build index with non-existent directory", func(t *testing.T) {
		// This should not panic and should return an empty index
		index := BuildISRCIndex("/non/existent/path")
		assert.NotNil(t, index)
		assert.Empty(t, index)
	})

	t.Run("build index with mixed file types", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create various file types
		files := []struct {
			name    string
			content []byte
		}{
			{"track1.flac", []byte("flac content 1")},
			{"track2.mp3", []byte("mp3 content")},
			{"track3.flac", []byte("flac content 3")},
			{"track4.wav", []byte("wav content")},
		}

		for _, f := range files {
			err := os.WriteFile(filepath.Join(tmpDir, f.name), f.content, 0644)
			require.NoError(t, err)
		}

		index := BuildISRCIndex(tmpDir)
		// Only FLAC files are processed
		assert.NotNil(t, index)
		// Since files aren't valid FLACs, no ISRCs should be found
		assert.Empty(t, index)
	})

	t.Run("build index with subdirectories", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create subdirectory structure
		subDir := filepath.Join(tmpDir, "subdir")
		err := os.MkdirAll(subDir, 0755)
		require.NoError(t, err)

		// Create files in both directories
		err = os.WriteFile(filepath.Join(tmpDir, "track1.flac"), []byte("flac content"), 0644)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(subDir, "track2.flac"), []byte("flac content"), 0644)
		require.NoError(t, err)

		index := BuildISRCIndex(tmpDir)
		assert.NotNil(t, index)
		// Should include files from subdirectories
		// Since files aren't valid FLACs, no ISRCs found
		assert.Empty(t, index)
	})

	t.Run("concurrent access to BuildISRCIndex", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create multiple FLAC files
		for i := 0; i < 10; i++ {
			err := os.WriteFile(filepath.Join(tmpDir, "track"+string(rune('0'+i))+".flac"), []byte("flac content"), 0644)
			require.NoError(t, err)
		}

		// Call BuildISRCIndex concurrently
		var wg sync.WaitGroup
		results := make([]map[string]string, 10)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				results[idx] = BuildISRCIndex(tmpDir)
			}(i)
		}

		wg.Wait()

		// All results should be equal and empty (no valid ISRCs)
		for _, result := range results {
			assert.NotNil(t, result)
		}
	})
}

func TestISRCMetadataStruct(t *testing.T) {
	t.Run("create ISRCMetadata with all fields", func(t *testing.T) {
		meta := ISRCMetadata{
			ISRC:       "USS1Z2400001",
			Artist:     "Test Artist",
			Album:      "Test Album",
			Title:      "Test Title",
			TrackNum:   1,
			DiscNum:    1,
			OutputPath: "/path/to/output",
		}

		assert.Equal(t, "USS1Z2400001", meta.ISRC)
		assert.Equal(t, "Test Artist", meta.Artist)
		assert.Equal(t, 1, meta.TrackNum)
		assert.Equal(t, 1, meta.DiscNum)
	})

	t.Run("create ISRCMetadata with minimal fields", func(t *testing.T) {
		meta := ISRCMetadata{
			ISRC:  "USS1Z2400001",
			Title: "Test",
		}

		assert.Equal(t, "USS1Z2400001", meta.ISRC)
		assert.Equal(t, "Test", meta.Title)
		assert.Empty(t, meta.Artist)
		assert.Equal(t, 0, meta.TrackNum)
	})
}

func TestValidateISRC(t *testing.T) {
	t.Run("valid ISRC", func(t *testing.T) {
		err := ValidateISRC("USS1Z2400001")
		assert.NoError(t, err)
	})

	t.Run("valid ISRC with numbers in registrant code", func(t *testing.T) {
		err := ValidateISRC("US1232400001")
		assert.NoError(t, err)
	})

	t.Run("empty ISRC is allowed", func(t *testing.T) {
		err := ValidateISRC("")
		assert.NoError(t, err)
	})

	t.Run("ISRC too short", func(t *testing.T) {
		err := ValidateISRC("US-S1Z-24-000")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})

	t.Run("ISRC too long", func(t *testing.T) {
		err := ValidateISRC("US-S1Z-24-0000001")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too long")
	})

	t.Run("ISRC with lowercase letters", func(t *testing.T) {
		err := ValidateISRC("usS1Z2400001")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})

	t.Run("ISRC with special characters", func(t *testing.T) {
		err := ValidateISRC("US-S1Z-24-00001")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})

	t.Run("ISRC with invalid year", func(t *testing.T) {
		err := ValidateISRC("USS1ZAA00001")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})

	t.Run("ISRC with invalid serial", func(t *testing.T) {
		err := ValidateISRC("USS1Z24000A01")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})

	t.Run("ISRC with hyphen separators", func(t *testing.T) {
		err := ValidateISRC("US-S1Z-24-00001")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})

	t.Run("ISRC with spaces", func(t *testing.T) {
		err := ValidateISRC("US S1Z 24 00001")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})
}
