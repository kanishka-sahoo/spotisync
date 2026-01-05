package storage

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestLocalStorage tests local storage implementation
func TestLocalStorage(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "storage-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage := NewLocalStorage()
	defer storage.Close()

	testPath := filepath.Join(tmpDir, "subdir", "test.txt")
	testData := []byte("Hello, World!")

	// Test WriteFile
	if err := storage.WriteFile(testPath, testData, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Test Stat
	info, err := storage.Stat(testPath)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Size() != int64(len(testData)) {
		t.Errorf("Expected size %d, got %d", len(testData), info.Size())
	}

	// Test ReadFile
	readData, err := storage.ReadFile(testPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(readData) != string(testData) {
		t.Errorf("Expected data %q, got %q", testData, readData)
	}

	// Test Open
	rc, err := storage.Open(testPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer rc.Close()
	readData2, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if string(readData2) != string(testData) {
		t.Errorf("Expected data %q, got %q", testData, readData2)
	}

	// Test Rename
	newPath := filepath.Join(tmpDir, "renamed.txt")
	if err := storage.Rename(testPath, newPath); err != nil {
		t.Fatalf("Rename failed: %v", err)
	}
	if _, err := storage.Stat(testPath); !os.IsNotExist(err) {
		t.Errorf("Old file should not exist after rename")
	}
	if _, err := storage.Stat(newPath); err != nil {
		t.Errorf("New file should exist after rename: %v", err)
	}

	// Test Remove
	if err := storage.Remove(newPath); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if _, err := storage.Stat(newPath); !os.IsNotExist(err) {
		t.Errorf("File should not exist after remove")
	}

	// Test MkdirAll
	dirPath := filepath.Join(tmpDir, "a", "b", "c")
	if err := storage.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if info, err := storage.Stat(dirPath); err != nil || !info.IsDir() {
		t.Errorf("Directory should exist and be a directory")
	}

	// Test Create
	createPath := filepath.Join(tmpDir, "created.txt")
	wc, err := storage.Create(createPath)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := wc.Write(testData); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := wc.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	readData3, err := storage.ReadFile(createPath)
	if err != nil {
		t.Fatalf("ReadFile after Create failed: %v", err)
	}
	if string(readData3) != string(testData) {
		t.Errorf("Expected data %q, got %q", testData, readData3)
	}
}

// TestNewStorage tests the factory function
func TestNewStorage(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "local storage",
			config: Config{
				Type: "local",
			},
			wantErr: false,
		},
		{
			name: "default to local storage",
			config: Config{
				Type: "",
			},
			wantErr: false,
		},
		{
			name: "invalid storage type",
			config: Config{
				Type: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := NewStorage(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewStorage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if storage != nil {
				defer storage.Close()
			}
		})
	}
}
