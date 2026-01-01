package metadata

import (
	"os"

	"github.com/go-flac/flacpicture"
	"github.com/go-flac/flacvorbis"
	"github.com/go-flac/go-flac"
	"io"
)

// FLACMetadata represents metadata extracted from a FLAC file
type FLACMetadata struct {
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	Date        string
	Genre       string
	TrackNumber string
	DiscNumber  string
	Comment     string
	CoverArt    []byte
	CoverMime   string
}

// ExtractMetadata extracts metadata from a FLAC file
func ExtractMetadata(filePath string) (*FLACMetadata, error) {
	flacFile, err := flac.ParseFile(filePath)
	if err != nil {
		return nil, err
	}

	meta := &FLACMetadata{}

	// Parse vorbis comments
	for _, metaBlock := range flacFile.Meta {
		if metaBlock.Type == flac.VorbisComment {
			vc, err := flacvorbis.ParseFromMetaDataBlock(*metaBlock)
			if err != nil {
				continue
			}

			if values, err := vc.Get("TITLE"); err == nil && len(values) > 0 {
				meta.Title = values[0]
			}
			if values, err := vc.Get("ARTIST"); err == nil && len(values) > 0 {
				meta.Artist = values[0]
			}
			if values, err := vc.Get("ALBUM"); err == nil && len(values) > 0 {
				meta.Album = values[0]
			}
			if values, err := vc.Get("ALBUMARTIST"); err == nil && len(values) > 0 {
				meta.AlbumArtist = values[0]
			}
			if values, err := vc.Get("DATE"); err == nil && len(values) > 0 {
				meta.Date = values[0]
			}
			if values, err := vc.Get("GENRE"); err == nil && len(values) > 0 {
				meta.Genre = values[0]
			}
			if values, err := vc.Get("TRACKNUMBER"); err == nil && len(values) > 0 {
				meta.TrackNumber = values[0]
			}
			if values, err := vc.Get("DISCNUMBER"); err == nil && len(values) > 0 {
				meta.DiscNumber = values[0]
			}
			if values, err := vc.Get("COMMENT"); err == nil && len(values) > 0 {
				meta.Comment = values[0]
			}
		}

		// Parse picture block for cover art
		if metaBlock.Type == flac.Picture {
			picture, err := flacpicture.ParseFromMetaDataBlock(*metaBlock)
			if err != nil {
				continue
			}
			meta.CoverArt = picture.ImageData
			meta.CoverMime = picture.MIME
		}
	}

	return meta, nil
}

// ExtractMetadataFromReader extracts metadata from a FLAC reader
func ExtractMetadataFromReader(r io.Reader) (*FLACMetadata, error) {
	flacFile, err := flac.ParseMetadata(r)
	if err != nil {
		return nil, err
	}

	meta := &FLACMetadata{}

	// Parse vorbis comments
	for _, metaBlock := range flacFile.Meta {
		if metaBlock.Type == flac.VorbisComment {
			vc, err := flacvorbis.ParseFromMetaDataBlock(*metaBlock)
			if err != nil {
				continue
			}

			if values, err := vc.Get("TITLE"); err == nil && len(values) > 0 {
				meta.Title = values[0]
			}
			if values, err := vc.Get("ARTIST"); err == nil && len(values) > 0 {
				meta.Artist = values[0]
			}
			if values, err := vc.Get("ALBUM"); err == nil && len(values) > 0 {
				meta.Album = values[0]
			}
			if values, err := vc.Get("ALBUMARTIST"); err == nil && len(values) > 0 {
				meta.AlbumArtist = values[0]
			}
			if values, err := vc.Get("DATE"); err == nil && len(values) > 0 {
				meta.Date = values[0]
			}
			if values, err := vc.Get("GENRE"); err == nil && len(values) > 0 {
				meta.Genre = values[0]
			}
			if values, err := vc.Get("TRACKNUMBER"); err == nil && len(values) > 0 {
				meta.TrackNumber = values[0]
			}
			if values, err := vc.Get("DISCNUMBER"); err == nil && len(values) > 0 {
				meta.DiscNumber = values[0]
			}
			if values, err := vc.Get("COMMENT"); err == nil && len(values) > 0 {
				meta.Comment = values[0]
			}
		}

		// Parse picture block for cover art
		if metaBlock.Type == flac.Picture {
			picture, err := flacpicture.ParseFromMetaDataBlock(*metaBlock)
			if err != nil {
				continue
			}
			meta.CoverArt = picture.ImageData
			meta.CoverMime = picture.MIME
		}
	}

	return meta, nil
}

// Metadata represents metadata to embed into a FLAC file
type Metadata struct {
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	Date        string
	TrackNumber string
	TotalTracks string
	DiscNumber  string
	TotalDiscs  string // Total number of discs
	ISRC        string
	Genre       string // Genre (may be empty)
	Copyright   string // Copyright info
	Label       string // Record label
	Description string
	Lyrics      string
	Explicit    bool   // Explicit content flag
	Composer    string // Composer (for classical music)
	Conductor   string // Conductor (for classical music)
}

// EmbedMetadata embeds metadata into a FLAC file
func EmbedMetadata(filePath string, meta Metadata, coverPath string) error {
	// Parse the FLAC file
	flacFile, err := flac.ParseFile(filePath)
	if err != nil {
		return err
	}

	// Create vorbis comments
	vc := flacvorbis.New()

	if meta.Title != "" {
		_ = vc.Add(flacvorbis.FIELD_TITLE, meta.Title)
	}
	if meta.Artist != "" {
		_ = vc.Add(flacvorbis.FIELD_ARTIST, meta.Artist)
	}
	if meta.Album != "" {
		_ = vc.Add(flacvorbis.FIELD_ALBUM, meta.Album)
	}
	if meta.AlbumArtist != "" {
		_ = vc.Add("ALBUMARTIST", meta.AlbumArtist)
	}
	if meta.Date != "" {
		_ = vc.Add(flacvorbis.FIELD_DATE, meta.Date)
	}
	if meta.TrackNumber != "" {
		_ = vc.Add(flacvorbis.FIELD_TRACKNUMBER, meta.TrackNumber)
	}
	if meta.TotalTracks != "" {
		_ = vc.Add("TOTALTRACKS", meta.TotalTracks)
	}
	if meta.DiscNumber != "" {
		_ = vc.Add("DISCNUMBER", meta.DiscNumber)
	}
	if meta.TotalDiscs != "" {
		_ = vc.Add("TOTALDISCS", meta.TotalDiscs)
	}
	if meta.ISRC != "" {
		_ = vc.Add(flacvorbis.FIELD_ISRC, meta.ISRC)
	}
	if meta.Genre != "" {
		_ = vc.Add(flacvorbis.FIELD_GENRE, meta.Genre)
	}
	if meta.Copyright != "" {
		_ = vc.Add(flacvorbis.FIELD_COPYRIGHT, meta.Copyright)
	}
	if meta.Label != "" {
		_ = vc.Add("LABEL", meta.Label)
		_ = vc.Add("ORGANIZATION", meta.Label) // Some players use this
	}
	if meta.Explicit {
		_ = vc.Add("EXPLICIT", "1")
	}
	if meta.Composer != "" {
		_ = vc.Add("COMPOSER", meta.Composer)
	}
	if meta.Conductor != "" {
		_ = vc.Add("CONDUCTOR", meta.Conductor)
	}
	if meta.Description != "" {
		_ = vc.Add("DESCRIPTION", meta.Description)
	}
	if meta.Lyrics != "" {
		_ = vc.Add("LYRICS", meta.Lyrics)
	}

	// Create vorbis comment metadata block
	vcData := vc.Marshal()

	// Remove existing VorbisComment metadata blocks
	var newMeta []*flac.MetaDataBlock
	for _, block := range flacFile.Meta {
		if block.Type != flac.VorbisComment {
			newMeta = append(newMeta, block)
		}
	}

	// Add new vorbis comment metadata block
	newMeta = append(newMeta, &vcData)

	// Embed cover art if provided
	if coverPath != "" {
		coverBlock, err := embedCoverArt(coverPath)
		if err != nil {
			return err
		}
		newMeta = append(newMeta, &coverBlock)
	}

	flacFile.Meta = newMeta

	// Save the modified FLAC file
	err = flacFile.Save(filePath)
	if err != nil {
		return err
	}

	return nil
}

// embedCoverArt embeds cover art into a FLAC file
func embedCoverArt(coverPath string) (flac.MetaDataBlock, error) {
	// Read cover art file
	coverData, err := os.ReadFile(coverPath)
	if err != nil {
		return flac.MetaDataBlock{}, err
	}

	// Determine MIME type
	mimeType := "image/jpeg"
	if len(coverData) > 4 {
		if coverData[0] == 0x89 && coverData[1] == 0x50 && coverData[2] == 0x4E && coverData[3] == 0x47 {
			mimeType = "image/png"
		} else if coverData[0] == 0x52 && coverData[1] == 0x49 && coverData[2] == 0x46 && coverData[3] == 0x46 {
			mimeType = "image/webp"
		}
	}

	// Create picture block
	picture, err := flacpicture.NewFromImageData(flacpicture.PictureTypeFrontCover, "Front cover", coverData, mimeType)
	if err != nil {
		return flac.MetaDataBlock{}, err
	}

	// Marshal returns flac.MetaDataBlock, not []byte
	return picture.Marshal(), nil
}

// ReadISRCFromFile reads ISRC from an existing FLAC file
func ReadISRCFromFile(filePath string) (string, error) {
	flacFile, err := flac.ParseFile(filePath)
	if err != nil {
		return "", err
	}

	// Find VorbisComment block
	for _, block := range flacFile.Meta {
		if block.Type == flac.VorbisComment {
			cmt, err := flacvorbis.ParseFromMetaDataBlock(*block)
			if err != nil {
				continue
			}

			// Get ISRC field
			isrcValues, err := cmt.Get(flacvorbis.FIELD_ISRC)
			if err == nil && len(isrcValues) > 0 {
				return isrcValues[0], nil
			}
		}
	}

	return "", nil // No ISRC found
}
