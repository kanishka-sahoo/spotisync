# SpotiSync Development Guide

This document provides architectural documentation for contributors working on SpotiSync.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Backend Architecture](#backend-architecture)
3. [Download Flow](#download-flow)
4. [Third-Party APIs](#third-party-apis)
5. [Key Files Reference](#key-files-reference)
6. [Frontend Architecture](#frontend-architecture)
7. [Environment Variables](#environment-variables)
8. [Testing](#testing)
9. [Adding New Features](#adding-new-features)
10. [Code Style](#code-style)

---

## Architecture Overview

SpotiSync is a full-stack application for downloading high-quality music from streaming services. The architecture consists of:

### Technology Stack

| Layer | Technology |
|-------|------------|
| **Backend** | Go with Gin/Chi router |
| **Database** | SQLite (embedded, zero-config) |
| **Job Processing** | Custom scheduler with worker pool |
| **Frontend** | Next.js 14 with App Router |
| **UI Framework** | React 18 + TailwindCSS |
| **State Management** | Zustand |
| **Real-time Updates** | WebSocket |
| **Deployment** | Docker / Docker Compose |

### High-Level Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│                 │     │                 │     │                 │
│  Next.js        │────▶│  Go Backend     │────▶│  SQLite         │
│  Frontend       │     │  (Gin/Chi)      │     │  Database       │
│                 │◀────│                 │     │                 │
└─────────────────┘     └────────┬────────┘     └─────────────────┘
        │                        │
        │ WebSocket              │
        ▼                        ▼
┌─────────────────┐     ┌─────────────────┐
│  Real-time      │     │  Job Scheduler  │
│  Updates        │     │  + Workers      │
└─────────────────┘     └────────┬────────┘
                                 │
                    ┌────────────┼────────────┐
                    ▼            ▼            ▼
             ┌──────────┐ ┌──────────┐ ┌──────────┐
             │ Spotify  │ │  Tidal   │ │  Qobuz   │
             │   API    │ │   API    │ │   API    │
             └──────────┘ └──────────┘ └──────────┘
```

---

## Backend Architecture

The backend follows a clean architecture pattern with clear separation of concerns:

```
backend/
├── cmd/server/main.go       # Entry point, initializes all components
├── internal/
│   ├── api/                 # HTTP routes & middleware
│   │   ├── server.go        # Route definitions & server setup
│   │   ├── handlers/        # HTTP request handlers
│   │   │   ├── auth.go      # Login, register, refresh token
│   │   │   ├── jobs.go      # Job CRUD operations
│   │   │   ├── batches.go   # Batch operations
│   │   │   └── health.go    # Health check endpoint
│   │   └── middleware/      # Request middleware
│   │       ├── auth.go      # JWT authentication
│   │       ├── cors.go      # CORS handling
│   │       └── ratelimit.go # Rate limiting
│   │
│   ├── auth/                # JWT authentication
│   │   ├── jwt.go           # Token generation & validation
│   │   └── password.go      # Password hashing (bcrypt)
│   │
│   ├── config/              # Configuration loading
│   │   └── config.go        # YAML & env var parsing
│   │
│   ├── db/                  # SQLite database layer
│   │   ├── db.go            # Database connection & setup
│   │   ├── models/          # Data models
│   │   │   ├── user.go      # User model
│   │   │   ├── job.go       # Job model (individual track)
│   │   │   └── batch.go     # Batch model (album/playlist)
│   │   ├── migrations/      # Schema migrations
│   │   │   └── 001_init.sql # Initial schema
│   │   └── queries/         # SQL query functions
│   │       ├── users.go     # User queries
│   │       ├── jobs.go      # Job queries
│   │       └── batches.go   # Batch queries
│   │
│   ├── scheduler/           # Background job processing
│   │   ├── scheduler.go     # Job queue management
│   │   └── worker.go        # Worker pool implementation
│   │
│   ├── services/            # Business logic layer
│   │   ├── cover/           # Cover art fetching
│   │   │   └── musicbrainz.go  # MusicBrainz API client
│   │   ├── download/        # Download orchestration
│   │   │   └── orchestrator.go # Main download workflow
│   │   ├── lyrics/          # Lyrics fetching
│   │   │   └── lrclib.go    # LRCLIB API client
│   │   ├── matching/        # Track matching
│   │   │   └── isrc.go      # ISRC & metadata matching
│   │   ├── metadata/        # Audio file tagging
│   │   │   └── flac.go      # FLAC metadata embedding
│   │   ├── qobuz/           # Qobuz integration
│   │   │   ├── api.go       # Qobuz API client
│   │   │   └── downloader.go # Qobuz download logic
│   │   ├── spotify/         # Spotify integration
│   │   │   └── api.go       # Spotify API client
│   │   └── tidal/           # Tidal integration
│   │       ├── api.go       # Tidal API client
│   │       └── downloader.go # Tidal download logic
│   │
│   ├── utils/               # Utility functions
│   │   ├── sanitize.go      # Filename sanitization
│   │   └── romaji.go        # Japanese romanization
│   │
│   └── websocket/           # Real-time updates
│       ├── hub.go           # Connection management
│       └── client.go        # Client handling
│
└── config.yaml              # Default configuration file
```

### Key Design Patterns

- **Repository Pattern**: Database queries are abstracted in `db/queries/`
- **Service Layer**: Business logic isolated in `services/`
- **Dependency Injection**: Services are injected into handlers
- **Worker Pool**: Configurable number of concurrent download workers

---

## Download Flow

The complete download workflow from user request to finished file:

### Sequence Diagram

```
User          API Handler      Database       Scheduler       Worker          Orchestrator
  │                │              │              │              │                  │
  │ POST /jobs     │              │              │              │                  │
  │───────────────▶│              │              │              │                  │
  │                │ Create Batch │              │              │                  │
  │                │─────────────▶│              │              │                  │
  │                │ Create Jobs  │              │              │                  │
  │                │─────────────▶│              │              │                  │
  │    200 OK      │              │              │              │                  │
  │◀───────────────│              │              │              │                  │
  │                │              │              │              │                  │
  │                │              │ Poll pending │              │                  │
  │                │              │◀─────────────│              │                  │
  │                │              │              │ Dispatch job │                  │
  │                │              │              │─────────────▶│                  │
  │                │              │              │              │  Download track  │
  │                │              │              │              │─────────────────▶│
  │                │              │              │              │                  │
  │                │              │              │              │    ┌─────────────┴─────────────┐
  │                │              │              │              │    │ 1. Search ISRC on Tidal   │
  │                │              │              │              │    │ 2. Get download URL       │
  │                │              │              │              │    │ 3. Download FLAC          │
  │                │              │              │              │    │ 4. Fetch cover art        │
  │                │              │              │              │    │ 5. Fetch lyrics           │
  │                │              │              │              │    │ 6. Embed metadata         │
  │                │              │              │              │    │ 7. Move to library        │
  │                │              │              │              │    └─────────────┬─────────────┘
  │                │              │              │              │                  │
  │                │              │              │              │◀─────────────────│
  │                │              │ Update status│              │                  │
  │                │              │◀─────────────│──────────────│                  │
  │                │              │              │              │                  │
  │◀═══════════════╪══════════════╪══════════════╪══════════════╪══════════════════╡
  │           WebSocket: Job completed                          │                  │
```

### Step-by-Step Breakdown

#### 1. User Submits Spotify URL
```go
// POST /api/jobs
// Body: { "url": "https://open.spotify.com/track/..." }
```

#### 2. Handler Creates Batch and Jobs
`handlers/jobs.go` processes the request:
- Parses Spotify URL to extract track/album/playlist ID
- Fetches metadata from Spotify API
- Creates a `Batch` record in database
- Creates individual `Job` records for each track
- Returns batch ID to user

#### 3. Scheduler Picks Up Pending Jobs
`scheduler/scheduler.go` runs a polling loop:
- Queries database for jobs with status `pending`
- Respects concurrency limits
- Marks job as `processing`

#### 4. Worker Executes Download
`scheduler/worker.go` calls the orchestrator:
- Receives job from scheduler queue
- Invokes `download/orchestrator.go`
- Handles errors and retries

#### 5. Orchestrator Workflow
`download/orchestrator.go` performs the full download:

```go
func (o *Orchestrator) Download(job *models.Job) error {
    // 1. Search for track via ISRC on Tidal/Qobuz
    match, err := o.matching.FindMatch(job.ISRC, job.Title, job.Artist)
    
    // 2. Get download URL from third-party API
    downloadURL, err := o.getDownloadURL(match)
    
    // 3. Download FLAC file to temp directory
    tempPath, err := o.downloadFile(downloadURL)
    
    // 4. Fetch cover art from MusicBrainz
    coverArt, err := o.cover.Fetch(job.Album, job.Artist)
    
    // 5. Fetch lyrics from LRCLIB
    lyrics, err := o.lyrics.Fetch(job.ISRC, job.Title, job.Artist)
    
    // 6. Embed metadata using metadata/flac.go
    err = o.metadata.Embed(tempPath, job, coverArt, lyrics)
    
    // 7. Move file to final location
    finalPath := o.buildFinalPath(job)
    err = os.Rename(tempPath, finalPath)
    
    return nil
}
```

#### 6. WebSocket Status Updates
Throughout the process, status updates are broadcast:
```go
// Status progression
"pending" → "processing" → "downloading" → "tagging" → "completed"
                                                     → "failed"
```

---

## Third-Party APIs

SpotiSync uses third-party APIs to obtain download URLs without requiring user authentication with streaming services.

### Tidal Third-Party Endpoints

There are **8 Tidal third-party endpoints** (base64 encoded in source):

```go
// Endpoints are tried in parallel for fastest response
var tidalEndpoints = []string{
    // Base64 encoded URLs for security
    "aHR0cHM6Ly9leGFtcGxlMS5jb20v...",
    "aHR0cHM6Ly9leGFtcGxlMi5jb20v...",
    // ... 6 more endpoints
}
```

**How it works:**
1. All 8 endpoints are queried in parallel
2. First successful response is used
3. Returns direct FLAC download URL or DASH manifest
4. DASH manifests require ffmpeg for decryption

### Qobuz Third-Party Endpoints

There are **2 Qobuz third-party endpoints**:

```go
// Endpoints are tried with fallback
var qobuzEndpoints = []string{
    "primary_endpoint",
    "fallback_endpoint",
}
```

**How it works:**
1. Primary endpoint is tried first
2. Falls back to secondary on failure
3. Returns direct FLAC download URL

### API Response Format

```json
{
  "url": "https://cdn.example.com/track.flac",
  "quality": "LOSSLESS",
  "format": "FLAC",
  "bitDepth": 16,
  "sampleRate": 44100
}
```

### DASH Stream Handling

For Tidal DASH streams (MQA/Hi-Res):
```bash
# Manifest is parsed and segments downloaded
# FFmpeg combines and decrypts the stream
ffmpeg -i manifest.mpd -c copy output.flac
```

---

## Key Files Reference

### Backend - Most Important Files

| File | Description |
|------|-------------|
| `/backend/internal/services/tidal/downloader.go` | Tidal download logic with third-party API integration |
| `/backend/internal/services/qobuz/downloader.go` | Qobuz download logic with third-party API integration |
| `/backend/internal/services/download/orchestrator.go` | Main download workflow orchestrating all services |
| `/backend/internal/services/metadata/flac.go` | FLAC metadata embedding (tags, cover art, lyrics) |
| `/backend/internal/services/spotify/api.go` | Spotify API client for fetching track metadata |
| `/backend/internal/scheduler/scheduler.go` | Job queue and scheduling logic |
| `/backend/internal/scheduler/worker.go` | Worker pool for concurrent downloads |
| `/backend/internal/api/handlers/jobs.go` | HTTP handlers for job creation and management |
| `/backend/internal/websocket/hub.go` | WebSocket connection hub for real-time updates |

### Frontend - Most Important Files

| File | Description |
|------|-------------|
| `/frontend/src/lib/api.ts` | API client for all backend calls |
| `/frontend/src/stores/jobStore.ts` | Zustand store for job state management |
| `/frontend/src/hooks/useWebSocket.ts` | WebSocket hook for real-time updates |
| `/frontend/src/components/jobs/JobList.tsx` | Main job list component |

---

## Frontend Architecture

The frontend uses Next.js 14 with the App Router pattern:

```
frontend/
├── src/
│   ├── app/                     # Next.js App Router pages
│   │   ├── layout.tsx           # Root layout with providers
│   │   ├── page.tsx             # Landing page
│   │   ├── (auth)/              # Auth route group
│   │   │   ├── login/page.tsx   # Login page
│   │   │   └── register/page.tsx # Registration page
│   │   └── dashboard/           # Protected dashboard routes
│   │       ├── layout.tsx       # Dashboard layout
│   │       ├── page.tsx         # Main dashboard
│   │       ├── jobs/page.tsx    # Job management
│   │       └── settings/page.tsx # User settings
│   │
│   ├── components/              # React components
│   │   ├── auth/                # Authentication components
│   │   │   ├── LoginForm.tsx    # Login form
│   │   │   └── RegisterForm.tsx # Registration form
│   │   ├── jobs/                # Job-related components
│   │   │   ├── JobList.tsx      # Job listing table
│   │   │   ├── JobCard.tsx      # Individual job card
│   │   │   ├── JobProgress.tsx  # Progress indicator
│   │   │   └── NewJobForm.tsx   # Create new job form
│   │   ├── layout/              # Layout components
│   │   │   ├── Header.tsx       # Navigation header
│   │   │   ├── Sidebar.tsx      # Dashboard sidebar
│   │   │   └── Footer.tsx       # Page footer
│   │   └── ui/                  # Base UI components (shadcn/ui)
│   │       ├── button.tsx       # Button component
│   │       ├── input.tsx        # Input component
│   │       ├── card.tsx         # Card component
│   │       └── ...              # Other UI primitives
│   │
│   ├── hooks/                   # Custom React hooks
│   │   ├── useAuth.ts           # Authentication hook
│   │   ├── useJobs.ts           # Job fetching hook
│   │   └── useWebSocket.ts      # WebSocket connection hook
│   │
│   ├── lib/                     # Utilities and API
│   │   ├── api.ts               # API client (fetch wrapper)
│   │   ├── types.ts             # TypeScript type definitions
│   │   ├── utils.ts             # Utility functions
│   │   └── constants.ts         # App constants
│   │
│   ├── providers/               # React context providers
│   │   ├── AuthProvider.tsx     # Authentication context
│   │   ├── ThemeProvider.tsx    # Theme context
│   │   └── WebSocketProvider.tsx # WebSocket context
│   │
│   └── stores/                  # Zustand state stores
│       ├── authStore.ts         # Auth state
│       ├── jobStore.ts          # Job state
│       └── uiStore.ts           # UI state (modals, toasts)
│
├── public/                      # Static assets
├── next.config.ts               # Next.js configuration
├── tailwind.config.ts           # TailwindCSS configuration
├── tsconfig.json                # TypeScript configuration
└── Dockerfile                   # Multi-stage Docker build
```

### State Management with Zustand

```typescript
// stores/jobStore.ts
import { create } from 'zustand'

interface JobStore {
  jobs: Job[]
  isLoading: boolean
  fetchJobs: () => Promise<void>
  updateJob: (job: Job) => void
}

export const useJobStore = create<JobStore>((set) => ({
  jobs: [],
  isLoading: false,
  fetchJobs: async () => {
    set({ isLoading: true })
    const jobs = await api.getJobs()
    set({ jobs, isLoading: false })
  },
  updateJob: (job) => set((state) => ({
    jobs: state.jobs.map((j) => j.id === job.id ? job : j)
  })),
}))
```

---

## Environment Variables

### Backend Configuration

Configuration can be set via `config.yaml` or environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `SPOTISYNC_HOST` | Server bind address | `0.0.0.0` |
| `SPOTISYNC_PORT` | Server port | `8080` |
| `SPOTISYNC_DB_PATH` | SQLite database path | `./data/spotisync.db` |
| `SPOTISYNC_MUSIC_ROOT` | Music library output path | `./music` |
| `SPOTISYNC_TEMP_DIR` | Temporary download directory | `/tmp/spotisync` |
| `SPOTISYNC_WORKERS` | Number of concurrent workers | `3` |
| `SPOTISYNC_SECRET_KEY` | JWT signing key (production) | Auto-generated |
| `SPOTISYNC_ALLOWED_ORIGINS` | CORS allowed origins (production) | `*` |

### Frontend Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `NEXT_PUBLIC_API_URL` | Backend API URL | `http://backend:8080` |

### Docker Compose Variables

Create a `.env` file in the project root:

```bash
# Required
JWT_SECRET=your-secure-jwt-secret-key
SPOTIFY_CLIENT_ID=your-spotify-client-id
SPOTIFY_CLIENT_SECRET=your-spotify-client-secret
MUSIC_LIBRARY_PATH=/path/to/your/music/library

# Optional - Enhanced quality sources
TIDAL_CLIENT_ID=your-tidal-client-id
TIDAL_CLIENT_SECRET=your-tidal-client-secret
QOBUZ_APP_ID=your-qobuz-app-id
QOBUZ_SECRET=your-qobuz-secret
```

---

## Testing

### Backend Tests

```bash
# Run all backend tests
cd backend && go test ./...

# Run tests with verbose output
cd backend && go test -v ./...

# Run tests with coverage
cd backend && go test -cover ./...

# Run specific package tests
cd backend && go test ./internal/services/download/...

# Run tests with race detection
cd backend && go test -race ./...
```

### Frontend Tests

```bash
# Run frontend build check (type checking)
cd frontend && npm run build

# Run linting
cd frontend && npm run lint

# Run type checking only
cd frontend && npx tsc --noEmit
```

### Docker Build Tests

```bash
# Build all containers
docker-compose build

# Build specific service
docker-compose build backend
docker-compose build frontend

# Build without cache
docker-compose build --no-cache
```

### Integration Tests

```bash
# Start services and run integration tests
docker-compose up -d
cd backend && go test -tags=integration ./...
docker-compose down
```

---

## Adding New Features

### Adding a New API Endpoint

1. **Create handler** in `internal/api/handlers/`:
   ```go
   // internal/api/handlers/newfeature.go
   func (h *Handler) NewFeature(c *gin.Context) {
       // Handle request
   }
   ```

2. **Register route** in `internal/api/server.go`:
   ```go
   api.GET("/newfeature", h.NewFeature)
   ```

3. **Add tests** in `internal/api/handlers/newfeature_test.go`

### Adding a New Service

1. **Create service directory** in `internal/services/`:
   ```
   internal/services/newservice/
   ├── service.go      # Main service logic
   └── service_test.go # Unit tests
   ```

2. **Define interface** for dependency injection:
   ```go
   type NewService interface {
       DoSomething(ctx context.Context) error
   }
   ```

3. **Wire up** in `cmd/server/main.go`

### Adding Database Changes

1. **Create migration** in `internal/db/migrations/`:
   ```sql
   -- 002_add_new_table.sql
   CREATE TABLE new_table (
       id INTEGER PRIMARY KEY,
       created_at DATETIME DEFAULT CURRENT_TIMESTAMP
   );
   ```

2. **Update model** in `internal/db/models/`

3. **Add queries** in `internal/db/queries/`

### Adding Frontend Features

1. **API calls** go in `lib/api.ts`:
   ```typescript
   export async function newFeature(data: NewFeatureData) {
     return fetch(`${API_URL}/newfeature`, {
       method: 'POST',
       body: JSON.stringify(data),
     })
   }
   ```

2. **State management** in `stores/`:
   ```typescript
   // stores/newFeatureStore.ts
   export const useNewFeatureStore = create<NewFeatureStore>((set) => ({
     // state and actions
   }))
   ```

3. **Components** in `components/newfeature/`

---

## Code Style

### Go

- Follow standard Go conventions
- Run `gofmt` before committing
- Use `golint` and `go vet` for additional checks
- Keep functions small and focused
- Use meaningful variable names
- Document exported functions

```bash
# Format code
gofmt -w .

# Run linter
golint ./...

# Run vet
go vet ./...
```

### TypeScript

- ESLint for linting
- Prettier for formatting
- Use TypeScript strict mode
- Prefer functional components
- Use proper type annotations

```bash
# Lint code
npm run lint

# Fix lint issues
npm run lint:fix

# Format code
npm run format
```

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `style`: Code style (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding tests
- `chore`: Maintenance tasks

**Examples:**
```
feat(download): add retry logic for failed downloads
fix(metadata): handle missing cover art gracefully
docs(readme): update installation instructions
refactor(scheduler): simplify worker pool implementation
```

---

## Questions?

If you have questions about the codebase or need help:

1. Check existing issues and discussions
2. Review the code comments
3. Open a new issue with your question

Happy coding! 🎵
