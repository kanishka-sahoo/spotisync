package models

import (
	"time"

	"github.com/google/uuid"
)

// JobStatus represents the status of a job
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusInProgress JobStatus = "in_progress"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusNotFound   JobStatus = "not_found"
	JobStatusSkipped    JobStatus = "skipped"
)

// SourceService represents the source music service
type SourceService string

const (
	SourceTidal SourceService = "tidal"
	SourceQobuz SourceService = "qobuz"
)

// Job represents a single download job
type Job struct {
	ID      string `json:"id"`
	BatchID string `json:"batch_id"`
	UserID  int64  `json:"user_id"`

	// Spotify metadata
	SpotifyTrackID string   `json:"spotify_track_id"`
	ISRC           string   `json:"isrc,omitempty"`
	TrackName      string   `json:"track_name"`
	ArtistName     string   `json:"artist_name"`
	Artists        []string `json:"artists,omitempty"` // Full list of artists for metadata
	AlbumName      string   `json:"album_name,omitempty"`
	AlbumArtist    string   `json:"album_artist,omitempty"`
	AlbumArtists   []string `json:"album_artists,omitempty"` // Full list of album artists
	TrackNumber    int      `json:"track_number,omitempty"`
	DiscNumber     int      `json:"disc_number,omitempty"`
	TotalTracks    int      `json:"total_tracks,omitempty"`
	TotalDiscs     int      `json:"total_discs,omitempty"`
	DurationMs     int      `json:"duration_ms,omitempty"`
	ReleaseYear    int      `json:"release_year,omitempty"`
	ReleaseDate    string   `json:"release_date,omitempty"`

	// Additional metadata
	Genre       string `json:"genre,omitempty"`
	Copyright   string `json:"copyright,omitempty"`
	Label       string `json:"label,omitempty"`
	Explicit    bool   `json:"explicit,omitempty"`
	CoverArtURL string `json:"cover_art_url,omitempty"`

	// Granular status for progress indicators
	SongStatus   string `json:"song_status,omitempty"`   // pending, downloading, completed, failed
	LyricsStatus string `json:"lyrics_status,omitempty"` // pending, fetching, completed, failed, not_found
	CoverStatus  string `json:"cover_status,omitempty"`  // pending, fetching, completed, failed, not_found

	// Download info
	Status        JobStatus     `json:"status"`
	InPlaylist    bool          `json:"in_playlist"`
	SourceService SourceService `json:"source_service,omitempty"`
	SourceID      string        `json:"source_id,omitempty"`
	LocalPath     string        `json:"local_path,omitempty"`
	FileSize      int64         `json:"file_size,omitempty"`

	// Additional files paths
	CoverPath  string `json:"cover_path,omitempty"`
	LyricsPath string `json:"lyrics_path,omitempty"`

	// Retry info
	RetryCount   int    `json:"retry_count"`
	ErrorMessage string `json:"error_message,omitempty"`

	// Progress
	Progress        float64 `json:"progress"`
	DownloadSpeed   float64 `json:"download_speed"`
	BytesDownloaded int64   `json:"bytes_downloaded"`
	BytesTotal      int64   `json:"bytes_total"`

	// Timestamps
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// NewJob creates a new job with a generated UUID
func NewJob(batchID string, userID int64, track *SpotifyTrack) *Job {
	now := time.Now()
	return &Job{
		ID:             uuid.New().String(),
		BatchID:        batchID,
		UserID:         userID,
		SpotifyTrackID: track.ID,
		ISRC:           track.ISRC,
		TrackName:      track.Name,
		ArtistName:     track.Artist,
		Artists:        track.Artists,
		AlbumName:      track.Album,
		AlbumArtist:    track.AlbumArtist,
		AlbumArtists:   track.AlbumArtists,
		TrackNumber:    track.TrackNumber,
		DiscNumber:     track.DiscNumber,
		TotalTracks:    track.TotalTracks,
		TotalDiscs:     track.TotalDiscs,
		DurationMs:     track.DurationMs,
		ReleaseYear:    track.ReleaseYear,
		ReleaseDate:    track.ReleaseDate,
		Genre:          "", // Genre is typically not available from Spotify API directly
		Copyright:      track.Copyright,
		Label:          track.Label,
		Explicit:       track.Explicit,
		CoverArtURL:    track.CoverArtURL,
		SongStatus:     "pending",
		LyricsStatus:   "pending",
		CoverStatus:    "pending",
		Status:         JobStatusPending,
		RetryCount:     0,
		Progress:       0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// SpotifyTrack represents Spotify track metadata
type SpotifyTrack struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Artist       string   `json:"artist"`
	Artists      []string `json:"artists"`
	Album        string   `json:"album"`
	AlbumID      string   `json:"album_id"`
	AlbumArtist  string   `json:"album_artist"`
	AlbumArtists []string `json:"album_artists"`
	TrackNumber  int      `json:"track_number"`
	DiscNumber   int      `json:"disc_number"`
	DurationMs   int      `json:"duration_ms"`
	ISRC         string   `json:"isrc"`
	ReleaseYear  int      `json:"release_year"`
	ReleaseDate  string   `json:"release_date"`
	TotalTracks  int      `json:"total_tracks"`
	TotalDiscs   int      `json:"total_discs"`
	CoverArtURL  string   `json:"cover_art_url"`
	Explicit     bool     `json:"explicit"`
	Label        string   `json:"label,omitempty"`
	Copyright    string   `json:"copyright,omitempty"`
	Lyrics       string   `json:"lyrics,omitempty"`
}

// JobResponse is the public-facing job data
type JobResponse struct {
	ID              string        `json:"id"`
	BatchID         string        `json:"batch_id"`
	SpotifyTrackID  string        `json:"spotify_track_id"`
	ISRC            string        `json:"isrc,omitempty"`
	TrackName       string        `json:"track_name"`
	ArtistName      string        `json:"artist_name"`
	AlbumName       string        `json:"album_name,omitempty"`
	AlbumArtist     string        `json:"album_artist,omitempty"`
	TrackNumber     int           `json:"track_number,omitempty"`
	DiscNumber      int           `json:"disc_number,omitempty"`
	TotalTracks     int           `json:"total_tracks,omitempty"`
	DurationMs      int           `json:"duration_ms,omitempty"`
	ReleaseYear     int           `json:"release_year,omitempty"`
	Genre           string        `json:"genre,omitempty"`
	Copyright       string        `json:"copyright,omitempty"`
	Label           string        `json:"label,omitempty"`
	Explicit        bool          `json:"explicit,omitempty"`
	SongStatus      string        `json:"song_status,omitempty"`
	LyricsStatus    string        `json:"lyrics_status,omitempty"`
	CoverStatus     string        `json:"cover_status,omitempty"`
	Status          JobStatus     `json:"status"`
	InPlaylist      bool          `json:"in_playlist"`
	SourceService   SourceService `json:"source_service,omitempty"`
	LocalPath       string        `json:"local_path,omitempty"`
	CoverPath       string        `json:"cover_path,omitempty"`
	LyricsPath      string        `json:"lyrics_path,omitempty"`
	FileSize        int64         `json:"file_size,omitempty"`
	RetryCount      int           `json:"retry_count"`
	ErrorMessage    string        `json:"error_message,omitempty"`
	Progress        float64       `json:"progress"`
	DownloadSpeed   float64       `json:"download_speed"`
	BytesDownloaded int64         `json:"bytes_downloaded"`
	BytesTotal      int64         `json:"bytes_total"`
	StartedAt       string        `json:"started_at,omitempty"`
	CompletedAt     string        `json:"completed_at,omitempty"`
	CreatedAt       string        `json:"created_at"`
	UpdatedAt       string        `json:"updated_at"`
}

// ToResponse converts a Job to JobResponse
func (j *Job) ToResponse() *JobResponse {
	resp := &JobResponse{
		ID:              j.ID,
		BatchID:         j.BatchID,
		SpotifyTrackID:  j.SpotifyTrackID,
		ISRC:            j.ISRC,
		TrackName:       j.TrackName,
		ArtistName:      j.ArtistName,
		AlbumName:       j.AlbumName,
		AlbumArtist:     j.AlbumArtist,
		TrackNumber:     j.TrackNumber,
		DiscNumber:      j.DiscNumber,
		TotalTracks:     j.TotalTracks,
		DurationMs:      j.DurationMs,
		ReleaseYear:     j.ReleaseYear,
		Genre:           j.Genre,
		Copyright:       j.Copyright,
		Label:           j.Label,
		Explicit:        j.Explicit,
		SongStatus:      j.SongStatus,
		LyricsStatus:    j.LyricsStatus,
		CoverStatus:     j.CoverStatus,
		Status:          j.Status,
		InPlaylist:      j.InPlaylist,
		SourceService:   j.SourceService,
		LocalPath:       j.LocalPath,
		CoverPath:       j.CoverPath,
		LyricsPath:      j.LyricsPath,
		FileSize:        j.FileSize,
		RetryCount:      j.RetryCount,
		ErrorMessage:    j.ErrorMessage,
		Progress:        j.Progress,
		DownloadSpeed:   j.DownloadSpeed,
		BytesDownloaded: j.BytesDownloaded,
		BytesTotal:      j.BytesTotal,
		CreatedAt:       j.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       j.UpdatedAt.Format(time.RFC3339),
	}
	if j.StartedAt != nil {
		resp.StartedAt = j.StartedAt.Format(time.RFC3339)
	}
	if j.CompletedAt != nil {
		resp.CompletedAt = j.CompletedAt.Format(time.RFC3339)
	}
	return resp
}
