package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"spotisync/internal/api/handlers"
	"spotisync/internal/api/middleware"
	"spotisync/internal/auth"
	"spotisync/internal/config"
	"spotisync/internal/db"
	"spotisync/internal/db/models"
	"spotisync/internal/scheduler"
	"spotisync/internal/services/spotify"
)

// setupJobsTestDB creates an in-memory SQLite database for testing
func setupJobsTestDB(t *testing.T) *db.Database {
	database, err := db.New(&config.DatabaseConfig{
		Path:            ":memory:",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
	})
	require.NoError(t, err)
	return database
}

// setupJobsHandler creates a jobs handler with test dependencies and mock Spotify client
func setupJobsHandler(t *testing.T) (*handlers.JobsHandler, *db.Database, *auth.JWTManager, *spotify.MockSpotifyClient) {
	database := setupJobsTestDB(t)
	jwtManager := auth.NewJWTManager("test-secret-key", time.Hour)
	retryPolicy := scheduler.RetryPolicy{
		MaxRetries: 3,
		Delays:     []time.Duration{time.Second, time.Second * 5, time.Second * 30},
	}
	jobScheduler := scheduler.NewJobScheduler(database, 2, retryPolicy, 100)

	// Use mock Spotify client
	mockSpotify := spotify.NewMockSpotifyClient()
	handler := handlers.NewJobsHandlerWithClient(database, jobScheduler, jwtManager, mockSpotify)

	return handler, database, jwtManager, mockSpotify
}

// createTestUser creates a test user and returns the user ID and token
func createTestUser(t *testing.T, database *db.Database, jwtManager *auth.JWTManager) (int64, string) {
	user := &models.User{
		Username: "testuser",
	}
	user.SetPassword("password123")
	id, err := database.CreateUser(user.Username, user.PasswordHash)
	require.NoError(t, err)
	user.ID = id

	token, err := jwtManager.GenerateToken(user.ID, user.Username)
	require.NoError(t, err)
	return user.ID, token
}

// createSecondTestUser creates a second test user for isolation tests
func createSecondTestUser(t *testing.T, database *db.Database, jwtManager *auth.JWTManager) (int64, string) {
	user := &models.User{
		Username: "testuser2",
	}
	user.SetPassword("password123")
	id, err := database.CreateUser(user.Username, user.PasswordHash)
	require.NoError(t, err)
	user.ID = id

	token, err := jwtManager.GenerateToken(user.ID, user.Username)
	require.NoError(t, err)
	return user.ID, token
}

// TestJobCreation tests job creation endpoint with mocked Spotify service
func TestJobCreation(t *testing.T) {
	handler, database, jwtManager, _ := setupJobsHandler(t)
	userID, token := createTestUser(t, database, jwtManager)

	tests := []struct {
		name           string
		spotifyURL     string
		expectedStatus int
	}{
		{
			name:           "successful job creation with track URL",
			spotifyURL:     "https://open.spotify.com/track/1234567890abcdef1234567890abcdef",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "successful job creation with album URL",
			spotifyURL:     "https://open.spotify.com/album/1234567890abcdef1234567890abcdef",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "successful job creation with playlist URL",
			spotifyURL:     "https://open.spotify.com/playlist/1234567890abcdef1234567890abcdef",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "successful job creation with short track ID",
			spotifyURL:     "https://spotify.com/track/123",
			expectedStatus: http.StatusCreated, // Mock returns data, so this succeeds
		},
		{
			name:           "invalid Spotify URL - wrong domain",
			spotifyURL:     "https://music.example.com/track/1234567890abcdef1234567890abcdef",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty Spotify URL",
			spotifyURL:     "",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"spotify_url": tt.spotifyURL})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)

			// Set up context with user ID
			ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
			ctx = context.WithValue(ctx, middleware.UsernameKey, "testuser")
			w := httptest.NewRecorder()

			handler.CreateBatch(w, req.WithContext(ctx))

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusCreated {
				var response models.BatchResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.NotEmpty(t, response.ID)
				assert.Equal(t, tt.spotifyURL, response.SpotifyURL)
			}
		})
	}
}

// TestJobListing tests job listing endpoint
func TestJobListing(t *testing.T) {
	handler, database, jwtManager, _ := setupJobsHandler(t)
	userID, token := createTestUser(t, database, jwtManager)

	// Create some test jobs
	batch := models.NewBatch(userID, "https://open.spotify.com/track/123", "track", "Test Batch")
	err := database.CreateBatch(batch)
	require.NoError(t, err)

	job := models.NewJob(batch.ID, userID, &models.SpotifyTrack{
		ID:   "track123",
		Name: "Test Track",
	})
	err = database.CreateJob(job)
	require.NoError(t, err)

	tests := []struct {
		name           string
		token          string
		expectedStatus int
		expectedJobs   int
	}{
		{
			name:           "list jobs with valid token",
			token:          token,
			expectedStatus: http.StatusOK,
			expectedJobs:   1,
		},
		{
			name:           "list jobs without token",
			token:          "",
			expectedStatus: http.StatusUnauthorized,
			expectedJobs:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			// Set up context with user ID if token is valid
			if tt.expectedStatus == http.StatusOK {
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			handler.ListJobs(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var jobs []*models.JobResponse
				err := json.Unmarshal(w.Body.Bytes(), &jobs)
				assert.NoError(t, err)
				assert.Len(t, jobs, tt.expectedJobs)
			}
		})
	}
}

// TestJobRetry tests job retry endpoint
func TestJobRetry(t *testing.T) {
	handler, database, jwtManager, _ := setupJobsHandler(t)
	userID, token := createTestUser(t, database, jwtManager)
	_, token2 := createSecondTestUser(t, database, jwtManager)

	// Create a test batch and job
	batch := models.NewBatch(userID, "https://open.spotify.com/track/123", "track", "Test Batch")
	err := database.CreateBatch(batch)
	require.NoError(t, err)

	job := models.NewJob(batch.ID, userID, &models.SpotifyTrack{
		ID:   "track123",
		Name: "Test Track",
	})
	err = database.CreateJob(job)
	require.NoError(t, err)

	// Mark job as failed
	err = database.UpdateJobFailed(job.ID, "test error")
	require.NoError(t, err)

	tests := []struct {
		name           string
		jobID          string
		token          string
		expectedStatus int
	}{
		{
			name:           "retry failed job successfully",
			jobID:          job.ID,
			token:          token,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "retry non-existent job",
			jobID:          "non-existent-job-id",
			token:          token,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "retry job without authentication",
			jobID:          job.ID,
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "retry job of another user",
			jobID:          job.ID,
			token:          token2,
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/"+tt.jobID+"/retry", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)

			ctx := req.Context()
			if tt.expectedStatus != http.StatusUnauthorized {
				ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
			}
			w := httptest.NewRecorder()

			handler.RetryJob(w, req.WithContext(ctx))

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestJobCancellation tests job cancellation endpoint
func TestJobCancellation(t *testing.T) {
	handler, database, jwtManager, _ := setupJobsHandler(t)
	userID, token := createTestUser(t, database, jwtManager)
	_, token2 := createSecondTestUser(t, database, jwtManager)

	// Create a test batch and job
	batch := models.NewBatch(userID, "https://open.spotify.com/track/123", "track", "Test Batch")
	err := database.CreateBatch(batch)
	require.NoError(t, err)

	job := models.NewJob(batch.ID, userID, &models.SpotifyTrack{
		ID:   "track123",
		Name: "Test Track",
	})
	err = database.CreateJob(job)
	require.NoError(t, err)

	tests := []struct {
		name           string
		jobID          string
		token          string
		expectedStatus int
	}{
		{
			name:           "cancel pending job successfully",
			jobID:          job.ID,
			token:          token,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "cancel non-existent job",
			jobID:          "non-existent-job-id",
			token:          token,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "cancel job without authentication",
			jobID:          job.ID,
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "cancel job of another user",
			jobID:          job.ID,
			token:          token2,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "cancel already completed job",
			jobID:          job.ID,
			token:          token,
			expectedStatus: http.StatusBadRequest, // Can't cancel completed job
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset job status for each test
			if tt.name == "cancel already completed job" {
				database.UpdateJobCompleted(job.ID, "/path/to/file.flac", 1024)
			} else if tt.name == "cancel pending job successfully" {
				database.UpdateJobStarted(job.ID) // Change to in_progress
			}

			req := httptest.NewRequest(http.MethodDelete, "/api/v1/jobs/"+tt.jobID, nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)

			ctx := req.Context()
			if tt.expectedStatus != http.StatusUnauthorized {
				ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
			}
			w := httptest.NewRecorder()

			handler.CancelJob(w, req.WithContext(ctx))

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestUserIsolation tests that users can only see their own jobs
func TestUserIsolation(t *testing.T) {
	handler, database, jwtManager, _ := setupJobsHandler(t)
	userID1, token1 := createTestUser(t, database, jwtManager)
	userID2, token2 := createSecondTestUser(t, database, jwtManager)

	// Create jobs for user 1
	batch1 := models.NewBatch(userID1, "https://open.spotify.com/track/123", "track", "User1 Batch")
	err := database.CreateBatch(batch1)
	require.NoError(t, err)

	job1 := models.NewJob(batch1.ID, userID1, &models.SpotifyTrack{
		ID:   "track1",
		Name: "User1 Track",
	})
	err = database.CreateJob(job1)
	require.NoError(t, err)

	// Create jobs for user 2
	batch2 := models.NewBatch(userID2, "https://open.spotify.com/track/456", "track", "User2 Batch")
	err = database.CreateBatch(batch2)
	require.NoError(t, err)

	job2 := models.NewJob(batch2.ID, userID2, &models.SpotifyTrack{
		ID:   "track2",
		Name: "User2 Track",
	})
	err = database.CreateJob(job2)
	require.NoError(t, err)

	// User 1 should only see their job
	t.Run("user1 sees only own jobs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID1)
		w := httptest.NewRecorder()

		handler.ListJobs(w, req.WithContext(ctx))

		assert.Equal(t, http.StatusOK, w.Code)
		var jobs []*models.JobResponse
		err := json.Unmarshal(w.Body.Bytes(), &jobs)
		assert.NoError(t, err)
		assert.Len(t, jobs, 1)
		assert.Equal(t, job1.ID, jobs[0].ID)
	})

	// User 2 should only see their job
	t.Run("user2 sees only own jobs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
		req.Header.Set("Authorization", "Bearer "+token2)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID2)
		w := httptest.NewRecorder()

		handler.ListJobs(w, req.WithContext(ctx))

		assert.Equal(t, http.StatusOK, w.Code)
		var jobs []*models.JobResponse
		err := json.Unmarshal(w.Body.Bytes(), &jobs)
		assert.NoError(t, err)
		assert.Len(t, jobs, 1)
		assert.Equal(t, job2.ID, jobs[0].ID)
	})

	// User 1 should not be able to access User 2's job
	t.Run("user1 cannot access user2 job", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+job2.ID, nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID1)
		w := httptest.NewRecorder()

		handler.GetJob(w, req.WithContext(ctx))

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	// User 1 should not be able to retry User 2's job
	t.Run("user1 cannot retry user2 job", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/"+job2.ID+"/retry", nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID1)
		w := httptest.NewRecorder()

		handler.RetryJob(w, req.WithContext(ctx))

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	// User 1 should not be able to cancel User 2's job
	t.Run("user1 cannot cancel user2 job", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/jobs/"+job2.ID, nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID1)
		w := httptest.NewRecorder()

		handler.CancelJob(w, req.WithContext(ctx))

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

// TestGetBatchJobs tests getting jobs for a specific batch
func TestGetBatchJobs(t *testing.T) {
	handler, database, jwtManager, _ := setupJobsHandler(t)
	userID, token := createTestUser(t, database, jwtManager)

	// Create a test batch with multiple jobs
	batch := models.NewBatch(userID, "https://open.spotify.com/album/123", "album", "Test Album")
	err := database.CreateBatch(batch)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		job := models.NewJob(batch.ID, userID, &models.SpotifyTrack{
			ID:   "track" + string(rune('1'+i)),
			Name: "Track " + string(rune('1'+i)),
		})
		err = database.CreateJob(job)
		require.NoError(t, err)
	}

	tests := []struct {
		name           string
		batchID        string
		token          string
		expectedStatus int
		expectedJobs   int
	}{
		{
			name:           "get batch jobs successfully",
			batchID:        batch.ID,
			token:          token,
			expectedStatus: http.StatusOK,
			expectedJobs:   3,
		},
		{
			name:           "get non-existent batch",
			batchID:        "non-existent-batch",
			token:          token,
			expectedStatus: http.StatusNotFound,
			expectedJobs:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/batches/"+tt.batchID, nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)
			ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
			w := httptest.NewRecorder()

			handler.GetBatchJobs(w, req.WithContext(ctx))

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response handlers.JobBatchResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Len(t, response.Jobs, tt.expectedJobs)
			}
		})
	}
}

// TestJobCreationWithMockedSpotifyService tests job creation with a mocked Spotify service
func TestJobCreationWithMockedSpotifyService(t *testing.T) {
	handler, database, jwtManager, _ := setupJobsHandler(t)
	userID, token := createTestUser(t, database, jwtManager)

	// Test Spotify URL parsing for different types
	tests := []struct {
		name           string
		spotifyURL     string
		expectedType   string
		expectedStatus int
	}{
		{
			name:           "track URL",
			spotifyURL:     "https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT",
			expectedType:   "track",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "album URL",
			spotifyURL:     "https://open.spotify.com/album/1DFixLWuPkv3KT3TnV35m3",
			expectedType:   "album",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "playlist URL",
			spotifyURL:     "https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M",
			expectedType:   "playlist",
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "artist URL",
			spotifyURL:     "https://open.spotify.com/artist/246dkjvS1zLTtiykXe5h60",
			expectedType:   "artist",
			expectedStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"spotify_url": tt.spotifyURL})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
			w := httptest.NewRecorder()

			handler.CreateBatch(w, req.WithContext(ctx))

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusCreated {
				var response models.BatchResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, models.SpotifyType(tt.expectedType), response.SpotifyType)
			}
		})
	}
}
