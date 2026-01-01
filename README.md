# 🎵 Spotisync

**Self-hosted FLAC music downloader that matches Spotify tracks to Tidal/Qobuz for high-quality downloads.**

## Features

- **No Credentials Needed** — Uses third-party APIs (8 for Tidal, 2 for Qobuz) by default
- **Hi-Res FLAC Downloads** — Lossless audio from Tidal and Qobuz
- **Full Spotify Support** — Tracks, albums, playlists
- **Complete Metadata** — Title, Artist, Album, AlbumArtist, Date, TrackNumber, TotalTracks, DiscNumber, TotalDiscs, ISRC, Genre, Copyright, Label, Explicit, Composer, Conductor, and more
- **Synced Lyrics** — Fetched from LRCLIB, embedded in FLAC and saved as `.lrc` files
- **Cover Art** — High-quality artwork from MusicBrainz, embedded and saved as `cover.jpg`
- **Docker Ready** — Full docker-compose setup for easy deployment
- **Modern Web UI** — Next.js frontend for managing downloads
- **Real-time Updates** — WebSocket support for live job progress
- **Multi-user Support** — JWT authentication for multiple users

## Quick Start (Docker)

### 1. Clone and Configure

```bash
git clone https://github.com/yourusername/spotisync.git
cd spotisync

# Copy the example config
cp .env.example .env
```

### 2. Edit Configuration

Open `.env` and set the required values:

```bash
# Generate a secret key
openssl rand -hex 32

# Edit .env with your values
nano .env
```

**Required settings:**
- `SPOTISYNC_SECRET_KEY` — JWT secret (use the generated key above)

**Optional but recommended:**
- `MUSIC_LIBRARY_PATH` — Where to save your music (defaults to `./music`)

### 3. Deploy

```bash
docker-compose up -d
```

### 4. Access

Open the url defined by `PUBLIC_URL` (defaults to `http://localhost:3000`) and create an account to start downloading!

## Configuration

All configuration is done via a single `.env` file. Copy `.env.example` to `.env` and customize:

```bash
cp .env.example .env
```

### Required Variables

| Variable | Description |
|----------|-------------|
| `SPOTISYNC_SECRET_KEY` | Secret key for JWT token signing (generate with `openssl rand -hex 32`) |

### Optional Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MUSIC_LIBRARY_PATH` | `./music` | Directory where downloaded music is saved |
| `SPOTISYNC_PORT` | `8080` | Backend API port |
| `FRONTEND_PORT` | `3000` | Frontend web UI port |
| `PUBLIC_URL` | `http://backend:8080` | Public API URL (for reverse proxy setups) |
| `SPOTISYNC_WORKERS` | `2` | Number of concurrent download workers |
| `SPOTISYNC_LOG_LEVEL` | `info` | Log level: debug, info, warn, error |
| `SPOTIFY_CLIENT_ID` | Built-in | Custom Spotify application Client ID |
| `SPOTIFY_CLIENT_SECRET` | Built-in | Custom Spotify application Client Secret |

> **Note:** Spotify credentials are optional. Spotisync includes built-in default credentials that work out of the box. Only set custom credentials if you want to use your own Spotify app.

### Navidrome Integration (Optional)

| Variable | Description |
|----------|-------------|
| `NAVIDROME_HOST` | Navidrome server URL (e.g., `http://navidrome:4533`) |
| `NAVIDROME_USER` | Navidrome admin username |
| `NAVIDROME_PASS` | Navidrome admin password |

### Official API Fallback (Optional)

By default, Spotisync uses third-party APIs that require no credentials. You can optionally configure official API credentials:

| Variable | Description |
|----------|-------------|
| `TIDAL_CLIENT_ID` | Official Tidal API client ID |
| `TIDAL_CLIENT_SECRET` | Official Tidal API client secret |
| `QOBUZ_APP_ID` | Official Qobuz API app ID |
| `QOBUZ_SECRET` | Official Qobuz API secret |

## Development Setup

### Backend (Go)

```bash
cd backend

# Install dependencies
go mod download

# Run the server
go run ./cmd/server
```

### Frontend (Next.js)

```bash
cd frontend

# Install dependencies
npm install

# Run development server
npm run dev
```

The backend runs on `http://localhost:8080` and frontend on `http://localhost:3000`.

## API Endpoints

### Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/auth/register` | Register a new user |
| `POST` | `/api/v1/auth/login` | Login and receive JWT token |

### Downloads

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/preview` | Preview tracks from a Spotify URL |
| `POST` | `/api/v1/jobs` | Create a download batch job |
| `GET` | `/api/v1/jobs` | List all jobs for the user |

### Settings

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/settings/*` | Retrieve user settings |

### Real-time Updates

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/ws` | WebSocket connection for live job updates |

## How It Works

```
┌─────────────────────────────────────────────────────────────────┐
│                         Spotisync Flow                          │
└─────────────────────────────────────────────────────────────────┘

    User provides a Spotify URL (track/album/playlist/artist)
                                 │
                                 ▼
          Backend fetches track metadata from Spotify API
       Extracts ISRC (International Standard Recording Code)
                                 │
                                 ▼
      Backend searches Tidal/Qobuz for matching tracks by ISRC
             Uses third-party APIs (no auth required)
                                 │
                                 ▼
         FLAC audio is downloaded from the matched source
                                 │
                                 ▼
             Metadata + Lyrics + Cover Art are embedded
              Lyrics from LRCLIB (embedded + .lrc file)
           Cover art from MusicBrainz (embedded + cover.jpg)
                                 │
                                 ▼
       File is saved to configured music library path
```

## Navidrome Integration

Spotisync integrates seamlessly with [Navidrome](https://www.navidrome.org/) for automatic library management:

### Features
- **Automatic Library Scanning** — After downloads complete, Spotisync triggers a Navidrome library scan
- **Playlist Sync** — Spotify playlists are automatically created as Navidrome playlists
- **Smart Duplicate Detection** — Tracks already in your library (by ISRC) are automatically skipped

### Configuration

Add these variables to your `.env` file:

```bash
# Navidrome Integration
NAVIDROME_HOST=http://localhost:4533  # Your Navidrome URL
NAVIDROME_USER=admin                   # Admin username for library scans
NAVIDROME_PASS=your-password           # Admin password
```

> **Note:** Admin credentials are used for library scan operations. User-specific credentials (configured in the UI) are used for playlist creation.


## Tech Stack

### Backend
- **Language:** Go 1.22+
- **Framework:** Gin / Chi
- **Database:** SQLite
- **Audio Processing:** FFmpeg

### Frontend
- **Framework:** Next.js 14+
- **UI Library:** React 18
- **Styling:** TailwindCSS
- **State Management:** Zustand

### Deployment
- **Containerization:** Docker
- **Orchestration:** Docker Compose

## Supported Spotify URLs

Spotisync accepts various Spotify URL formats:

```
# Single Track
https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh

# Album
https://open.spotify.com/album/1DFixLWuPkv3KT3TnV35m3

# Playlist
https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M
```

## License

This project is licensed under the **MIT License** — see the [LICENSE](LICENSE) file for details.
For other open-source licenses see `licenses/` folder


## Useful Links and Acknowledgements
- **[SpotiFLAC](https://github.com/afkarxyz/SpotiFLAC)** - The inspiration for this project, and source for some of the code
- **[LRCLIB](https://lrclib.net/)** — Synced lyrics database
- **[MusicBrainz](https://musicbrainz.org/)** — Cover art and music metadata
For licenses see `licenses/` folder

## Disclaimer

This software is provided for educational and personal use only. Please respect copyright laws and the terms of service of the platforms involved. The developers are not responsible for any misuse of this software.

---

<p align="center">
  Made with ❤️ for music enthusiasts
</p>
