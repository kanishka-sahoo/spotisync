package storage

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SFTPStorage implements Storage interface for SFTP remote operations
type SFTPStorage struct {
	host       string
	port       int
	username   string
	password   string
	sshKeyPath string
	remotePath string
	sshClient  *ssh.Client
	sftpClient *sftp.Client
}

// SFTPConfig holds configuration for SFTP connection
type SFTPConfig struct {
	Host       string
	Port       int
	Username   string
	Password   string
	SSHKeyPath string
	RemotePath string
}

// NewSFTPStorage creates a new SFTP storage backend with persistent connection
func NewSFTPStorage(cfg SFTPConfig) (*SFTPStorage, error) {
	s := &SFTPStorage{
		host:       cfg.Host,
		port:       cfg.Port,
		username:   cfg.Username,
		password:   cfg.Password,
		sshKeyPath: cfg.SSHKeyPath,
		remotePath: cfg.RemotePath,
	}

	// Establish initial connection
	if err := s.connect(); err != nil {
		return nil, fmt.Errorf("failed to establish SFTP connection: %w", err)
	}

	// Verify the connection and remote path accessibility using SFTP-only operations
	if err := s.verifyConnection(); err != nil {
		s.Close() // Clean up the connection
		return nil, fmt.Errorf("SFTP connection verification failed: %w", err)
	}

	log.Printf("SFTP connection established and verified to %s:%d (remote path: %s)", cfg.Host, cfg.Port, cfg.RemotePath)
	return s, nil
}

// connect establishes SSH and SFTP connections
func (s *SFTPStorage) connect() error {
	// Build SSH client config
	config := &ssh.ClientConfig{
		User:            s.username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Consider adding host key verification
		Timeout:         30 * time.Second,
		// Optimize TCP settings for better throughput
		Config: ssh.Config{
			// Use faster ciphers that balance security and performance
			Ciphers: []string{
				"aes128-gcm@openssh.com",
				"chacha20-poly1305@openssh.com",
				"aes128-ctr",
				"aes256-ctr",
			},
		},
	}

	// Add authentication methods
	var authMethods []ssh.AuthMethod

	// Try SSH key authentication first if key path is provided
	if s.sshKeyPath != "" {
		key, err := os.ReadFile(s.sshKeyPath)
		if err != nil {
			log.Printf("Warning: failed to read SSH key from %s: %v", s.sshKeyPath, err)
		} else {
			signer, err := ssh.ParsePrivateKey(key)
			if err != nil {
				log.Printf("Warning: failed to parse SSH key: %v", err)
			} else {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
				log.Printf("Using SSH key authentication from %s", s.sshKeyPath)
			}
		}
	}

	// Add password authentication if password is provided
	if s.password != "" {
		authMethods = append(authMethods, ssh.Password(s.password))
		log.Printf("Using password authentication")
	}

	if len(authMethods) == 0 {
		return fmt.Errorf("no authentication methods available (provide either password or SSH key)")
	}

	config.Auth = authMethods

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("SSH connection failed to %s: %w", addr, err)
	}

	// Create SFTP client with optimized settings for better performance
	// Use 32KB packets (max safe size) with increased concurrency for faster transfers
	sftpClient, err := sftp.NewClient(sshClient,
		sftp.MaxPacket(32768),                  // 32KB packets (maximum safe size)
		sftp.MaxConcurrentRequestsPerFile(256), // 256 concurrent requests per file to compensate
		sftp.UseConcurrentWrites(true),         // Enable concurrent writes for better performance
	)
	if err != nil {
		sshClient.Close()
		return fmt.Errorf("SFTP client creation failed: %w", err)
	}

	s.sshClient = sshClient
	s.sftpClient = sftpClient

	return nil
}

// verifyConnection verifies the SFTP connection and remote path accessibility
// using SFTP-only operations (works with SFTP-only servers without SSH shell access)
func (s *SFTPStorage) verifyConnection() error {
	if s.sftpClient == nil {
		return fmt.Errorf("SFTP client is not initialized")
	}

	// Test connection with SFTP-native operation
	// Try to stat the remote path to verify it exists and is accessible
	_, err := s.sftpClient.Stat(s.remotePath)
	if err != nil {
		// If remote path doesn't exist, try to create it
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "no such file") {
			log.Printf("Remote path %s does not exist, attempting to create it...", s.remotePath)
			if mkdirErr := s.sftpClient.MkdirAll(s.remotePath); mkdirErr != nil {
				return fmt.Errorf("remote path %s does not exist and cannot be created: %w", s.remotePath, mkdirErr)
			}
			log.Printf("Successfully created remote path %s", s.remotePath)
			return nil
		}
		// For other errors (like permission denied), return them
		return fmt.Errorf("cannot access remote path %s: %w", s.remotePath, err)
	}

	return nil
}

// ensureConnection checks if connection is alive and reconnects if needed
// Uses SFTP-only operations to work with SFTP-only servers (no SSH shell required)
func (s *SFTPStorage) ensureConnection() error {
	// Check if connection is alive
	if s.sftpClient == nil || s.sshClient == nil {
		log.Printf("SFTP connection not established, connecting...")
		if err := s.connect(); err != nil {
			return fmt.Errorf("failed to establish connection: %w", err)
		}
		if err := s.verifyConnection(); err != nil {
			return fmt.Errorf("failed to verify connection: %w", err)
		}
		return nil
	}

	// Test connection with SFTP-native operation instead of SSH channel request
	// This works with SFTP-only servers that don't support SSH shell access
	_, err := s.sftpClient.Getwd()
	if err != nil {
		log.Printf("SFTP connection appears dead (error: %v), attempting to reconnect...", err)
		s.Close() // Close stale connections
		if err := s.connect(); err != nil {
			return fmt.Errorf("failed to reconnect: %w", err)
		}
		if err := s.verifyConnection(); err != nil {
			return fmt.Errorf("failed to verify reconnection: %w", err)
		}
		log.Printf("SFTP connection successfully reestablished")
	}

	return nil
}

// toRemotePath converts a local path to remote path (prepends remotePath and ensures forward slashes)
func (s *SFTPStorage) toRemotePath(localPath string) string {
	// Convert to forward slashes for SFTP
	localPath = filepath.ToSlash(localPath)
	// Join with remote base path
	return path.Join(s.remotePath, localPath)
}

// WriteFile writes data to a file on the SFTP server
func (s *SFTPStorage) WriteFile(filePath string, data []byte, perm fs.FileMode) error {
	if err := s.ensureConnection(); err != nil {
		return fmt.Errorf("SFTP connection error: %w", err)
	}

	remotePath := s.toRemotePath(filePath)

	// Ensure parent directory exists
	parentDir := path.Dir(remotePath)
	if err := s.sftpClient.MkdirAll(parentDir); err != nil {
		return fmt.Errorf("failed to create parent directory %s: %w", parentDir, err)
	}

	// Write file with buffering for better performance
	f, err := s.sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", remotePath, err)
	}
	defer f.Close()

	// Use buffered writer for better performance on large files
	bufWriter := bufio.NewWriterSize(f, 256*1024) // 256KB buffer

	if _, err := bufWriter.Write(data); err != nil {
		return fmt.Errorf("failed to write data to %s: %w", remotePath, err)
	}

	// Ensure all buffered data is flushed
	if err := bufWriter.Flush(); err != nil {
		return fmt.Errorf("failed to flush data to %s: %w", remotePath, err)
	}

	// Set file permissions
	if err := s.sftpClient.Chmod(remotePath, perm); err != nil {
		log.Printf("Warning: failed to set permissions on %s: %v", remotePath, err)
	}

	return nil
}

// ReadFile reads a file from the SFTP server
func (s *SFTPStorage) ReadFile(filePath string) ([]byte, error) {
	if err := s.ensureConnection(); err != nil {
		return nil, fmt.Errorf("SFTP connection error: %w", err)
	}

	remotePath := s.toRemotePath(filePath)

	f, err := s.sftpClient.Open(remotePath)
	if err != nil {
		// Return os.ErrNotExist for compatibility
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "no such file") {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("failed to open file %s: %w", remotePath, err)
	}
	defer f.Close()

	return io.ReadAll(f)
}

// ReadRange reads a range of bytes from a file
func (s *SFTPStorage) ReadRange(path string, offset int64, length int64) ([]byte, error) {
	if err := s.ensureConnection(); err != nil {
		return nil, fmt.Errorf("SFTP connection error: %w", err)
	}

	remotePath := s.toRemotePath(path)

	// Open file for reading
	file, err := s.sftpClient.Open(remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Seek to offset
	if _, err := file.Seek(offset, 0); err != nil {
		return nil, fmt.Errorf("failed to seek: %w", err)
	}

	// Read up to length bytes
	buffer := make([]byte, length)
	n, err := io.ReadFull(file, buffer)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("failed to read: %w", err)
	}

	return buffer[:n], nil
}

// MkdirAll creates directory and all parent directories on SFTP server
func (s *SFTPStorage) MkdirAll(dirPath string, perm fs.FileMode) error {
	if err := s.ensureConnection(); err != nil {
		return fmt.Errorf("SFTP connection error: %w", err)
	}

	remotePath := s.toRemotePath(dirPath)
	return s.sftpClient.MkdirAll(remotePath)
}

// Stat returns file information for a file on SFTP server
func (s *SFTPStorage) Stat(filePath string) (fs.FileInfo, error) {
	if err := s.ensureConnection(); err != nil {
		return nil, fmt.Errorf("SFTP connection error: %w", err)
	}

	remotePath := s.toRemotePath(filePath)
	info, err := s.sftpClient.Stat(remotePath)
	if err != nil {
		// Return os.ErrNotExist for compatibility
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "no such file") {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("failed to stat file %s: %w", remotePath, err)
	}
	return info, nil
}

// Remove deletes a file from the SFTP server
func (s *SFTPStorage) Remove(filePath string) error {
	if err := s.ensureConnection(); err != nil {
		return fmt.Errorf("SFTP connection error: %w", err)
	}

	remotePath := s.toRemotePath(filePath)
	if err := s.sftpClient.Remove(remotePath); err != nil {
		return fmt.Errorf("failed to remove file %s: %w", remotePath, err)
	}
	return nil
}

// Rename moves/renames a file on the SFTP server
func (s *SFTPStorage) Rename(src, dst string) error {
	if err := s.ensureConnection(); err != nil {
		return fmt.Errorf("SFTP connection error: %w", err)
	}

	remoteSrc := s.toRemotePath(src)
	remoteDst := s.toRemotePath(dst)

	// Ensure destination directory exists
	parentDir := path.Dir(remoteDst)
	if err := s.sftpClient.MkdirAll(parentDir); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", parentDir, err)
	}

	// Rename/move the file
	if err := s.sftpClient.Rename(remoteSrc, remoteDst); err != nil {
		return fmt.Errorf("failed to rename %s to %s: %w", remoteSrc, remoteDst, err)
	}

	return nil
}

// Open opens a file on the SFTP server for reading
func (s *SFTPStorage) Open(filePath string) (io.ReadCloser, error) {
	if err := s.ensureConnection(); err != nil {
		return nil, fmt.Errorf("SFTP connection error: %w", err)
	}

	remotePath := s.toRemotePath(filePath)
	f, err := s.sftpClient.Open(remotePath)
	if err != nil {
		// Return os.ErrNotExist for compatibility
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "no such file") {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("failed to open file %s: %w", remotePath, err)
	}
	return f, nil
}

// Create creates or truncates a file on the SFTP server for writing
func (s *SFTPStorage) Create(filePath string) (io.WriteCloser, error) {
	if err := s.ensureConnection(); err != nil {
		return nil, fmt.Errorf("SFTP connection error: %w", err)
	}

	remotePath := s.toRemotePath(filePath)

	// Ensure parent directory exists
	parentDir := path.Dir(remotePath)
	if err := s.sftpClient.MkdirAll(parentDir); err != nil {
		return nil, fmt.Errorf("failed to create parent directory %s: %w", parentDir, err)
	}

	f, err := s.sftpClient.Create(remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file %s: %w", remotePath, err)
	}
	return f, nil
}

// Walk walks the file tree rooted at root
func (s *SFTPStorage) Walk(root string, walkFn func(path string, info fs.FileInfo, err error) error) error {
	if err := s.ensureConnection(); err != nil {
		return fmt.Errorf("SFTP connection error: %w", err)
	}

	remotePath := s.toRemotePath(root)

	// Use SFTP's Walker for recursive directory traversal
	walker := s.sftpClient.Walk(remotePath)

	for walker.Step() {
		if err := walker.Err(); err != nil {
			// Call walkFn with the error
			if err := walkFn(walker.Path(), nil, err); err != nil {
				return err
			}
			continue
		}

		// Call walkFn for this file/directory
		if err := walkFn(walker.Path(), walker.Stat(), nil); err != nil {
			return err
		}
	}

	return nil
}

// Close closes the SFTP and SSH connections
func (s *SFTPStorage) Close() error {
	var err error
	if s.sftpClient != nil {
		if closeErr := s.sftpClient.Close(); closeErr != nil {
			err = closeErr
		}
		s.sftpClient = nil
	}
	if s.sshClient != nil {
		if closeErr := s.sshClient.Close(); closeErr != nil {
			err = closeErr
		}
		s.sshClient = nil
	}
	return err
}
