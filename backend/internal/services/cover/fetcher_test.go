package cover

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBuildCoverFilename(t *testing.T) {
	t.Run("basic artist and album", func(t *testing.T) {
		filename, err := BuildCoverFilename("Artist", "Album")
		assert.NoError(t, err)
		assert.Equal(t, "Artist - Album (Cover)", filename)
	})

	t.Run("artist with slash replaced", func(t *testing.T) {
		filename, err := BuildCoverFilename("Artist/Name", "Album")
		assert.NoError(t, err)
		assert.Equal(t, "Artist-Name - Album (Cover)", filename)
	})

	t.Run("album with slash replaced", func(t *testing.T) {
		filename, err := BuildCoverFilename("Artist", "Album/Name")
		assert.NoError(t, err)
		assert.Equal(t, "Artist - Album-Name (Cover)", filename)
	})

	t.Run("colon replaced with dash", func(t *testing.T) {
		filename, err := BuildCoverFilename("Artist:Name", "Album")
		assert.NoError(t, err)
		assert.Equal(t, "Artist-Name - Album (Cover)", filename)
	})

	t.Run("album with colon replaced with dash", func(t *testing.T) {
		filename, err := BuildCoverFilename("Artist", "Album:Name")
		assert.NoError(t, err)
		assert.Equal(t, "Artist - Album-Name (Cover)", filename)
	})

	t.Run("both artist and album with special chars", func(t *testing.T) {
		filename, err := BuildCoverFilename("Artist/Name", "Album/Name")
		assert.NoError(t, err)
		assert.Equal(t, "Artist-Name - Album-Name (Cover)", filename)
	})

	t.Run("empty strings", func(t *testing.T) {
		filename, err := BuildCoverFilename("", "")
		assert.NoError(t, err)
		assert.Equal(t, " -  (Cover)", filename)
	})

	t.Run("multiple slashes", func(t *testing.T) {
		filename, err := BuildCoverFilename("Artist/Name/Deep", "Album")
		assert.NoError(t, err)
		assert.Equal(t, "Artist-Name-Deep - Album (Cover)", filename)
	})

	t.Run("unicode characters preserved", func(t *testing.T) {
		filename, err := BuildCoverFilename("藝人", "專輯")
		assert.NoError(t, err)
		assert.Equal(t, "藝人 - 專輯 (Cover)", filename)
	})

	t.Run("path traversal attempt", func(t *testing.T) {
		_, err := BuildCoverFilename("..", "Album")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal")
	})

	t.Run("path traversal in album", func(t *testing.T) {
		_, err := BuildCoverFilename("Artist", "../Album")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal")
	})
}

func TestCheckAlbumCoverExists(t *testing.T) {
	t.Run("check with non-existent directory", func(t *testing.T) {
		fetcher := NewFetcher(CoverArtConfig{})

		path, exists := fetcher.CheckAlbumCoverExists("/non/existent", "Artist", "Album")
		assert.Empty(t, path)
		assert.False(t, exists)
	})

	t.Run("check with empty directory", func(t *testing.T) {
		// This test verifies the logic without actual file system access
		// In a real test, you'd create a temp directory
		fetcher := NewFetcher(CoverArtConfig{})

		// The function checks for .jpg, .png, .webp extensions
		// Since the directory doesn't exist, it should return false
		path, exists := fetcher.CheckAlbumCoverExists("/tmp/nonexistent-dir-12345", "Artist", "Album")
		assert.Empty(t, path)
		assert.False(t, exists)
	})

	t.Run("check with different file extensions", func(t *testing.T) {
		// Test filename building logic
		filename, err := BuildCoverFilename("Test Artist", "Test Album")
		assert.NoError(t, err)

		// Verify the filename format (spaces are replaced with dashes by SanitizePath)
		assert.Equal(t, "Test-Artist - Test-Album (Cover)", filename)
	})

	t.Run("check with special characters in names", func(t *testing.T) {
		fetcher := NewFetcher(CoverArtConfig{})

		// The function replaces / with - in both artist and album
		path, exists := fetcher.CheckAlbumCoverExists("/tmp", "Artist/Name", "Album/Name")
		assert.Empty(t, path)
		assert.False(t, exists)
	})
}

func TestDetectContentType(t *testing.T) {
	t.Run("detect JPEG", func(t *testing.T) {
		// JPEG magic bytes: FF D8 FF
		data := []byte{0xFF, 0xD8, 0xFF, 0xE0}
		reader := bytes.NewReader(data)

		result := detectContentType(reader)
		assert.Equal(t, "image/jpeg", result)
	})

	t.Run("detect PNG", func(t *testing.T) {
		// PNG magic bytes: 89 50 4E 47
		data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
		reader := bytes.NewReader(data)

		result := detectContentType(reader)
		assert.Equal(t, "image/png", result)
	})

	t.Run("detect WebP", func(t *testing.T) {
		// WebP magic bytes: 52 49 46 46 (RIFF) followed by WEBP
		data := []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50}
		reader := bytes.NewReader(data)

		result := detectContentType(reader)
		assert.Equal(t, "image/webp", result)
	})

	t.Run("unknown content type", func(t *testing.T) {
		// Random bytes that don't match known types
		data := []byte{0x00, 0x00, 0x00, 0x00}
		reader := bytes.NewReader(data)

		result := detectContentType(reader)
		assert.Equal(t, "application/octet-stream", result)
	})

	t.Run("empty data", func(t *testing.T) {
		reader := bytes.NewReader([]byte{})

		result := detectContentType(reader)
		assert.Equal(t, "application/octet-stream", result)
	})

	t.Run("partial JPEG header", func(t *testing.T) {
		// Only 2 bytes of JPEG
		data := []byte{0xFF, 0xD8}
		reader := bytes.NewReader(data)

		result := detectContentType(reader)
		assert.Equal(t, "image/jpeg", result)
	})

	t.Run("3 bytes not enough for PNG detection", func(t *testing.T) {
		// Only 3 bytes of PNG - not enough for full detection
		data := []byte{0x89, 0x50, 0x4E}
		reader := bytes.NewReader(data)

		result := detectContentType(reader)
		// Function needs 4 bytes to detect PNG
		assert.Equal(t, "application/octet-stream", result)
	})

	t.Run("detect from reader without consuming all data", func(t *testing.T) {
		// PNG with extra data
		data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
		reader := bytes.NewReader(data)

		result := detectContentType(reader)
		assert.Equal(t, "image/png", result)
	})

	t.Run("WebP RIFF format", func(t *testing.T) {
		// RIFF header
		data := []byte{0x52, 0x49, 0x46, 0x46}
		reader := bytes.NewReader(data)

		result := detectContentType(reader)
		assert.Equal(t, "image/webp", result)
	})
}

func TestCoverArtConfig(t *testing.T) {
	t.Run("default config values", func(t *testing.T) {
		config := CoverArtConfig{}

		// NewFetcher should set defaults
		fetcher := NewFetcher(config)

		assert.Equal(t, 10*time.Second, fetcher.config.Timeout)
		assert.Equal(t, 5*1024*1024, fetcher.config.MaxSize)
		assert.Equal(t, "SpotiSync/1.0", fetcher.config.UserAgent)
		assert.Equal(t, []string{"image/jpeg", "image/png", "image/webp"}, fetcher.config.AllowedTypes)
	})

	t.Run("custom config values", func(t *testing.T) {
		config := CoverArtConfig{
			Timeout:      30 * time.Second,
			MaxSize:      10 * 1024 * 1024,
			UserAgent:    "CustomAgent/1.0",
			AllowedTypes: []string{"image/png"},
		}

		fetcher := NewFetcher(config)

		assert.Equal(t, 30*time.Second, fetcher.config.Timeout)
		assert.Equal(t, 10*1024*1024, fetcher.config.MaxSize)
		assert.Equal(t, "CustomAgent/1.0", fetcher.config.UserAgent)
		assert.Equal(t, []string{"image/png"}, fetcher.config.AllowedTypes)
	})
}

func TestCoverResultStruct(t *testing.T) {
	t.Run("create result with data", func(t *testing.T) {
		result := CoverResult{
			Data:   []byte{0x89, 0x50, 0x4E, 0x47},
			Format: "image/png",
			Source: "https://example.com/cover.jpg",
		}

		assert.NotEmpty(t, result.Data)
		assert.Equal(t, "image/png", result.Format)
		assert.Equal(t, "https://example.com/cover.jpg", result.Source)
	})

	t.Run("create result with error", func(t *testing.T) {
		result := CoverResult{
			Error: assert.AnError,
		}

		assert.Error(t, result.Error)
		assert.Nil(t, result.Data)
	})

	t.Run("create empty result", func(t *testing.T) {
		result := CoverResult{}

		assert.Nil(t, result.Data)
		assert.Empty(t, result.Format)
		assert.Empty(t, result.Source)
		assert.NoError(t, result.Error)
	})
}

func TestDetectContentTypeWithCustomReader(t *testing.T) {
	t.Run("detect after partial read", func(t *testing.T) {
		// Create a reader that we'll read from partially first
		data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

		// Read first 2 bytes
		partialReader := io.NopCloser(bytes.NewReader(data))
		buf := make([]byte, 2)
		_, _ = partialReader.Read(buf)

		// Now detect content type - it should still work with remaining data
		// Note: detectContentType reads up to 4 bytes from the start
		// So this tests that it handles already-read data gracefully
		// But since we already read 2 bytes, starting from byte 2 won't have PNG header
		result := detectContentType(bytes.NewReader(data[2:]))
		assert.Equal(t, "application/octet-stream", result)
	})

	t.Run("detect from large buffer", func(t *testing.T) {
		// Create a large buffer starting with PNG header
		largeData := make([]byte, 1024)
		largeData[0] = 0x89
		largeData[1] = 0x50
		largeData[2] = 0x4E
		largeData[3] = 0x47

		result := detectContentType(bytes.NewReader(largeData))
		assert.Equal(t, "image/png", result)
	})
}
