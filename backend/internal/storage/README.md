# Storage Backend Architecture

## Overview

Spotisync now supports multiple storage backends for downloading music files:
- **Local Storage**: Files are stored on the local filesystem (default)
- **SFTP Storage**: Files are stored directly on a remote SFTP server

This abstraction allows Spotisync to run anywhere while storing music files on a different machine, useful for scenarios like:
- Running Spotisync in a container but storing music on a NAS
- Running Spotisync on a lightweight VPS while storing files on a home server
- Centralizing music storage across multiple Spotisync instances

## Architecture

The storage implementation follows an interface-based design pattern:

```
storage.Storage (interface)
    ├── LocalStorage (local filesystem)
    └── SFTPStorage (remote SFTP server)
```

All file operations in the download orchestrator go through the `Storage` interface, making it transparent whether files are stored locally or remotely.

## Configuration

### Environment Variables

```bash
# Storage backend type: "local" (default) or "sftp"
STORAGE_TYPE=sftp

# SFTP Configuration (only used when STORAGE_TYPE=sftp)
SFTP_HOST=sftp.example.com
SFTP_PORT=22
SFTP_USERNAME=your_username
SFTP_PASSWORD=your_password  # Optional if using SSH key
SFTP_SSH_KEY_PATH=/path/to/private_key  # Optional if using password
SFTP_REMOTE_PATH=/path/to/remote/music
```

### config.yaml

```yaml
storage:
  type: sftp  # "local" or "sftp"
  music_root: /data/music
  temp_dir: /data/temp
  max_file_size: 500MB
  sftp:
    host: sftp.example.com
    port: 22
    username: your_username
    password: your_password  # Optional
    ssh_key_path: /path/to/private_key  # Optional
    remote_path: /media/music
```

## Implementation Details

### Local Storage

- Uses Go's standard `os` package for file operations
- No network overhead
- Direct filesystem access
- Fastest performance

### SFTP Storage

- Maintains a persistent SSH/SFTP connection
- Automatically reconnects if connection drops
- Supports both password and SSH key authentication
- All paths use forward slashes (SFTP standard)
- Connection pooling for better performance

### Performance Considerations

1. **Temp Directory**: Downloads always use local temp directory for performance
   - Tidal/Qobuz downloads to local temp
   - File is then uploaded to final storage backend
   
2. **Metadata Operations**: FLAC metadata embedding requires local files
   - For SFTP: downloads file, modifies locally, uploads back
   - Adds overhead but necessary for proper tagging
   
3. **ISRC Duplicate Detection**: 
   - Builds index at startup (background task)
   - For SFTP: requires downloading files to read metadata
   - Index is cached in memory for fast lookups

## Usage Example

### Local Storage (Default)

```bash
# No configuration needed, uses local filesystem by default
MUSIC_LIBRARY_PATH=./music
```

### SFTP Storage with Password

```bash
STORAGE_TYPE=sftp
SFTP_HOST=192.168.1.100
SFTP_PORT=22
SFTP_USERNAME=music
SFTP_PASSWORD=securepassword
SFTP_REMOTE_PATH=/home/music/library
```

### SFTP Storage with SSH Key

```bash
STORAGE_TYPE=sftp
SFTP_HOST=nas.local
SFTP_PORT=22
SFTP_USERNAME=spotisync
SFTP_SSH_KEY_PATH=/app/ssh/id_rsa
SFTP_REMOTE_PATH=/volume1/music
```

## Error Handling

The storage implementations provide comprehensive error handling:

- **Connection Errors**: SFTP automatically reconnects on connection loss
- **Authentication Errors**: Clear error messages distinguish auth failures
- **Permission Errors**: Detailed errors for filesystem permission issues
- **Network Errors**: Retries on transient network failures

## Testing

Run storage tests:

```bash
cd backend
go test ./internal/storage -v
```

## Security Considerations

1. **SSH Key Authentication**: Preferred over password authentication
2. **Host Key Verification**: Currently disabled (uses `InsecureIgnoreHostKey`)
   - TODO: Add proper host key verification for production
3. **Password Storage**: Passwords are masked in config output
4. **Connection Security**: All SFTP traffic is encrypted via SSH

## Migration Guide

### Switching from Local to SFTP

1. Set up SFTP server and create user account
2. Update configuration with SFTP settings
3. Restart Spotisync
4. Existing local files remain accessible
5. New downloads go to SFTP server

### Switching from SFTP to Local

1. Change `STORAGE_TYPE` to `local`
2. Restart Spotisync
3. Files on SFTP remain but won't be accessed
4. New downloads go to local storage

## Limitations

1. **Mixed Storage**: Cannot use both local and SFTP simultaneously
2. **Migration**: No automatic migration between storage types
3. **Host Key Verification**: Currently disabled (security consideration)
4. **Bandwidth**: SFTP adds network overhead compared to local storage
5. **Metadata Operations**: Multiple round-trips for SFTP (download, modify, upload)

## Future Improvements

- [ ] Add host key verification for SFTP
- [ ] Support for S3-compatible object storage
- [ ] Support for multiple SFTP servers (sharding)
- [ ] Caching layer for frequently accessed files
- [ ] Async metadata operations to reduce latency
- [ ] Storage migration tool
