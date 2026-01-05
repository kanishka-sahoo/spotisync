package storage

import (
	"fmt"
	"io"
	"io/fs"
	"log"
)

// Storage defines the interface for file storage operations
// Implementations can target local filesystem or remote SFTP servers
type Storage interface {
	// WriteFile writes data to a file at the specified path
	// Creates parent directories if they don't exist
	WriteFile(path string, data []byte, perm fs.FileMode) error

	// ReadFile reads the entire file at the specified path
	ReadFile(path string) ([]byte, error)

	// ReadRange reads a range of bytes from a file
	// Returns the bytes read, which may be less than the requested range if EOF is reached
	ReadRange(path string, offset int64, length int64) ([]byte, error)

	// MkdirAll creates a directory path, including parent directories
	MkdirAll(path string, perm fs.FileMode) error

	// Stat returns file information for the specified path
	Stat(path string) (fs.FileInfo, error)

	// Remove deletes a file at the specified path
	Remove(path string) error

	// Rename moves/renames a file from src to dst
	Rename(src, dst string) error

	// Open opens a file for reading
	// Caller is responsible for closing the returned ReadCloser
	Open(path string) (io.ReadCloser, error)

	// Create creates or truncates a file for writing
	// Caller is responsible for closing the returned WriteCloser
	Create(path string) (io.WriteCloser, error)

	// WalkFunc is called for each file or directory visited by Walk
	// path is the file/directory path, info contains file metadata, err is any error encountered
	Walk(root string, walkFn func(path string, info fs.FileInfo, err error) error) error

	// Close closes any persistent connections
	// Should be called when the storage is no longer needed
	Close() error
}

// Config holds configuration for storage backend initialization
type Config struct {
	Type string // "local" or "sftp"
	SFTP SFTPConfig
}

// NewStorage creates the appropriate storage backend based on configuration
func NewStorage(cfg Config) (Storage, error) {
	switch cfg.Type {
	case "", "local":
		log.Printf("Initializing local filesystem storage")
		return NewLocalStorage(), nil
	case "sftp":
		log.Printf("Initializing SFTP storage (host: %s:%d, remote path: %s)",
			cfg.SFTP.Host, cfg.SFTP.Port, cfg.SFTP.RemotePath)
		return NewSFTPStorage(cfg.SFTP)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Type)
	}
}
