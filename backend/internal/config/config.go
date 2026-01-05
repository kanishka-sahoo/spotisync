package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main application configuration
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Storage   StorageConfig   `yaml:"storage"`
	Workers   WorkersConfig   `yaml:"workers"`
	Spotify   ServiceConfig   `yaml:"spotify"`
	Tidal     ServiceConfig   `yaml:"tidal"`
	Qobuz     ServiceConfig   `yaml:"qobuz"`
	Sources   SourcesConfig   `yaml:"sources"`
	Navidrome NavidromeConfig `yaml:"navidrome"`
	WebSocket WebSocketConfig `yaml:"websocket"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Host           string        `yaml:"host"`
	Port           int           `yaml:"port"`
	Env            string        `yaml:"env"`
	LogLevel       string        `yaml:"log_level"`
	SecretKey      string        `yaml:"secret_key"`
	TokenTTL       time.Duration `yaml:"token_ttl"`
	AllowedOrigins []string      `yaml:"allowed_origins"`
}

// DatabaseConfig holds SQLite database settings
type DatabaseConfig struct {
	Path            string        `yaml:"path"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

// StorageConfig holds file storage settings
type StorageConfig struct {
	MusicRoot   string     `yaml:"music_root"`
	TempDir     string     `yaml:"temp_dir"`
	MaxFileSize ByteSize   `yaml:"max_file_size"`
	Type        string     `yaml:"type"` // "local" or "sftp"
	SFTP        SFTPConfig `yaml:"sftp"`
}

// SFTPConfig holds SFTP connection settings
type SFTPConfig struct {
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	SSHKeyPath string `yaml:"ssh_key_path"`
	RemotePath string `yaml:"remote_path"`
}

// ByteSize represents a size in bytes with human-readable format support
type ByteSize int64

// UnmarshalYAML implements yaml.Unmarshaler for ByteSize
func (b *ByteSize) UnmarshalYAML(value *yaml.Node) error {
	var str string
	if err := value.Decode(&str); err != nil {
		return err
	}

	// Parse human-readable format (e.g., "100MB", "1GB")
	multiplier := int64(1)
	str = strings.TrimSpace(str)

	if strings.HasSuffix(str, "KB") || strings.HasSuffix(str, "kb") {
		multiplier = 1024
		str = strings.TrimSuffix(str, "KB")
		str = strings.TrimSuffix(str, "kb")
	} else if strings.HasSuffix(str, "MB") || strings.HasSuffix(str, "mb") {
		multiplier = 1024 * 1024
		str = strings.TrimSuffix(str, "MB")
		str = strings.TrimSuffix(str, "mb")
	} else if strings.HasSuffix(str, "GB") || strings.HasSuffix(str, "gb") {
		multiplier = 1024 * 1024 * 1024
		str = strings.TrimSuffix(str, "GB")
		str = strings.TrimSuffix(str, "gb")
	} else if strings.HasSuffix(str, "TB") || strings.HasSuffix(str, "tb") {
		multiplier = 1024 * 1024 * 1024 * 1024
		str = strings.TrimSuffix(str, "TB")
		str = strings.TrimSuffix(str, "tb")
	}

	str = strings.TrimSpace(str)
	val, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid byte size: %s", str)
	}

	*b = ByteSize(val * multiplier)
	return nil
}

// WorkersConfig holds worker concurrency settings
type WorkersConfig struct {
	Count       int             `yaml:"count"`
	RetryMax    int             `yaml:"retry_max"`
	RetryDelays []time.Duration `yaml:"retry_delays"`
}

// ServiceConfig holds streaming service credentials (deprecated, use TidalConfig/QobuzConfig)
type ServiceConfig struct {
	// Credentials can be configured at server level
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	// Additional service-specific settings
}

// TidalConfig holds Tidal-specific credentials and settings
type TidalConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	Quality      string `yaml:"quality"` // HI_RES, HI_RES_LOSSLESS, LOSSLESS, HIGH, LOW
}

// QobuzConfig holds Qobuz-specific credentials and settings
type QobuzConfig struct {
	AppID   string `yaml:"app_id"`
	Secret  string `yaml:"secret"`
	Quality string `yaml:"quality"` // FLAC24, FLAC16, MP3
}

// SourcesConfig holds music source credentials and preferences
type SourcesConfig struct {
	Tidal           TidalConfig `yaml:"tidal"`
	Qobuz           QobuzConfig `yaml:"qobuz"`
	PreferredSource string      `yaml:"preferred_source"` // "tidal" or "qobuz"
}

// NavidromeConfig holds Navidrome integration settings
type NavidromeConfig struct {
	Host     string `yaml:"host"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// WebSocketConfig holds WebSocket settings
type WebSocketConfig struct {
	PingInterval time.Duration `yaml:"ping_interval"`
	PongTimeout  time.Duration `yaml:"pong_timeout"`
}

// RateLimitConfig holds rate limiting settings
type RateLimitConfig struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
	Burst             int `yaml:"burst"`
}

// Load reads the configuration from a YAML file, or falls back to environment variables
func Load(path string) (*Config, error) {
	// Try to load from YAML file first (backward compatible)
	data, err := os.ReadFile(path)
	if err != nil {
		// If YAML file doesn't exist, load purely from environment variables
		if os.IsNotExist(err) {
			return LoadFromEnv()
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides
	cfg.applyEnvOverrides()

	// Set defaults
	cfg.setDefaults()

	// Generate a secure random secret key in development if not set
	if cfg.Server.SecretKey == "" && cfg.Server.Env != "production" {
		cfg.Server.SecretKey = generateRandomSecretKey()
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &cfg, nil
}

// LoadFromEnv creates configuration purely from environment variables with sensible defaults
// This enables Docker deployment without requiring a config.yaml file
func LoadFromEnv() (*Config, error) {
	cfg := &Config{}

	// Set Docker-friendly defaults first
	cfg.setDockerDefaults()

	// Apply all environment variable overrides
	cfg.applyEnvOverrides()

	// Generate a secure random secret key in development if not set
	if cfg.Server.SecretKey == "" && cfg.Server.Env != "production" {
		cfg.Server.SecretKey = generateRandomSecretKey()
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

// setDockerDefaults sets defaults optimized for Docker deployment
func (c *Config) setDockerDefaults() {
	// Server defaults (Docker-optimized)
	c.Server.Host = "0.0.0.0"
	c.Server.Port = 8080
	c.Server.Env = "production"
	c.Server.LogLevel = "info"
	c.Server.TokenTTL = 24 * time.Hour
	c.Server.AllowedOrigins = []string{"*"}

	// Database defaults (Docker volume paths)
	c.Database.Path = "/data/db/spotisync.db"
	c.Database.MaxOpenConns = 25
	c.Database.MaxIdleConns = 5
	c.Database.ConnMaxLifetime = 5 * time.Minute

	// Storage defaults (Docker volume paths)
	c.Storage.MusicRoot = "/data/music"
	c.Storage.TempDir = "/data/temp"
	c.Storage.MaxFileSize = ByteSize(500 * 1024 * 1024) // 500MB
	c.Storage.Type = "local"                            // Default to local storage
	c.Storage.SFTP.Port = 22                            // Default SFTP port

	// Workers defaults
	c.Workers.Count = 2
	c.Workers.RetryMax = 3
	c.Workers.RetryDelays = []time.Duration{
		1 * time.Minute,
		5 * time.Minute,
		15 * time.Minute,
	}

	// Sources defaults
	c.Sources.Tidal.Quality = "HI_RES"
	c.Sources.Qobuz.Quality = "FLAC24"
	c.Sources.PreferredSource = "tidal"

	// WebSocket defaults
	c.WebSocket.PingInterval = 30 * time.Second
	c.WebSocket.PongTimeout = 10 * time.Second

	// Rate limit defaults
	c.RateLimit.RequestsPerMinute = 100
	c.RateLimit.Burst = 20
}

// generateRandomSecretKey generates a cryptographically random secret key
func generateRandomSecretKey() string {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a time-based key if crypto random fails (should never happen)
		return fmt.Sprintf("dev-secret-key-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// applyEnvOverrides applies environment variable overrides to config
func (c *Config) applyEnvOverrides() {
	// Server overrides
	if v := os.Getenv("SPOTISYNC_HOST"); v != "" {
		c.Server.Host = v
	}
	if v := os.Getenv("SPOTISYNC_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &c.Server.Port)
	}
	if v := os.Getenv("SPOTISYNC_ENV"); v != "" {
		c.Server.Env = v
	}
	if v := os.Getenv("SPOTISYNC_LOG_LEVEL"); v != "" {
		c.Server.LogLevel = v
	}
	if v := os.Getenv("SPOTISYNC_SECRET_KEY"); v != "" {
		c.Server.SecretKey = v
	}
	if v := os.Getenv("SPOTISYNC_ALLOWED_ORIGINS"); v != "" {
		c.Server.AllowedOrigins = strings.Split(v, ",")
	}

	// Database overrides
	if v := os.Getenv("SPOTISYNC_DB_PATH"); v != "" {
		c.Database.Path = v
	}

	// Storage overrides
	if v := os.Getenv("SPOTISYNC_MUSIC_ROOT"); v != "" {
		c.Storage.MusicRoot = v
	}
	if v := os.Getenv("SPOTISYNC_TEMP_DIR"); v != "" {
		c.Storage.TempDir = v
	}
	if v := os.Getenv("STORAGE_TYPE"); v != "" {
		c.Storage.Type = v
	}

	// SFTP overrides
	if v := os.Getenv("SFTP_HOST"); v != "" {
		c.Storage.SFTP.Host = v
	}
	if v := os.Getenv("SFTP_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &c.Storage.SFTP.Port)
	}
	if v := os.Getenv("SFTP_USERNAME"); v != "" {
		c.Storage.SFTP.Username = v
	}
	if v := os.Getenv("SFTP_PASSWORD"); v != "" {
		c.Storage.SFTP.Password = v
	}
	if v := os.Getenv("SFTP_SSH_KEY_PATH"); v != "" {
		c.Storage.SFTP.SSHKeyPath = v
	}
	if v := os.Getenv("SFTP_REMOTE_PATH"); v != "" {
		c.Storage.SFTP.RemotePath = v
	}

	// Workers overrides
	if v := os.Getenv("SPOTISYNC_WORKERS"); v != "" {
		fmt.Sscanf(v, "%d", &c.Workers.Count)
	}

	// Navidrome overrides
	if v := os.Getenv("SPOTISYNC_NAVIDROME_HOST"); v != "" {
		c.Navidrome.Host = v
	}
	if v := os.Getenv("SPOTISYNC_NAVIDROME_USER"); v != "" {
		c.Navidrome.Username = v
	}
	if v := os.Getenv("SPOTISYNC_NAVIDROME_PASS"); v != "" {
		c.Navidrome.Password = v
	}

	// Spotify credentials (for reference/usage by the app)
	if v := os.Getenv("SPOTISYNC_SPOTIFY_CLIENT_ID"); v != "" {
		// Spotify credentials are typically handled separately,
		// but we can store them for reference
		c.Spotify.Username = v // Using Username field for client_id
	}
	if v := os.Getenv("SPOTISYNC_SPOTIFY_CLIENT_SECRET"); v != "" {
		c.Spotify.Password = v // Using Password field for client_secret
	}

	// Sources overrides (Tidal)
	if v := os.Getenv("SPOTISYNC_TIDAL_CLIENT_ID"); v != "" {
		c.Sources.Tidal.ClientID = v
	}
	if v := os.Getenv("SPOTISYNC_TIDAL_CLIENT_SECRET"); v != "" {
		c.Sources.Tidal.ClientSecret = v
	}
	if v := os.Getenv("SPOTISYNC_TIDAL_QUALITY"); v != "" {
		c.Sources.Tidal.Quality = v
	}

	// Sources overrides (Qobuz)
	if v := os.Getenv("SPOTISYNC_QOBUZ_APP_ID"); v != "" {
		c.Sources.Qobuz.AppID = v
	}
	if v := os.Getenv("SPOTISYNC_QOBUZ_SECRET"); v != "" {
		c.Sources.Qobuz.Secret = v
	}
	if v := os.Getenv("SPOTISYNC_QOBUZ_QUALITY"); v != "" {
		c.Sources.Qobuz.Quality = v
	}

	// Sources preferred source override
	if v := os.Getenv("SPOTISYNC_PREFERRED_SOURCE"); v != "" {
		c.Sources.PreferredSource = v
	}
}

// setDefaults sets default values for config options
func (c *Config) setDefaults() {
	// Server defaults
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.Env == "" {
		c.Server.Env = "development"
	}
	if c.Server.LogLevel == "" {
		c.Server.LogLevel = "info"
	}
	if c.Server.TokenTTL == 0 {
		c.Server.TokenTTL = 24 * time.Hour
	}
	// Note: SecretKey is intentionally left empty to require explicit configuration
	// In production, it must be set via config or environment variable

	// Database defaults
	if c.Database.Path == "" {
		c.Database.Path = "./data/spotisync.db"
	}
	if c.Database.MaxOpenConns == 0 {
		c.Database.MaxOpenConns = 25
	}
	if c.Database.MaxIdleConns == 0 {
		c.Database.MaxIdleConns = 5
	}
	if c.Database.ConnMaxLifetime == 0 {
		c.Database.ConnMaxLifetime = 5 * time.Minute
	}

	// Storage defaults
	if c.Storage.MusicRoot == "" {
		c.Storage.MusicRoot = "./music"
	}
	if c.Storage.TempDir == "" {
		c.Storage.TempDir = "./data/temp"
	}
	if c.Storage.MaxFileSize == 0 {
		c.Storage.MaxFileSize = ByteSize(500 * 1024 * 1024) // 500MB
	}
	if c.Storage.Type == "" {
		c.Storage.Type = "local" // Default to local storage
	}
	if c.Storage.SFTP.Port == 0 {
		c.Storage.SFTP.Port = 22 // Default SFTP port
	}

	// Workers defaults
	if c.Workers.Count == 0 {
		c.Workers.Count = 2
	}
	if c.Workers.RetryMax == 0 {
		c.Workers.RetryMax = 3
	}
	if len(c.Workers.RetryDelays) == 0 {
		c.Workers.RetryDelays = []time.Duration{
			1 * time.Minute,
			5 * time.Minute,
			15 * time.Minute,
		}
	}

	// Sources defaults
	if c.Sources.Tidal.Quality == "" {
		c.Sources.Tidal.Quality = "HI_RES"
	}
	if c.Sources.Qobuz.Quality == "" {
		c.Sources.Qobuz.Quality = "FLAC24"
	}
	if c.Sources.PreferredSource == "" {
		c.Sources.PreferredSource = "tidal"
	}

	// WebSocket defaults
	if c.WebSocket.PingInterval == 0 {
		c.WebSocket.PingInterval = 30 * time.Second
	}
	if c.WebSocket.PongTimeout == 0 {
		c.WebSocket.PongTimeout = 10 * time.Second
	}

	// Rate limit defaults
	if c.RateLimit.RequestsPerMinute == 0 {
		c.RateLimit.RequestsPerMinute = 100
	}
	if c.RateLimit.Burst == 0 {
		c.RateLimit.Burst = 20
	}
}

// Validate checks that all required configuration values are present and valid
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	if c.Storage.MusicRoot == "" {
		return fmt.Errorf("music_root is required")
	}
	if c.Database.Path == "" {
		return fmt.Errorf("database path is required")
	}

	// Validate secret key is set in production
	if c.Server.Env == "production" && c.Server.SecretKey == "" {
		return fmt.Errorf("secret_key is required in production mode. Set it via config file or SPOTISYNC_SECRET_KEY environment variable")
	}

	// Validate allowed origins in production
	if c.Server.Env == "production" && len(c.Server.AllowedOrigins) == 0 {
		return fmt.Errorf("allowed_origins is required in production mode. Set it via config file or SPOTISYNC_ALLOWED_ORIGINS environment variable")
	}

	// Validate storage configuration
	if c.Storage.Type != "" && c.Storage.Type != "local" && c.Storage.Type != "sftp" {
		return fmt.Errorf("invalid storage type: %s (must be 'local' or 'sftp')", c.Storage.Type)
	}

	// Validate SFTP configuration if SFTP storage is selected
	if c.Storage.Type == "sftp" {
		if c.Storage.SFTP.Host == "" {
			return fmt.Errorf("SFTP host is required when storage type is 'sftp'")
		}
		if c.Storage.SFTP.Port <= 0 || c.Storage.SFTP.Port > 65535 {
			return fmt.Errorf("invalid SFTP port: %d", c.Storage.SFTP.Port)
		}
		if c.Storage.SFTP.Username == "" {
			return fmt.Errorf("SFTP username is required when storage type is 'sftp'")
		}
		if c.Storage.SFTP.Password == "" && c.Storage.SFTP.SSHKeyPath == "" {
			return fmt.Errorf("either SFTP password or SSH key path is required when storage type is 'sftp'")
		}
		if c.Storage.SFTP.RemotePath == "" {
			return fmt.Errorf("SFTP remote path is required when storage type is 'sftp'")
		}
	}

	return nil
}

// MaskSensitive returns a copy of the config with sensitive values masked
func (c *Config) MaskSensitive() *Config {
	masked := *c
	masked.Tidal.Password = "***"
	masked.Qobuz.Password = "***"
	masked.Sources.Tidal.ClientSecret = "***"
	masked.Sources.Qobuz.Secret = "***"
	masked.Navidrome.Password = "***"
	masked.Storage.SFTP.Password = "***"
	return &masked
}
