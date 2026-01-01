package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"spotisync/internal/config"
	"spotisync/internal/db/models"
)

// Database wraps the SQL connection with Spotisync-specific operations
type Database struct {
	*sql.DB
	cfg *config.DatabaseConfig
}

// New creates a new database connection
func New(cfg *config.DatabaseConfig) (*Database, error) {
	// Ensure the directory exists
	dbDir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Verify connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	database := &Database{
		DB:  db,
		cfg: cfg,
	}

	// Run migrations
	if err := database.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return database, nil
}

// migrate runs database migrations
func (d *Database) migrate() error {
	migrations := []string{
		// Migration 001: Initial schema
		`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			navidrome_url TEXT,
			navidrome_username TEXT,
			navidrome_password TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		`,
		`
		CREATE TABLE IF NOT EXISTS batches (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			spotify_url TEXT NOT NULL,
			spotify_type TEXT NOT NULL,
			name TEXT,
			total_jobs INTEGER DEFAULT 0,
			completed_jobs INTEGER DEFAULT 0,
			failed_jobs INTEGER DEFAULT 0,
			status TEXT DEFAULT 'pending',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
		`,
		`
		CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			batch_id TEXT NOT NULL,
			user_id INTEGER NOT NULL,
			spotify_track_id TEXT NOT NULL,
			isrc TEXT,
			track_name TEXT NOT NULL,
			artist_name TEXT NOT NULL,
			album_name TEXT,
			album_artist TEXT,
			track_number INTEGER,
			disc_number INTEGER,
			total_tracks INTEGER,
			total_discs INTEGER,
			duration_ms INTEGER,
			release_year INTEGER,
			release_date TEXT,
			cover_art_url TEXT,
			song_status TEXT DEFAULT 'pending',
			lyrics_status TEXT DEFAULT 'pending',
			cover_status TEXT DEFAULT 'pending',
			status TEXT DEFAULT 'pending',
			source_service TEXT,
			source_id TEXT,
			local_path TEXT,
			cover_path TEXT,
			lyrics_path TEXT,
			file_size INTEGER,
			retry_count INTEGER DEFAULT 0,
			error_message TEXT,
			progress REAL DEFAULT 0,
			download_speed REAL DEFAULT 0,
			bytes_downloaded INTEGER DEFAULT 0,
			bytes_total INTEGER DEFAULT 0,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (batch_id) REFERENCES batches(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
		`,
		// Create indexes
		`CREATE INDEX IF NOT EXISTS idx_jobs_batch_id ON jobs(batch_id);`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_user_id ON jobs(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_isrc ON jobs(isrc);`,
		`CREATE INDEX IF NOT EXISTS idx_batches_user_id ON batches(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_batches_status ON batches(status);`,
	}

	// Run each migration
	for i, migration := range migrations {
		if _, err := d.Exec(migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}

	// Migrate existing databases: add cover_art_url column if it doesn't exist
	// SQLite doesn't support IF NOT EXISTS for ALTER TABLE, so we try and ignore errors
	_, _ = d.Exec("ALTER TABLE jobs ADD COLUMN cover_art_url TEXT")

	// Migrate existing databases: add granular status columns if they don't exist
	_, _ = d.Exec("ALTER TABLE jobs ADD COLUMN song_status TEXT DEFAULT 'pending'")
	_, _ = d.Exec("ALTER TABLE jobs ADD COLUMN lyrics_status TEXT DEFAULT 'pending'")
	_, _ = d.Exec("ALTER TABLE jobs ADD COLUMN cover_status TEXT DEFAULT 'pending'")

	// Migrate existing databases: add artists columns for full artist metadata
	_, _ = d.Exec("ALTER TABLE jobs ADD COLUMN artists TEXT")
	_, _ = d.Exec("ALTER TABLE jobs ADD COLUMN album_artists TEXT")

	// Migrate existing databases: add artists columns for full artist metadata
	_, _ = d.Exec("ALTER TABLE jobs ADD COLUMN artists TEXT")
	_, _ = d.Exec("ALTER TABLE jobs ADD COLUMN album_artists TEXT")

	// Enable WAL mode for better concurrency
	if _, err := d.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := d.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	return nil
}

// CreateUser creates a new user
func (d *Database) CreateUser(username, passwordHash string) (int64, error) {
	result, err := d.Exec(
		`INSERT INTO users (username, password_hash) VALUES (?, ?)`,
		username, passwordHash,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetUserByUsername retrieves a user by username
func (d *Database) GetUserByUsername(username string) (*models.User, error) {
	var user models.User
	var navidromeURL, navidromeUsername, navidromePassword sql.NullString
	err := d.QueryRow(
		`SELECT id, username, password_hash, navidrome_url, navidrome_username, 
				navidrome_password, created_at, updated_at 
		 FROM users WHERE username = ?`,
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &navidromeURL,
		&navidromeUsername, &navidromePassword, &user.CreatedAt, &user.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Handle NULL values
	if navidromeURL.Valid {
		user.NavidromeURL = navidromeURL.String
	}
	if navidromeUsername.Valid {
		user.NavidromeUsername = navidromeUsername.String
	}
	if navidromePassword.Valid {
		user.NavidromePassword = navidromePassword.String
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID
func (d *Database) GetUserByID(id int64) (*models.User, error) {
	var user models.User
	var navidromeURL, navidromeUsername, navidromePassword sql.NullString
	err := d.QueryRow(
		`SELECT id, username, password_hash, navidrome_url, navidrome_username, 
				navidrome_password, created_at, updated_at 
		 FROM users WHERE id = ?`,
		id,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &navidromeURL,
		&navidromeUsername, &navidromePassword, &user.CreatedAt, &user.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Handle NULL values
	if navidromeURL.Valid {
		user.NavidromeURL = navidromeURL.String
	}
	if navidromeUsername.Valid {
		user.NavidromeUsername = navidromeUsername.String
	}
	if navidromePassword.Valid {
		user.NavidromePassword = navidromePassword.String
	}

	return &user, nil
}

// UpdateUserNavidrome updates a user's Navidrome credentials with partial update support
// For navidromeURL: if nil, preserve existing; if empty string, clear to NULL; otherwise set to value
func (d *Database) UpdateUserNavidrome(id int64, navidromeURL interface{}, navidromeUsername, navidromePassword string) error {
	// Build the query dynamically based on what needs to be updated
	sets := []string{}
	params := []interface{}{}

	// Handle URL - if empty string, clear to NULL; if non-empty string, set to value; if nil, preserve
	if urlStr, ok := navidromeURL.(string); ok {
		if urlStr == "" {
			// Clear URL to NULL
			sets = append(sets, `navidrome_url = NULL`)
		} else {
			// Set to specific URL value
			sets = append(sets, `navidrome_url = ?`)
			params = append(params, urlStr)
		}
	}
	// If navidromeURL is nil, don't include it in the update (preserve existing)

	// Username: NULLIF to convert empty string to NULL, then COALESCE to keep existing if new is NULL
	if navidromeUsername != "" || navidromePassword != "" {
		// At least one credential is being updated
		sets = append(sets, `navidrome_username = COALESCE(NULLIF(?, ''), navidrome_username)`)
		params = append(params, navidromeUsername)

		sets = append(sets, `navidrome_password = COALESCE(NULLIF(?, ''), navidrome_password)`)
		params = append(params, navidromePassword)
	}

	// Build final query
	if len(sets) == 0 {
		// Nothing to update
		return nil
	}

	query := `UPDATE users SET ` + strings.Join(sets, ", ") + `, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	params = append(params, id)

	_, err := d.Exec(query, params...)
	return err
}

// CreateBatch creates a new batch
func (d *Database) CreateBatch(batch *models.Batch) error {
	result, err := d.Exec(
		`INSERT INTO batches (id, user_id, spotify_url, spotify_type, name, total_jobs, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		batch.ID, batch.UserID, batch.SpotifyURL, batch.SpotifyType, batch.Name, batch.TotalJobs, batch.Status,
		batch.CreatedAt, batch.UpdatedAt,
	)
	if err != nil {
		return err
	}
	_ = result
	return nil
}

// GetBatchByID retrieves a batch by ID
func (d *Database) GetBatchByID(id string) (*models.Batch, error) {
	var batch models.Batch
	err := d.QueryRow(
		`SELECT id, user_id, spotify_url, spotify_type, name, total_jobs, completed_jobs, failed_jobs, status,
				created_at, updated_at
		 FROM batches WHERE id = ?`,
		id,
	).Scan(&batch.ID, &batch.UserID, &batch.SpotifyURL, &batch.SpotifyType, &batch.Name, &batch.TotalJobs,
		&batch.CompletedJobs, &batch.FailedJobs, &batch.Status, &batch.CreatedAt, &batch.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &batch, nil
}

// GetBatchesByUserID retrieves all batches for a user
func (d *Database) GetBatchesByUserID(userID int64) ([]*models.Batch, error) {
	rows, err := d.Query(
		`SELECT id, user_id, spotify_url, spotify_type, name, total_jobs, completed_jobs, failed_jobs, status,
				created_at, updated_at
		 FROM batches WHERE user_id = ? ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var batches []*models.Batch
	for rows.Next() {
		var batch models.Batch
		if err := rows.Scan(&batch.ID, &batch.UserID, &batch.SpotifyURL, &batch.SpotifyType, &batch.Name, &batch.TotalJobs,
			&batch.CompletedJobs, &batch.FailedJobs, &batch.Status, &batch.CreatedAt, &batch.UpdatedAt); err != nil {
			return nil, err
		}
		batches = append(batches, &batch)
	}
	return batches, nil
}

// UpdateBatchStatus updates the status and job counts of a batch
func (d *Database) UpdateBatchStatus(id string, status models.BatchStatus, completedJobs, failedJobs int) error {
	_, err := d.Exec(
		`UPDATE batches SET status = ?, completed_jobs = ?, failed_jobs = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, completedJobs, failedJobs, id,
	)
	return err
}

// DeleteBatch deletes a batch and its jobs
func (d *Database) DeleteBatch(id string) error {
	// First delete all jobs in the batch
	if _, err := d.Exec("DELETE FROM jobs WHERE batch_id = ?", id); err != nil {
		return err
	}
	// Then delete the batch
	if _, err := d.Exec("DELETE FROM batches WHERE id = ?", id); err != nil {
		return err
	}
	return nil
}

// CreateJob creates a new job
func (d *Database) CreateJob(job *models.Job) error {
	// Serialize artists arrays to JSON
	artistsJSON := ""
	if len(job.Artists) > 0 {
		if data, err := json.Marshal(job.Artists); err == nil {
			artistsJSON = string(data)
		}
	}
	albumArtistsJSON := ""
	if len(job.AlbumArtists) > 0 {
		if data, err := json.Marshal(job.AlbumArtists); err == nil {
			albumArtistsJSON = string(data)
		}
	}

	result, err := d.Exec(
		`INSERT INTO jobs (id, batch_id, user_id, spotify_track_id, isrc, track_name, artist_name, artists, album_name, album_artist, album_artists,
			track_number, disc_number, total_tracks, total_discs, duration_ms, release_year, release_date, cover_art_url,
			song_status, lyrics_status, cover_status, status,
			source_service, source_id, local_path, cover_path, lyrics_path, file_size, retry_count, error_message,
			progress, download_speed, bytes_downloaded, bytes_total, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.BatchID, job.UserID, job.SpotifyTrackID, job.ISRC, job.TrackName, job.ArtistName, artistsJSON, job.AlbumName,
		job.AlbumArtist, albumArtistsJSON, job.TrackNumber, job.DiscNumber, job.TotalTracks, job.TotalDiscs, job.DurationMs,
		job.ReleaseYear, job.ReleaseDate, job.CoverArtURL, job.SongStatus, job.LyricsStatus, job.CoverStatus,
		job.Status, job.SourceService, job.SourceID, job.LocalPath,
		job.CoverPath, job.LyricsPath, job.FileSize, job.RetryCount, job.ErrorMessage, job.Progress,
		job.DownloadSpeed, job.BytesDownloaded, job.BytesTotal, job.CreatedAt, job.UpdatedAt,
	)
	if err != nil {
		return err
	}
	_ = result
	return nil
}

// GetJobByID retrieves a job by ID
func (d *Database) GetJobByID(id string) (*models.Job, error) {
	var job models.Job
	var artistsJSON, albumArtistsJSON sql.NullString
	err := d.QueryRow(
		`SELECT id, batch_id, user_id, spotify_track_id, isrc, track_name, artist_name, artists, album_name, album_artist, album_artists,
			track_number, disc_number, total_tracks, total_discs, duration_ms, release_year, release_date, cover_art_url,
			song_status, lyrics_status, cover_status, status,
			source_service, source_id, local_path, cover_path, lyrics_path, file_size, retry_count, error_message,
			progress, download_speed, bytes_downloaded, bytes_total, started_at, completed_at, created_at, updated_at
		 FROM jobs WHERE id = ?`,
		id,
	).Scan(&job.ID, &job.BatchID, &job.UserID, &job.SpotifyTrackID, &job.ISRC, &job.TrackName, &job.ArtistName,
		&artistsJSON, &job.AlbumName, &job.AlbumArtist, &albumArtistsJSON, &job.TrackNumber, &job.DiscNumber, &job.TotalTracks, &job.TotalDiscs,
		&job.DurationMs, &job.ReleaseYear, &job.ReleaseDate, &job.CoverArtURL,
		&job.SongStatus, &job.LyricsStatus, &job.CoverStatus, &job.Status,
		&job.SourceService, &job.SourceID,
		&job.LocalPath, &job.CoverPath, &job.LyricsPath, &job.FileSize, &job.RetryCount, &job.ErrorMessage,
		&job.Progress, &job.DownloadSpeed, &job.BytesDownloaded, &job.BytesTotal, &job.StartedAt, &job.CompletedAt,
		&job.CreatedAt, &job.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Parse artists JSON
	if artistsJSON.Valid && artistsJSON.String != "" {
		json.Unmarshal([]byte(artistsJSON.String), &job.Artists)
	}
	if albumArtistsJSON.Valid && albumArtistsJSON.String != "" {
		json.Unmarshal([]byte(albumArtistsJSON.String), &job.AlbumArtists)
	}

	return &job, nil
}

// GetJobsByUserID retrieves all jobs for a user
func (d *Database) GetJobsByUserID(userID int64) ([]*models.Job, error) {
	rows, err := d.Query(
		`SELECT id, batch_id, user_id, spotify_track_id, isrc, track_name, artist_name, artists, album_name, album_artist, album_artists,
			track_number, disc_number, total_tracks, total_discs, duration_ms, release_year, release_date, cover_art_url,
			song_status, lyrics_status, cover_status, status,
			source_service, source_id, local_path, cover_path, lyrics_path, file_size, retry_count, error_message,
			progress, download_speed, bytes_downloaded, bytes_total, started_at, completed_at, created_at, updated_at
		 FROM jobs WHERE user_id = ? ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		var job models.Job
		var artistsJSON, albumArtistsJSON sql.NullString
		if err := rows.Scan(&job.ID, &job.BatchID, &job.UserID, &job.SpotifyTrackID, &job.ISRC, &job.TrackName,
			&job.ArtistName, &artistsJSON, &job.AlbumName, &job.AlbumArtist, &albumArtistsJSON, &job.TrackNumber, &job.DiscNumber, &job.TotalTracks,
			&job.TotalDiscs, &job.DurationMs, &job.ReleaseYear, &job.ReleaseDate, &job.CoverArtURL,
			&job.SongStatus, &job.LyricsStatus, &job.CoverStatus, &job.Status,
			&job.SourceService,
			&job.SourceID, &job.LocalPath, &job.CoverPath, &job.LyricsPath, &job.FileSize, &job.RetryCount,
			&job.ErrorMessage, &job.Progress, &job.DownloadSpeed, &job.BytesDownloaded, &job.BytesTotal,
			&job.StartedAt, &job.CompletedAt, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, err
		}
		// Parse artists JSON
		if artistsJSON.Valid && artistsJSON.String != "" {
			json.Unmarshal([]byte(artistsJSON.String), &job.Artists)
		}
		if albumArtistsJSON.Valid && albumArtistsJSON.String != "" {
			json.Unmarshal([]byte(albumArtistsJSON.String), &job.AlbumArtists)
		}
		jobs = append(jobs, &job)
	}
	return jobs, nil
}

// GetJobsByBatchID retrieves all jobs for a batch
func (d *Database) GetJobsByBatchID(batchID string) ([]*models.Job, error) {
	rows, err := d.Query(
		`SELECT id, batch_id, user_id, spotify_track_id, isrc, track_name, artist_name, artists, album_name, album_artist, album_artists,
			track_number, disc_number, total_tracks, total_discs, duration_ms, release_year, release_date, cover_art_url,
			song_status, lyrics_status, cover_status, status,
			source_service, source_id, local_path, cover_path, lyrics_path, file_size, retry_count, error_message,
			progress, download_speed, bytes_downloaded, bytes_total, started_at, completed_at, created_at, updated_at
		 FROM jobs WHERE batch_id = ? ORDER BY created_at ASC`,
		batchID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		var job models.Job
		var artistsJSON, albumArtistsJSON sql.NullString
		if err := rows.Scan(&job.ID, &job.BatchID, &job.UserID, &job.SpotifyTrackID, &job.ISRC, &job.TrackName,
			&job.ArtistName, &artistsJSON, &job.AlbumName, &job.AlbumArtist, &albumArtistsJSON, &job.TrackNumber, &job.DiscNumber, &job.TotalTracks,
			&job.TotalDiscs, &job.DurationMs, &job.ReleaseYear, &job.ReleaseDate, &job.CoverArtURL,
			&job.SongStatus, &job.LyricsStatus, &job.CoverStatus, &job.Status,
			&job.SourceService,
			&job.SourceID, &job.LocalPath, &job.CoverPath, &job.LyricsPath, &job.FileSize, &job.RetryCount,
			&job.ErrorMessage, &job.Progress, &job.DownloadSpeed, &job.BytesDownloaded, &job.BytesTotal,
			&job.StartedAt, &job.CompletedAt, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, err
		}
		// Parse artists JSON
		if artistsJSON.Valid && artistsJSON.String != "" {
			json.Unmarshal([]byte(artistsJSON.String), &job.Artists)
		}
		if albumArtistsJSON.Valid && albumArtistsJSON.String != "" {
			json.Unmarshal([]byte(albumArtistsJSON.String), &job.AlbumArtists)
		}
		jobs = append(jobs, &job)
	}
	return jobs, nil
}

// UpdateJobStatus updates the status and details of a job
func (d *Database) UpdateJobStatus(id string, status models.JobStatus, sourceService, sourceID, localPath string,
	coverPath, lyricsPath string, fileSize int64, errorMessage string, progress float64) error {
	_, err := d.Exec(
		`UPDATE jobs SET status = ?, source_service = ?, source_id = ?, local_path = ?, cover_path = ?,
			lyrics_path = ?, file_size = ?, error_message = ?, progress = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		status, sourceService, sourceID, localPath, coverPath, lyricsPath, fileSize, errorMessage, progress, id,
	)
	return err
}

// UpdateJobProgress updates the progress of a job
func (d *Database) UpdateJobProgress(id string, progress float64, downloadSpeed float64, bytesDownloaded, bytesTotal int64) error {
	_, err := d.Exec(
		`UPDATE jobs SET progress = ?, download_speed = ?, bytes_downloaded = ?, bytes_total = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		progress, downloadSpeed, bytesDownloaded, bytesTotal, id,
	)
	return err
}

// UpdateJobStarted marks a job as started
func (d *Database) UpdateJobStarted(id string) error {
	_, err := d.Exec(
		`UPDATE jobs SET status = 'in_progress', started_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	)
	return err
}

// UpdateJobCompleted marks a job as completed
func (d *Database) UpdateJobCompleted(id string, localPath string, fileSize int64) error {
	_, err := d.Exec(
		`UPDATE jobs SET status = 'completed', local_path = ?, file_size = ?, completed_at = CURRENT_TIMESTAMP,
			progress = 100, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		localPath, fileSize, id,
	)
	return err
}

// UpdateJobFailed marks a job as failed
func (d *Database) UpdateJobFailed(id string, errorMessage string) error {
	_, err := d.Exec(
		`UPDATE jobs SET status = 'failed', error_message = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		errorMessage, id,
	)
	return err
}

// IncrementJobRetryCount increments the retry count for a job
func (d *Database) IncrementJobRetryCount(id string) error {
	_, err := d.Exec(
		`UPDATE jobs SET retry_count = retry_count + 1, status = 'pending', updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	)
	return err
}

// ResetJobForRetry resets a job for retry
func (d *Database) ResetJobForRetry(id string) error {
	_, err := d.Exec(
		`UPDATE jobs SET status = 'pending', progress = 0, download_speed = 0, bytes_downloaded = 0, bytes_total = 0,
			started_at = NULL, completed_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	)
	return err
}

// DeleteJob deletes a job
func (d *Database) DeleteJob(id string) error {
	_, err := d.Exec("DELETE FROM jobs WHERE id = ?", id)
	return err
}

// GetPendingJobs retrieves all pending jobs
func (d *Database) GetPendingJobs() ([]*models.Job, error) {
	rows, err := d.Query(
		`SELECT id, batch_id, user_id, spotify_track_id, isrc, track_name, artist_name, artists, album_name, album_artist, album_artists,
			track_number, disc_number, total_tracks, total_discs, duration_ms, release_year, release_date, cover_art_url,
			song_status, lyrics_status, cover_status, status,
			source_service, source_id, local_path, cover_path, lyrics_path, file_size, retry_count, error_message,
			progress, download_speed, bytes_downloaded, bytes_total, started_at, completed_at, created_at, updated_at
		 FROM jobs WHERE status = 'pending' ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		var job models.Job
		var artistsJSON, albumArtistsJSON sql.NullString
		if err := rows.Scan(&job.ID, &job.BatchID, &job.UserID, &job.SpotifyTrackID, &job.ISRC, &job.TrackName,
			&job.ArtistName, &artistsJSON, &job.AlbumName, &job.AlbumArtist, &albumArtistsJSON, &job.TrackNumber, &job.DiscNumber, &job.TotalTracks,
			&job.TotalDiscs, &job.DurationMs, &job.ReleaseYear, &job.ReleaseDate, &job.CoverArtURL,
			&job.SongStatus, &job.LyricsStatus, &job.CoverStatus, &job.Status,
			&job.SourceService,
			&job.SourceID, &job.LocalPath, &job.CoverPath, &job.LyricsPath, &job.FileSize, &job.RetryCount,
			&job.ErrorMessage, &job.Progress, &job.DownloadSpeed, &job.BytesDownloaded, &job.BytesTotal,
			&job.StartedAt, &job.CompletedAt, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, err
		}
		// Parse artists JSON
		if artistsJSON.Valid && artistsJSON.String != "" {
			json.Unmarshal([]byte(artistsJSON.String), &job.Artists)
		}
		if albumArtistsJSON.Valid && albumArtistsJSON.String != "" {
			json.Unmarshal([]byte(albumArtistsJSON.String), &job.AlbumArtists)
		}
		jobs = append(jobs, &job)
	}
	return jobs, nil
}

// UpdateJobGranularStatus updates the granular status fields for a job
func (d *Database) UpdateJobGranularStatus(id, songStatus, lyricsStatus, coverStatus string) error {
	_, err := d.Exec(
		`UPDATE jobs SET song_status = ?, lyrics_status = ?, cover_status = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		songStatus, lyricsStatus, coverStatus, id,
	)
	return err
}
