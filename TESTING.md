# Spotisync Testing Guide

This comprehensive guide covers all testing procedures for Spotisync, from backend unit tests to end-to-end workflow validation.

## 1. Prerequisites

Before testing Spotisync, ensure you have the following installed:

### Required Software

- **Go** (version 1.21 or higher)
  - Check with: `go version`
  - Download from: https://go.dev/dl/

- **Node.js** (version 18 or higher)
  - Check with: `node --version`
  - Download from: https://nodejs.org/

- **SQLite3** (for database testing)
  - Usually pre-installed on macOS/Linux
  - Install via: `brew install sqlite3` (macOS)

### API Keys Required

For full functionality testing, you'll need API credentials for:

1. **Spotify**
   - Create app at: https://developer.spotify.com/dashboard
   - Required: Client ID and Client Secret

2. **Tidal**
   - Apply for access at: https://listen.tidal.com/artist/12345
   - Required: Username and Password

3. **Qobuz**
   - Apply for API access at: https://qobuz.com/developer
   - Required: Email and Password

## 2. Backend Setup & Testing

### 2.1 Building the Go Backend

Navigate to the backend directory:

```bash
cd backend
```

Install dependencies:

```bash
go mod download
go mod tidy
```

Build the server:

```bash
go build -o spotisync-server ./cmd/server
```

### 2.2 Configuration

Spotisync uses a YAML configuration file. Create a `config.yaml` in the backend directory:

```yaml
# Spotisync Configuration File
server:
  host: "0.0.0.0"
  port: 8080
  env: "development"
  log_level: "info"
  secret_key: "your-secret-key-here"  # Generate with: openssl rand -hex 32
  token_ttl: "24h"
  allowed_origins:
    - "http://localhost:3000"
    - "http://127.0.0.1:3000"

database:
  path: "./data/spotisync.db"
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: "5m"

storage:
  music_root: "./music"
  temp_dir: "./data/temp"
  max_file_size: "500MB"

workers:
  count: 2
  retry_max: 3
  retry_delays:
    - "1m"
    - "5m"
    - "15m"

spotify:
  username: "your-spotify-username"
  password: "your-spotify-password"

tidal:
  username: "your-tidal-username"
  password: "your-tidal-password"

qobuz:
  username: "your-qobuz-username"
  password: "your-qobuz-password"

navidrome:
  host: "http://localhost:4533"
  username: "your-navidrome-username"
  password: "your-navidrome-password"

websocket:
  ping_interval: "30s"
  pong_timeout: "10s"

rate_limit:
  requests_per_minute: 100
  burst: 20
```

### 2.3 Starting the Backend Server

**Development mode:**
```bash
go run ./cmd/server
```

**Production mode:**
```bash
./spotisync-server -config config.yaml
```

**With environment variables:**
```bash
export SPOTISYNC_SECRET_KEY="your-secret-key"
export SPOTISYNC_DB_PATH="./data/spotisync.db"
export SPOTISYNC_MUSIC_ROOT="./music"
go run ./cmd/server
```

### 2.4 Running Backend Tests

The backend includes comprehensive unit and integration tests:

```bash
# Run all tests
go test ./... -v

# Run tests with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# Run specific test packages
go test ./internal/api/handlers/ -v
go test ./internal/db/ -v
go test ./internal/services/ -v

# Run tests with race ./ detector
go test... -race

# Run tests with timeout
go test ./... -timeout 5m
```

**Backend Test Categories:**

| Package | Tests | Description |
|---------|-------|-------------|
| `internal/api/handlers` | Auth, Batches, Jobs, Settings | API endpoint tests |
| `internal/api/middleware` | Rate limiting, Auth middleware | Middleware tests |
| `internal/db/models` | User, Batch, Job models | Model validation tests |
| `internal/db` | Database operations | DB integration tests |
| `internal/auth` | JWT, Authentication | Auth logic tests |
| `internal/services` | Spotify, Tidal, Qobuz clients | Service client tests |
| `internal/websocket` | Hub, Client | WebSocket tests |
| `internal/config` | Configuration loading | Config tests |

### 2.5 Default Port and Endpoints

**Base URL:** `http://localhost:8080`

**API Endpoints:**

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/v1/auth/register` | No | Register new user |
| POST | `/api/v1/auth/login` | No | Login user |
| GET | `/api/v1/auth/me` | Yes | Get current user |
| POST | `/api/v1/jobs` | Yes | Create batch |
| GET | `/api/v1/jobs` | Yes | List all jobs |
| GET | `/api/v1/jobs/{id}` | Yes | Get job details |
| POST | `/api/v1/jobs/{id}/retry` | Yes | Retry failed job |
| DELETE | `/api/v1/jobs/{id}` | Yes | Cancel job |
| GET | `/api/v1/batches` | Yes | List batches |
| GET | `/api/v1/batches/{id}` | Yes | Get batch jobs |
| DELETE | `/api/v1/batches/{id}` | Yes | Delete batch |
| GET | `/api/v1/settings` | Yes | Get settings |
| PUT | `/api/v1/settings` | Yes | Update settings |
| POST | `/api/v1/settings/test-navidrome` | Yes | Test Navidrome |
| POST | `/api/v1/scan` | Yes | Trigger scan |
| GET | `/api/v1/ws` | Yes | WebSocket |

## 3. Frontend Setup & Testing

### 3.1 Installing Dependencies

Navigate to the frontend directory:

```bash
cd frontend
```

Install dependencies:

```bash
npm install
# or
yarn install
# or
pnpm install
```

### 3.2 Starting the Development Server

```bash
npm run dev
# or
yarn dev
# or
pnpm dev
```

The frontend will start at `http://localhost:3000`

**Development features:**
- Hot module replacement (HMR)
- TypeScript checking
- ESLint integration
- Tailwind CSS

### 3.3 Building for Production

```bash
npm run build
# or
yarn build
# or
pnpm build
```

**Production build output:**
- Optimized bundles
- Minified assets
- Static files in `.next/` directory

### 3.4 Running Production Build

```bash
npm run start
# or
yarn start
# or
pnpm start
```

### 3.5 Frontend Testing

```bash
# Run unit tests
npm run test

# Run tests in watch mode
npm run test:watch

# Run tests with coverage
npm run test:coverage

# Run e2e tests (if configured)
npm run test:e2e

# Run linting
npm run lint
```

## 4. End-to-End Testing Workflow

Follow this comprehensive workflow to test Spotisync functionality:

### 4.1 Register a New User

1. Open browser to `http://localhost:3000`
2. Click "Register" or navigate to `/register`
3. Fill in registration form:
   - Username: `testuser`
   - Password: `SecurePassword123!`
4. Submit form
5. Verify redirect to dashboard

**Expected behavior:**
- User created successfully
- Auto-login after registration
- Dashboard displays with empty state

### 4.2 Login with User

1. Navigate to `/login`
2. Enter credentials:
   - Username: `testuser`
   - Password: `SecurePassword123!`
3. Click "Login"
4. Verify redirect to dashboard

**Expected behavior:**
- Login successful
- JWT token stored
- User profile visible

### 4.3 Create a Batch with Spotify URL

1. On dashboard, click "New Batch"
2. Enter Spotify URL:
   ```
   https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT
   ```
3. Optionally add batch name
4. Click "Create Batch"

**Expected behavior:**
- Batch created successfully
- Status: "pending"
- Job added to queue
- WebSocket notification received

### 4.4 View Batch Details

1. Navigate to `/batches`
2. Click on created batch
3. View batch details:
   - Batch ID
   - Status
   - Progress
   - Jobs list
   - Created/Updated timestamps

**Expected behavior:**
- Batch details load correctly
- Jobs display with status
- Progress bar updates

### 4.5 Test Retry Flow for Failed Jobs

1. Navigate to `/failed`
2. View failed jobs
3. Click "Retry" on a failed job
4. Confirm retry action

**Expected behavior:**
- Job status changes to "pending"
- Job re-queued for processing
- Progress updates via WebSocket

**Alternative: Retry via API**
```bash
curl -X POST http://localhost:8080/api/v1/jobs/{job_id}/retry \
  -H "Authorization: Bearer {token}"
```

### 4.6 Configure Navidrome Settings

1. Navigate to `/settings`
2. Fill Navidrome configuration:
   - Host: `http://localhost:4533`
   - Username: `navidrome-user`
   - Password: `navidrome-password`
3. Click "Test Connection"
4. Save settings

**Expected behavior:**
- Connection test succeeds
- Settings saved
- Future syncs use Navidrome

## 5. API Testing with curl

### 5.1 Register a New User

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "SecurePassword123!"
  }'
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {
    "id": 1,
    "username": "testuser"
  }
}
```

### 5.2 Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "SecurePassword123!"
  }'
```

### 5.3 Create a Batch

```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer {token}" \
  -d '{
    "spotify_url": "https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT"
  }'
```

### 5.4 List Batches

```bash
curl -X GET http://localhost:8080/api/v1/batches \
  -H "Authorization: Bearer {token}"
```

### 5.5 Get Settings

```bash
curl -X GET http://localhost:8080/api/v1/settings \
  -H "Authorization: Bearer {token}"
```

### 5.6 Update Settings

```bash
curl -X PUT http://localhost:8080/api/v1/settings \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer {token}" \
  -d '{
    "navidrome": {
      "host": "http://localhost:4533",
      "username": "user",
      "password": "password"
    }
  }'
```

### 5.7 Test Navidrome Connection

```bash
curl -X POST http://localhost:8080/api/v1/settings/test-navidrome \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer {token}" \
  -d '{
    "host": "http://localhost:4533",
    "username": "user",
    "password": "password"
  }'
```

### 5.8 Complete Test Sequence

```bash
# 1. Register
REGISTER=$(curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"SecurePassword123!"}')
TOKEN=$(echo $REGISTER | jq -r '.token')

# 2. Create batch
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"spotify_url":"https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT"}'

# 3. List batches
curl -X GET http://localhost:8080/api/v1/batches \
  -H "Authorization: Bearer $TOKEN"
```

## 6. Troubleshooting

### 6.1 Common Errors and Fixes

| Error | Cause | Solution |
|-------|-------|----------|
| `connection refused` | Server not running | Start backend: `go run ./cmd/server` |
| `database locked` | Multiple connections | Check `max_open_conns` setting |
| `invalid token` | Token expired | Re-login to get new token |
| `CORS error` | Origin not allowed | Add origin to `allowed_origins` |
| `Spotify auth failed` | Invalid credentials | Check Spotify credentials in config |
| `port in use` | Port 8080 occupied | Kill process or change port |
| `EOF error` | Database closed | Check database connection |

### 6.2 Checking Backend Logs

**View logs in terminal:**
```bash
# Run server with logging
SPOTISYNC_LOG_LEVEL=debug go run ./cmd/server
```

**Log levels:** `debug`, `info`, `warn`, `error`

**Check logs:**
```bash
# Tail logs (Linux/macOS)
tail -f spotisync.log

# View last 100 lines
tail -100 spotisync.log

# Search for errors
grep -i error spotisync.log
```

### 6.3 Checking Browser Console

1. Open browser DevTools (F12)
2. Go to Console tab
3. Look for:
   - **Red errors**: API failures, network issues
   - **Yellow warnings**: Performance issues, deprecations
   - **Network tab**: Failed requests, response codes

**Common console messages:**

| Message | Meaning |
|---------|---------|
| `Failed to load resource` | Network/API error |
| `CORS policy` | Cross-origin issue |
| `401 Unauthorized` | Invalid/missing token |
| `500 Server Error` | Backend error |

### 6.4 Database Issues

**Verify database:**
```bash
sqlite3 ./data/spotisync.db
sqlite> .tables
sqlite> SELECT * FROM users;
sqlite> .quit
```

**Reset database:**
```bash
rm ./data/spotisync.db
go run ./cmd/server  # Will create new database
```

### 6.5 Port Conflicts

**Check port 8080:**
```bash
# macOS
lsof -i :8080

# Linux
netstat -tulpn | grep 8080
```

**Kill process on port:**
```bash
kill $(lsof -t -i:8080)
```

### 6.6 WebSocket Issues

**Test WebSocket connection:**
```bash
# Using websocat (install with: brew install websocat)
websocat ws://localhost:8080/api/v1/ws \
  -H "Authorization: Bearer {token}"
```

**Common WS issues:**
- Token expired → Re-authenticate
- CORS blocked → Check origin settings
- Connection refused → Server not running

## Environment Variables Reference

| Variable | Description | Default |
|----------|-------------|---------|
| `SPOTISYNC_HOST` | Server host | `0.0.0.0` |
| `SPOTISYNC_PORT` | Server port | `8080` |
| `SPOTISYNC_LOG_LEVEL` | Log verbosity | `info` |
| `SPOTISYNC_SECRET_KEY` | JWT signing key | Auto-generated (dev) |
| `SPOTISYNC_DB_PATH` | Database path | `./data/spotisync.db` |
| `SPOTISYNC_MUSIC_ROOT` | Music directory | `./music` |
| `SPOTISYNC_TEMP_DIR` | Temp directory | `./data/temp` |
| `SPOTISYNC_WORKERS` | Worker count | `2` |
| `SPOTISYNC_ALLOWED_ORIGINS` | CORS origins | Empty |

## Quick Reference Commands

```bash
# Backend
cd backend
go run ./cmd/server                    # Start dev server
go test ./... -v                      # Run all tests
go test ./internal/api/handlers/       # API tests only

# Frontend
cd frontend
npm run dev                           # Start dev server
npm run build                         # Production build
npm run start                         # Production server

# Database
sqlite3 ./data/spotisync.db           # Open DB shell

# API Testing
BASE_URL="http://localhost:8080"
TOKEN=$(curl -s -X POST $BASE_URL/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"user","password":"pass"}' | jq -r '.token')
curl -H "Authorization: Bearer $TOKEN" $BASE_URL/api/v1/batches
```
