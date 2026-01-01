package download

import "spotisync/internal/db/models"

// DownloadResult contains the result of a download operation
type DownloadResult struct {
	// File paths
	LocalPath  string
	CoverPath  string
	LyricsPath string

	// File info
	FileSize int64

	// Source information
	SourceService models.SourceService
	SourceID      string

	// Status
	Status  models.JobStatus
	Message string

	// Granular status for each component
	SongStatus   string
	LyricsStatus string
	CoverStatus  string
}

// ErrorType represents different error categories
type ErrorType int

const (
	ErrorTypeNone ErrorType = iota
	ErrorTypeNotFound
	ErrorTypeDownloadFailed
	ErrorTypeMetadataFailed
	ErrorTypeFilesystemError
	ErrorTypeSkipped
)

// DownloadError represents a download-specific error with type information
type DownloadError struct {
	Type    ErrorType
	Message string
	Err     error
}

func (e *DownloadError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *DownloadError) Unwrap() error {
	return e.Err
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(message string) *DownloadError {
	return &DownloadError{
		Type:    ErrorTypeNotFound,
		Message: message,
	}
}

// NewDownloadError creates a new download error
func NewDownloadError(message string, err error) *DownloadError {
	return &DownloadError{
		Type:    ErrorTypeDownloadFailed,
		Message: message,
		Err:     err,
	}
}

// NewMetadataError creates a new metadata error
func NewMetadataError(message string, err error) *DownloadError {
	return &DownloadError{
		Type:    ErrorTypeMetadataFailed,
		Message: message,
		Err:     err,
	}
}

// NewFilesystemError creates a new filesystem error
func NewFilesystemError(message string, err error) *DownloadError {
	return &DownloadError{
		Type:    ErrorTypeFilesystemError,
		Message: message,
		Err:     err,
	}
}

// NewSkippedError creates a new skipped error (file already exists)
func NewSkippedError(message string) *DownloadError {
	return &DownloadError{
		Type:    ErrorTypeSkipped,
		Message: message,
	}
}

// IsNotFoundError checks if the error is a not found error
func IsNotFoundError(err error) bool {
	if de, ok := err.(*DownloadError); ok {
		return de.Type == ErrorTypeNotFound
	}
	return false
}

// IsSkippedError checks if the error is a skipped error
func IsSkippedError(err error) bool {
	if de, ok := err.(*DownloadError); ok {
		return de.Type == ErrorTypeSkipped
	}
	return false
}
