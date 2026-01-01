package lyrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildLyricsFilename(t *testing.T) {
	t.Run("basic artist and title", func(t *testing.T) {
		filename, err := BuildLyricsFilename("Artist", "Title", 0, 0)
		assert.NoError(t, err)
		assert.Equal(t, "Artist - Title.lrc", filename)
	})

	t.Run("with disc and track number", func(t *testing.T) {
		filename, err := BuildLyricsFilename("Artist", "Title", 1, 2)
		assert.NoError(t, err)
		assert.Equal(t, "Artist - Title (Disc 01-Track 02).lrc", filename)
	})

	t.Run("with only disc number", func(t *testing.T) {
		filename, err := BuildLyricsFilename("Artist", "Title", 1, 0)
		assert.NoError(t, err)
		assert.Equal(t, "Artist - Title (Disc 01).lrc", filename)
	})

	t.Run("with only track number", func(t *testing.T) {
		filename, err := BuildLyricsFilename("Artist", "Title", 0, 5)
		assert.NoError(t, err)
		assert.Equal(t, "Artist - Title (Track 05).lrc", filename)
	})

	t.Run("artist with slash replaced", func(t *testing.T) {
		filename, err := BuildLyricsFilename("Artist/Name", "Title", 0, 0)
		assert.NoError(t, err)
		assert.Equal(t, "Artist-Name - Title.lrc", filename)
	})

	t.Run("title with slash replaced", func(t *testing.T) {
		filename, err := BuildLyricsFilename("Artist", "Title/Subtitle", 0, 0)
		assert.NoError(t, err)
		assert.Equal(t, "Artist - Title-Subtitle.lrc", filename)
	})

	t.Run("colon replaced with dash", func(t *testing.T) {
		filename, err := BuildLyricsFilename("Artist:Name", "Title", 0, 0)
		assert.NoError(t, err)
		assert.Equal(t, "Artist-Name - Title.lrc", filename)
	})

	t.Run("multiple disc and track", func(t *testing.T) {
		filename, err := BuildLyricsFilename("Queen", "Bohemian Rhapsody", 1, 11)
		assert.NoError(t, err)
		assert.Equal(t, "Queen - Bohemian-Rhapsody (Disc 01-Track 11).lrc", filename)
	})

	t.Run("zero values", func(t *testing.T) {
		filename, err := BuildLyricsFilename("", "", 0, 0)
		assert.NoError(t, err)
		assert.Equal(t, " - .lrc", filename)
	})

	t.Run("single digit numbers", func(t *testing.T) {
		filename, err := BuildLyricsFilename("Artist", "Title", 1, 1)
		assert.NoError(t, err)
		assert.Equal(t, "Artist - Title (Disc 01-Track 01).lrc", filename)
	})

	t.Run("path traversal attempt in artist", func(t *testing.T) {
		_, err := BuildLyricsFilename("..", "Title", 0, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal")
	})

	t.Run("path traversal attempt in title", func(t *testing.T) {
		_, err := BuildLyricsFilename("Artist", "../Title", 0, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal")
	})
}

func TestParseSyncedLyrics(t *testing.T) {
	t.Run("parse basic LRC format", func(t *testing.T) {
		lrcContent := `[00:00.00]First line
[00:05.50]Second line
[00:10.25]Third line`

		lines := ParseSyncedLyrics(lrcContent)

		assert.Len(t, lines, 3)
		assert.Equal(t, int64(0), lines[0].Start)
		assert.Equal(t, "First line", lines[0].Text)
		assert.Equal(t, int64(5500), lines[1].Start)
		assert.Equal(t, "Second line", lines[1].Text)
		assert.Equal(t, int64(10250), lines[2].Start)
		assert.Equal(t, "Third line", lines[2].Text)
	})

	t.Run("parse LRC with milliseconds", func(t *testing.T) {
		lrcContent := `[01:23.45]Test line`

		lines := ParseSyncedLyrics(lrcContent)

		assert.Len(t, lines, 1)
		// 1 minute = 60000ms, 23.45 seconds = 23450ms, total = 83450ms
		assert.Equal(t, int64(83450), lines[0].Start)
		assert.Equal(t, "Test line", lines[0].Text)
	})

	t.Run("parse LRC with empty lines", func(t *testing.T) {
		lrcContent := `[00:00.00]First line

[00:05.00]Third line`

		lines := ParseSyncedLyrics(lrcContent)

		assert.Len(t, lines, 2)
	})

	t.Run("parse LRC with malformed lines", func(t *testing.T) {
		lrcContent := `Not a timestamp line
[invalid timestamp]Test
[00:00.00]Valid line
`

		lines := ParseSyncedLyrics(lrcContent)

		assert.Len(t, lines, 1)
		assert.Equal(t, "Valid line", lines[0].Text)
	})

	t.Run("parse LRC with no timestamps", func(t *testing.T) {
		lrcContent := `First line
Second line
Third line`

		lines := ParseSyncedLyrics(lrcContent)

		assert.Len(t, lines, 0)
	})

	t.Run("parse LRC with trimmed text (whitespace removed from text only)", func(t *testing.T) {
		// Note: The function requires [ to be at position 0, so leading spaces
		// before the bracket will cause the line to be skipped
		lrcContent := `[00:00.00]  First line  `

		lines := ParseSyncedLyrics(lrcContent)

		assert.Len(t, lines, 1)
		// Function trims whitespace from text
		assert.Equal(t, "First line", lines[0].Text)
	})

	t.Run("parse LRC with long duration", func(t *testing.T) {
		lrcContent := `[99:59.99]End of song`

		lines := ParseSyncedLyrics(lrcContent)

		assert.Len(t, lines, 1)
		// 99 minutes = 5940000ms, 59.99 seconds = 59990ms, total = 5999990ms
		assert.Equal(t, int64(5999990), lines[0].Start)
	})

	t.Run("parse empty content", func(t *testing.T) {
		lines := ParseSyncedLyrics("")
		assert.Len(t, lines, 0)
	})

	t.Run("parse LRC with only timestamps and text", func(t *testing.T) {
		lrcContent := `[00:00.00]First line
[00:01.00]Second line
[00:02.00]Third line`

		lines := ParseSyncedLyrics(lrcContent)

		assert.Len(t, lines, 3)
		assert.Equal(t, "First line", lines[0].Text)
		assert.Equal(t, "Second line", lines[1].Text)
		assert.Equal(t, "Third line", lines[2].Text)
	})
}

func TestFormatDuration(t *testing.T) {
	t.Run("format zero duration", func(t *testing.T) {
		result := FormatDuration(0)
		assert.Equal(t, "[00:00.00]", result)
	})

	t.Run("format seconds", func(t *testing.T) {
		result := FormatDuration(5000) // 5 seconds
		assert.Equal(t, "[00:05.00]", result)
	})

	t.Run("format minutes", func(t *testing.T) {
		result := FormatDuration(60000) // 1 minute
		assert.Equal(t, "[01:00.00]", result)
	})

	t.Run("format minutes and seconds", func(t *testing.T) {
		result := FormatDuration(125000) // 2 minutes 5 seconds
		assert.Equal(t, "[02:05.00]", result)
	})

	t.Run("format with milliseconds", func(t *testing.T) {
		result := FormatDuration(12345) // 12.345 seconds
		assert.Equal(t, "[00:12.34]", result)
	})

	t.Run("format long duration", func(t *testing.T) {
		result := FormatDuration(3600000) // 60 minutes
		assert.Equal(t, "[60:00.00]", result)
	})

	t.Run("format with hundredths", func(t *testing.T) {
		result := FormatDuration(1234) // 1.234 seconds
		assert.Equal(t, "[00:01.23]", result)
	})

	t.Run("format edge case at 1 second", func(t *testing.T) {
		result := FormatDuration(1000)
		assert.Equal(t, "[00:01.00]", result)
	})

	t.Run("format edge case at 59 seconds", func(t *testing.T) {
		result := FormatDuration(59000)
		assert.Equal(t, "[00:59.00]", result)
	})

	t.Run("format just under a minute", func(t *testing.T) {
		result := FormatDuration(59999)
		assert.Equal(t, "[00:59.99]", result)
	})
}

func TestPlainToLRC(t *testing.T) {
	t.Run("convert plain lyrics to LRC", func(t *testing.T) {
		plain := `First line
Second line
Third line`

		result := plainToLRC(plain, 9000) // 3 lines over 9 seconds = 3 seconds per line

		assert.NotEmpty(t, result)
		resultLines := ParseSyncedLyrics(result)
		assert.Len(t, resultLines, 3)
	})

	t.Run("handle empty plain lyrics", func(t *testing.T) {
		result := plainToLRC("", 1000)
		assert.Equal(t, "", result)
	})

	t.Run("handle single line", func(t *testing.T) {
		plain := "Single line"
		result := plainToLRC(plain, 5000)

		assert.NotEmpty(t, result)
		resultLines := ParseSyncedLyrics(result)
		assert.Len(t, resultLines, 1)
		assert.Equal(t, "Single line", resultLines[0].Text)
	})

	t.Run("handle empty lines in plain lyrics", func(t *testing.T) {
		plain := `Line 1

Line 3`

		result := plainToLRC(plain, 6000)

		assert.NotEmpty(t, result)
		resultLines := ParseSyncedLyrics(result)
		assert.Len(t, resultLines, 2)
	})

	t.Run("use default time when duration is zero", func(t *testing.T) {
		plain := `Line 1
Line 2`

		result := plainToLRC(plain, 0)

		assert.NotEmpty(t, result)
		resultLines := ParseSyncedLyrics(result)
		assert.Len(t, resultLines, 2)
		// Default is 3 seconds per line
		assert.Equal(t, int64(0), resultLines[0].Start)
		assert.Equal(t, int64(3000), resultLines[1].Start)
	})

	t.Run("use calculated time when duration is less than lines", func(t *testing.T) {
		plain := `Line 1
Line 2
Line 3
Line 4`

		result := plainToLRC(plain, 1000) // 1000ms / 4 lines = 250ms per line

		assert.NotEmpty(t, result)
		resultLines := ParseSyncedLyrics(result)
		assert.Len(t, resultLines, 4)
		// 1000ms / 4 lines = 250ms per line
		assert.Equal(t, int64(0), resultLines[0].Start)
		assert.Equal(t, int64(250), resultLines[1].Start)
		assert.Equal(t, int64(500), resultLines[2].Start)
		assert.Equal(t, int64(750), resultLines[3].Start)
	})

	t.Run("whitespace handling", func(t *testing.T) {
		plain := `  Line 1  
  Line 2  `

		result := plainToLRC(plain, 6000)

		assert.NotEmpty(t, result)
		resultLines := ParseSyncedLyrics(result)
		assert.Len(t, resultLines, 2)
		assert.Equal(t, "Line 1", resultLines[0].Text)
		assert.Equal(t, "Line 2", resultLines[1].Text)
	})

	t.Run("timestamp format in output", func(t *testing.T) {
		plain := "Test line"

		result := plainToLRC(plain, 10000)

		// Should start with [00:00.00]
		assert.Contains(t, result, "[00:00.00]")
	})
}

func TestLyricsLineStruct(t *testing.T) {
	t.Run("create Line with all fields", func(t *testing.T) {
		line := Line{
			Start: 12345,
			Text:  "Test lyrics",
		}

		assert.Equal(t, int64(12345), line.Start)
		assert.Equal(t, "Test lyrics", line.Text)
	})

	t.Run("create Line with zero values", func(t *testing.T) {
		line := Line{}

		assert.Equal(t, int64(0), line.Start)
		assert.Empty(t, line.Text)
	})
}
