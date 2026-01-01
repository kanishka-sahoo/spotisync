package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Create a temporary config file
	content := `
server:
  host: "127.0.0.1"
  port: 9090
  env: "test"
  log_level: "debug"

database:
  path: ":memory:"
  max_open_conns: 10
  max_idle_conns: 2
  conn_max_lifetime: 1m

storage:
  music_root: "/tmp/music"
  temp_dir: "/tmp/temp"
  max_file_size: 100MB

workers:
  count: 1
  retry_max: 2
  retry_delays:
    - 30s
    - 2m

navidrome:
  host: "http://localhost:4533"
  username: "test"
  password: "test123"

websocket:
  ping_interval: 15s
  pong_timeout: 5s

rate_limit:
  requests_per_minute: 50
  burst: 10
`

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	tmpFile.Close()

	// Test loading
	cfg, err := Load(tmpFile.Name())
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify server config
	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "test", cfg.Server.Env)
	assert.Equal(t, "debug", cfg.Server.LogLevel)

	// Verify database config
	assert.Equal(t, ":memory:", cfg.Database.Path)
	assert.Equal(t, 10, cfg.Database.MaxOpenConns)
	assert.Equal(t, 2, cfg.Database.MaxIdleConns)
	assert.Equal(t, time.Minute, cfg.Database.ConnMaxLifetime)

	// Verify storage config
	assert.Equal(t, "/tmp/music", cfg.Storage.MusicRoot)
	assert.Equal(t, "/tmp/temp", cfg.Storage.TempDir)
	assert.Equal(t, ByteSize(100*1024*1024), cfg.Storage.MaxFileSize)

	// Verify workers config
	assert.Equal(t, 1, cfg.Workers.Count)
	assert.Equal(t, 2, cfg.Workers.RetryMax)
	assert.Equal(t, []time.Duration{30 * time.Second, 2 * time.Minute}, cfg.Workers.RetryDelays)

	// Verify navidrome config
	assert.Equal(t, "http://localhost:4533", cfg.Navidrome.Host)
	assert.Equal(t, "test", cfg.Navidrome.Username)
	assert.Equal(t, "test123", cfg.Navidrome.Password)

	// Verify websocket config
	assert.Equal(t, 15*time.Second, cfg.WebSocket.PingInterval)
	assert.Equal(t, 5*time.Second, cfg.WebSocket.PongTimeout)

	// Verify rate limit config
	assert.Equal(t, 50, cfg.RateLimit.RequestsPerMinute)
	assert.Equal(t, 10, cfg.RateLimit.Burst)
}

func TestLoadWithDefaults(t *testing.T) {
	// Minimal config file
	content := `
server:
  port: 9999
storage:
  music_root: "/my/music"
database:
  path: "/my/db.db"
`

	tmpFile, err := os.CreateTemp("", "config-minimal-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	require.NoError(t, err)

	// Check defaults are applied
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, "development", cfg.Server.Env)
	assert.Equal(t, "info", cfg.Server.LogLevel)
	assert.Equal(t, 25, cfg.Database.MaxOpenConns)
	assert.Equal(t, 5, cfg.Database.MaxIdleConns)
	assert.Equal(t, 5*time.Minute, cfg.Database.ConnMaxLifetime)
	assert.Equal(t, "./data/temp", cfg.Storage.TempDir)
	assert.Equal(t, ByteSize(500*1024*1024), cfg.Storage.MaxFileSize)
	assert.Equal(t, 2, cfg.Workers.Count)
	assert.Equal(t, 3, cfg.Workers.RetryMax)
	assert.Equal(t, []time.Duration{1 * time.Minute, 5 * time.Minute, 15 * time.Minute}, cfg.Workers.RetryDelays)
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestLoadInvalidYAML(t *testing.T) {
	content := `
server:
  port: [invalid yaml
`

	tmpFile, err := os.CreateTemp("", "config-invalid-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	tmpFile.Close()

	_, err = Load(tmpFile.Name())
	assert.Error(t, err)
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Server:   ServerConfig{Port: 8080},
				Storage:  StorageConfig{MusicRoot: "/music"},
				Database: DatabaseConfig{Path: "/db.db"},
			},
			wantErr: false,
		},
		{
			name: "invalid port - zero",
			cfg: Config{
				Server:   ServerConfig{Port: 0},
				Storage:  StorageConfig{MusicRoot: "/music"},
				Database: DatabaseConfig{Path: "/db.db"},
			},
			wantErr: true,
		},
		{
			name: "invalid port - too high",
			cfg: Config{
				Server:   ServerConfig{Port: 70000},
				Storage:  StorageConfig{MusicRoot: "/music"},
				Database: DatabaseConfig{Path: "/db.db"},
			},
			wantErr: true,
		},
		{
			name: "missing music root",
			cfg: Config{
				Server:   ServerConfig{Port: 8080},
				Storage:  StorageConfig{MusicRoot: ""},
				Database: DatabaseConfig{Path: "/db.db"},
			},
			wantErr: true,
		},
		{
			name: "missing database path",
			cfg: Config{
				Server:   ServerConfig{Port: 8080},
				Storage:  StorageConfig{MusicRoot: "/music"},
				Database: DatabaseConfig{Path: ""},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMaskSensitive(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Host: "localhost"},
		Tidal:  ServiceConfig{Password: "tidal_password"},
		Qobuz:  ServiceConfig{Password: "qobuz_password"},
		Navidrome: NavidromeConfig{
			Host:     "http://localhost:4533",
			Password: "navidrome_password",
		},
	}

	masked := cfg.MaskSensitive()

	assert.Equal(t, "localhost", masked.Server.Host)
	assert.Equal(t, "***", masked.Tidal.Password)
	assert.Equal(t, "***", masked.Qobuz.Password)
	assert.Equal(t, "***", masked.Navidrome.Password)

	// Original should be unchanged
	assert.Equal(t, "tidal_password", cfg.Tidal.Password)
}

func TestEnvOverrides(t *testing.T) {
	content := `
server:
  host: "localhost"
  port: 8080
storage:
  music_root: "/default/music"
database:
  path: "/default/db.db"
`

	tmpFile, err := os.CreateTemp("", "config-env-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	tmpFile.Close()

	// Set environment variables
	os.Setenv("SPOTISYNC_HOST", "0.0.0.0")
	os.Setenv("SPOTISYNC_PORT", "9000")
	os.Setenv("SPOTISYNC_MUSIC_ROOT", "/env/music")
	defer func() {
		os.Unsetenv("SPOTISYNC_HOST")
		os.Unsetenv("SPOTISYNC_PORT")
		os.Unsetenv("SPOTISYNC_MUSIC_ROOT")
	}()

	cfg, err := Load(tmpFile.Name())
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 9000, cfg.Server.Port)
	assert.Equal(t, "/env/music", cfg.Storage.MusicRoot)
}
