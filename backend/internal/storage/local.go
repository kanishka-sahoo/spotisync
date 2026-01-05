package storage

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// LocalStorage implements Storage interface for local filesystem operations
type LocalStorage struct{}

// NewLocalStorage creates a new local filesystem storage backend
func NewLocalStorage() *LocalStorage {
	return &LocalStorage{}
}

// WriteFile writes data to a file on the local filesystem
func (l *LocalStorage) WriteFile(path string, data []byte, perm fs.FileMode) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

// ReadFile reads a file from the local filesystem
func (l *LocalStorage) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// ReadRange reads a range of bytes from a file
func (l *LocalStorage) ReadRange(path string, offset int64, length int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Seek to offset
	if _, err := file.Seek(offset, 0); err != nil {
		return nil, err
	}

	// Read up to length bytes
	buffer := make([]byte, length)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return buffer[:n], nil
}

// MkdirAll creates directory and all parent directories on local filesystem
func (l *LocalStorage) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Stat returns file information for a local file
func (l *LocalStorage) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

// Remove deletes a file from the local filesystem
func (l *LocalStorage) Remove(path string) error {
	return os.Remove(path)
}

// Rename moves/renames a file on the local filesystem
func (l *LocalStorage) Rename(src, dst string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

// Open opens a local file for reading
func (l *LocalStorage) Open(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

// Create creates or truncates a local file for writing
func (l *LocalStorage) Create(path string) (io.WriteCloser, error) {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	return os.Create(path)
}

// Walk walks the file tree rooted at root
func (l *LocalStorage) Walk(root string, walkFn func(path string, info fs.FileInfo, err error) error) error {
	return filepath.Walk(root, filepath.WalkFunc(walkFn))
}

// Close is a no-op for local storage (no persistent connections)
func (l *LocalStorage) Close() error {
	return nil
}
