# Spotisync Backend Configuration Guide

This document explains the configuration options for the Spotisync backend and how to customize them.

## Configuration File Structure

The configuration is managed through a `config.yaml` file. Each section of the configuration is explained below.

### Server Configuration

The `server` section controls the basic server settings:

- `host`: The network address to bind to. Use `0.0.0.0` to listen on all interfaces or `127.0.0.1` for localhost only.
- `port`: The port number the server will listen on (default: 8080).
- `env`: The environment mode. Set to `production` for deployment or `development` for debugging.
- `log_level`: Controls the verbosity of logs. Options: `debug`, `info`, `warn`, `error`.

### Database Configuration

The `database` section configures the SQLite database connection:

- `path`: The path to the SQLite database file. The directory will be created if it doesn't exist.
- `max_open_conns`: Maximum number of open connections to the database.
- `max_idle_conns`: Maximum number of idle connections in the connection pool.
- `conn_max_lifetime`: Maximum lifetime of a database connection (e.g., `5m` for 5 minutes).

### Storage Configuration

The `storage` section defines music file storage locations:

- `music_root`: Root directory for the music library.
- `temp_dir`: Temporary directory for processing files.
- `max_file_size`: Maximum size for music files (e.g., `500MB`).

### Workers Configuration

The `workers` section controls background job processing:

- `count`: Number of worker threads for processing tasks.
- `retry_max`: Maximum number of retries for failed tasks.
- `retry_delays`: Delay intervals between retries (e.g., `1m`, `5m`, `15m`).

### Service Credentials

The `spotify`, `tidal`, and `qobuz` sections are optional and allow configuring server-level credentials for music streaming services. If these are left empty, users can configure their own credentials through the UI.

### Navidrome Integration

The `navidrome` section configures connection to a Navidrome server for music scanning:

- `host`: URL of the Navidrome server.
- `username`: Admin username for Navidrome.
- `password`: Admin password for Navidrome. **Change this password immediately after deployment.**

### WebSocket Configuration

The `websocket` section controls real-time communication:

- `ping_interval`: How often to send ping messages to clients (e.g., `30s`).
- `pong_timeout`: Time to wait for a pong response (e.g., `10s`).

### Rate Limiting Configuration

The `rate_limit` section controls API request limits:

- `requests_per_minute`: Maximum number of requests allowed per minute.
- `burst`: Maximum number of requests allowed in a short burst.

## Overriding Configuration with Environment Variables

All configuration settings can be overridden using environment variables. Use the following naming convention:

```
SPOTISYNC_<SECTION>_<KEY>
```

Where `<SECTION>` is the uppercase name of the configuration section and `<KEY>` is the uppercase name of the setting.

For example:

```bash
# Override server port
export SPOTISYNC_SERVER_PORT=8080

# Override database path
export SPOTISYNC_DATABASE_PATH="./data/spotisync.db"

# Override Navidrome password
export SPOTISYNC_NAVIDROME_PASSWORD="your_secure_password"
```

### Special Handling for Lists

For settings that accept lists (like `retry_delays`), use comma-separated values:

```bash
# Set retry delays to 1 minute, 3 minutes, and 10 minutes
export SPOTISYNC_WORKERS_RETRY_DELAYS="1m,3m,10m"
```

## Security Notes

1. **File Permissions**: The `config.yaml` file may contain sensitive information such as passwords and API keys. Set the file permissions to `600` to prevent unauthorized access:

   ```bash
   chmod 600 config.yaml
   ```

2. **Environment Variables**: For production deployments, consider using environment variables for sensitive settings instead of storing them in the configuration file.

3. **Default Passwords**: Change all default passwords immediately after deployment, especially:
   - Navidrome admin password
   - Any service credentials

4. **Secrets Management**: For larger deployments, consider using a secrets management tool like HashiCorp Vault or Kubernetes Secrets.

## Directory Structure

Ensure the following directories exist or will be created:

```
backend/
├── config.yaml        # Main configuration file
├── data/              # Database and temporary files
│   ├── spotisync.db
│   └── temp/
└── music/             # Music library root
```

## Troubleshooting

- If the server fails to start, check the logs for configuration errors.
- Ensure all required directories exist and are writable by the server process.
- Verify that database connections are not exceeding the maximum allowed connections.
