package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"spotisync/internal/api/handlers"
	"spotisync/internal/api/middleware"
	"spotisync/internal/auth"
	"spotisync/internal/config"
	"spotisync/internal/db"
	"spotisync/internal/db/models"
	"spotisync/internal/scheduler"
	"spotisync/internal/services/navidrome"
)

// setupBatchesTestDB creates an in-memory SQLite database for testing
func setupBatchesTestDB(t *testing.T) *db.Database {
	database, err := db.New(&config.DatabaseConfig{
		Path:            ":memory:",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
	})
	require.NoError(t, err)
	return database
}

// setupBatchesHandler creates a batches handler with test dependencies
func setupBatchesHandler(t *testing.T) (*handlers.BatchesHandler, *handlers.JobsHandler, *db.Database, *auth.JWTManager) {
	database := setupBatchesTestDB(t)
	jwtManager := auth.NewJWTManager("test-secret-key", time.Hour)
	retryPolicy := scheduler.RetryPolicy{
		MaxRetries: 3,
		Delays:     []time.Duration{time.Second, time.Second * 5, time.Second * 30},
	}
	jobScheduler := scheduler.NewJobScheduler(database, 2, retryPolicy)
	cfg := &config.Config{
		Spotify: config.ServiceConfig{
			Username: "test-client-id",
			Password: "test-client-secret",
		},
	}
	// Create a navidrome syncer for testing
	playlistSyncer := navidrome.NewSyncer(database)
	batchesHandler := handlers.NewBatchesHandler(database, jwtManager, jobScheduler, playlistSyncer)
	jobsHandler := handlers.NewJobsHandler(database, jobScheduler, jwtManager, cfg)
	return batchesHandler, jobsHandler, database, jwtManager
}

// createTestBatch creates a test batch for a user
func createTestBatch(t *testing.T, database *db.Database, userID int64, name, spotifyURL, spotifyType string) *models.Batch {
	batch := models.NewBatch(userID, spotifyURL, spotifyType, name)
	err := database.CreateBatch(batch)
	require.NoError(t, err)
	return batch
}

// TestBatchListing tests batch listing endpoint
func TestBatchListing(t *testing.T) {
	handler, _, database, jwtManager := setupBatchesHandler(t)
	userID, token := createTestUser(t, database, jwtManager)

	// Create some test batches
	_ = createTestBatch(t, database, userID, "Batch 1", "https://open.spotify.com/track/123", "track")
	_ = createTestBatch(t, database, userID, "Batch 2", "https://open.spotify.com/album/456", "album")

	tests := []struct {
		name            string
		token           string
		expectedStatus  int
		expectedBatches int
	}{
		{
			name:            "list batches with valid token",
			token:           token,
			expectedStatus:  http.StatusOK,
			expectedBatches: 2,
		},
		{
			name:            "list batches without token",
			token:           "",
			expectedStatus:  http.StatusUnauthorized,
			expectedBatches: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/batches", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			// Set up context with user ID if token is valid
			if tt.expectedStatus == http.StatusOK {
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			handler.ListBatches(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var batches []*models.BatchResponse
				err := json.Unmarshal(w.Body.Bytes(), &batches)
				assert.NoError(t, err)
				assert.Len(t, batches, tt.expectedBatches)
			}
		})
	}
}

// TestBatchRetrievalWithJobs tests batch retrieval with jobs
func TestBatchRetrievalWithJobs(t *testing.T) {
	_, _, database, jwtManager := setupBatchesHandler(t)
	userID, _ := createTestUser(t, database, jwtManager)

	// Create a test batch with jobs
	batch := createTestBatch(t, database, userID, "Test Batch", "https://open.spotify.com/album/123", "album")

	// Create jobs for the batch
	for i := 0; i < 3; i++ {
		job := models.NewJob(batch.ID, userID, &models.SpotifyTrack{
			ID:   "track" + string(rune('1'+i)),
			Name: "Track " + string(rune('1'+i)),
		})
		err := database.CreateJob(job)
		require.NoError(t, err)
	}

	// Directly test the database methods (bypassing HTTP)
	t.Run("get batch from database", func(t *testing.T) {
		retrievedBatch, err := database.GetBatchByID(batch.ID)
		require.NoError(t, err)
		assert.NotNil(t, retrievedBatch)
		assert.Equal(t, batch.ID, retrievedBatch.ID)
		assert.Equal(t, "Test Batch", retrievedBatch.Name)
	})

	t.Run("get jobs from database", func(t *testing.T) {
		jobs, err := database.GetJobsByBatchID(batch.ID)
		require.NoError(t, err)
		assert.Len(t, jobs, 3)
	})

	t.Run("get non-existent batch from database", func(t *testing.T) {
		retrievedBatch, err := database.GetBatchByID("non-existent-batch")
		require.NoError(t, err)
		assert.Nil(t, retrievedBatch)
	})
}

// TestBatchDeletion tests batch deletion endpoint
func TestBatchDeletion(t *testing.T) {
	handler, _, database, jwtManager := setupBatchesHandler(t)
	userID, token := createTestUser(t, database, jwtManager)
	_, token2 := createSecondTestUser(t, database, jwtManager)

	// Create a test batch with jobs
	batch := createTestBatch(t, database, userID, "Batch to Delete", "https://open.spotify.com/track/123", "track")

	// Create a job for the batch
	job := models.NewJob(batch.ID, userID, &models.SpotifyTrack{
		ID:   "track123",
		Name: "Track to Delete",
	})
	err := database.CreateJob(job)
	require.NoError(t, err)

	tests := []struct {
		name           string
		batchID        string
		token          string
		expectedStatus int
		checkDeleted   bool
	}{
		{
			name:           "delete batch successfully",
			batchID:        batch.ID,
			token:          token,
			expectedStatus: http.StatusOK,
			checkDeleted:   true,
		},
		{
			name:           "delete non-existent batch",
			batchID:        "non-existent-batch",
			token:          token,
			expectedStatus: http.StatusNotFound,
			checkDeleted:   false,
		},
		{
			name:           "delete batch without authentication",
			batchID:        batch.ID,
			token:          "",
			expectedStatus: http.StatusUnauthorized,
			checkDeleted:   false,
		},
		{
			name:           "delete batch of another user",
			batchID:        batch.ID,
			token:          token2,
			expectedStatus: http.StatusForbidden,
			checkDeleted:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/api/v1/batches/"+tt.batchID, nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)

			ctx := req.Context()
			if tt.expectedStatus != http.StatusUnauthorized {
				ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
			}
			w := httptest.NewRecorder()

			handler.DeleteBatch(w, req.WithContext(ctx))

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkDeleted {
				// Verify batch was deleted
				deletedBatch, err := database.GetBatchByID(batch.ID)
				assert.NoError(t, err)
				assert.Nil(t, deletedBatch)

				// Verify job was also deleted
				deletedJob, err := database.GetJobByID(job.ID)
				assert.NoError(t, err)
				assert.Nil(t, deletedJob)
			}
		})
	}
}

// TestBatchIsolation tests that users can only see their own batches
func TestBatchIsolation(t *testing.T) {
	handler, _, database, jwtManager := setupBatchesHandler(t)
	userID1, token1 := createTestUser(t, database, jwtManager)
	userID2, token2 := createSecondTestUser(t, database, jwtManager)

	// Create batches for user 1
	batch1 := createTestBatch(t, database, userID1, "User1 Batch", "https://open.spotify.com/track/123", "track")

	// Create batches for user 2
	batch2 := createTestBatch(t, database, userID2, "User2 Batch", "https://open.spotify.com/track/456", "track")

	// User 1 should only see their batch
	t.Run("user1 sees only own batches", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/batches", nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID1)
		w := httptest.NewRecorder()

		handler.ListBatches(w, req.WithContext(ctx))

		assert.Equal(t, http.StatusOK, w.Code)
		var batches []*models.BatchResponse
		err := json.Unmarshal(w.Body.Bytes(), &batches)
		assert.NoError(t, err)
		assert.Len(t, batches, 1)
		assert.Equal(t, batch1.ID, batches[0].ID)
	})

	// User 2 should only see their batch
	t.Run("user2 sees only own batches", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/batches", nil)
		req.Header.Set("Authorization", "Bearer "+token2)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID2)
		w := httptest.NewRecorder()

		handler.ListBatches(w, req.WithContext(ctx))

		assert.Equal(t, http.StatusOK, w.Code)
		var batches []*models.BatchResponse
		err := json.Unmarshal(w.Body.Bytes(), &batches)
		assert.NoError(t, err)
		assert.Len(t, batches, 1)
		assert.Equal(t, batch2.ID, batches[0].ID)
	})

	// User 1 should not be able to access User 2's batch
	t.Run("user1 cannot access user2 batch", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/batches/"+batch2.ID, nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID1)
		w := httptest.NewRecorder()

		handler.GetBatch(w, req.WithContext(ctx))

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	// User 1 should not be able to delete User 2's batch
	t.Run("user1 cannot delete user2 batch", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/batches/"+batch2.ID, nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID1)
		w := httptest.NewRecorder()

		handler.DeleteBatch(w, req.WithContext(ctx))

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	// Verify User 2's batch still exists
	t.Run("user2 batch still exists after failed delete", func(t *testing.T) {
		batch, err := database.GetBatchByID(batch2.ID)
		assert.NoError(t, err)
		assert.NotNil(t, batch)
	})
}

// createTestJob creates a job with a specific status for testing
func createTestJob(t *testing.T, database *db.Database, batchID string, userID int64, status models.JobStatus) *models.Job {
	job := models.NewJob(batchID, userID, &models.SpotifyTrack{
		ID:   "test-track-id",
		Name: "Test Track",
	})
	job.Status = status
	err := database.CreateJob(job)
	require.NoError(t, err)
	return job
}

// TestBatchListingOrder tests that batches are listed in descending order by creation date
func TestBatchListingOrder(t *testing.T) {
	handler, _, database, jwtManager := setupBatchesHandler(t)
	userID, token := createTestUser(t, database, jwtManager)

	// Create batches in a specific order
	batch1 := createTestBatch(t, database, userID, "First Batch", "https://open.spotify.com/track/111", "track")
	time.Sleep(time.Millisecond) // Ensure different timestamps
	batch2 := createTestBatch(t, database, userID, "Second Batch", "https://open.spotify.com/track/222", "track")
	time.Sleep(time.Millisecond)
	batch3 := createTestBatch(t, database, userID, "Third Batch", "https://open.spotify.com/track/333", "track")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/batches", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
	w := httptest.NewRecorder()

	handler.ListBatches(w, req.WithContext(ctx))

	assert.Equal(t, http.StatusOK, w.Code)
	var batches []*models.BatchResponse
	err := json.Unmarshal(w.Body.Bytes(), &batches)
	assert.NoError(t, err)
	assert.Len(t, batches, 3)

	// Verify order (most recent first)
	assert.Equal(t, batch3.ID, batches[0].ID)
	assert.Equal(t, batch2.ID, batches[1].ID)
	assert.Equal(t, batch1.ID, batches[2].ID)
}

// TestBatchResync tests the ResyncBatch endpoint
func TestBatchResync(t *testing.T) {
	handler, _, database, jwtManager := setupBatchesHandler(t)
	userID, token := createTestUser(t, database, jwtManager)
	_, token2 := createSecondTestUser(t, database, jwtManager)

	tests := []struct {
		name             string
		batchID          string
		jobs             []models.JobStatus
		token            string
		expectedStatus   int
		expectedResynced int
	}{
		{
			name:             "resync successfully with mixed statuses",
			batchID:          "", // will set in the test
			jobs:             []models.JobStatus{models.JobStatusCompleted, models.JobStatusPending, models.JobStatusInProgress, models.JobStatusFailed, models.JobStatusNotFound},
			token:            token,
			expectedStatus:   http.StatusOK,
			expectedResynced: 3, // pending, failed, not_found (completed and in_progress are excluded)
		},
		{
			name:             "resync batch with all completed jobs",
			batchID:          "", // will set in the test
			jobs:             []models.JobStatus{models.JobStatusCompleted, models.JobStatusCompleted},
			token:            token,
			expectedStatus:   http.StatusOK,
			expectedResynced: 0,
		},
		{
			name:             "resync non-existent batch",
			batchID:          uuid.New().String(), // valid UUID format that doesn't exist
			token:            token,
			expectedStatus:   http.StatusNotFound,
			expectedResynced: 0,
		},
		{
			name:             "resync without authentication",
			batchID:          "", // will set in the test
			token:            "",
			expectedStatus:   http.StatusUnauthorized,
			expectedResynced: 0,
		},
		{
			name:             "resync another user's batch",
			batchID:          "", // will set in the test
			token:            token2,
			expectedStatus:   http.StatusForbidden,
			expectedResynced: 0,
		},
		{
			name:             "resync batch with only failed jobs",
			batchID:          "", // will set in the test
			jobs:             []models.JobStatus{models.JobStatusFailed, models.JobStatusFailed},
			token:            token,
			expectedStatus:   http.StatusOK,
			expectedResynced: 2,
		},
		{
			name:             "resync batch with only pending jobs",
			batchID:          "", // will set in the test
			jobs:             []models.JobStatus{models.JobStatusPending, models.JobStatusPending},
			token:            token,
			expectedStatus:   http.StatusOK,
			expectedResynced: 2,
		},
		{
			name:             "resync batch with not_found jobs",
			batchID:          "", // will set in the test
			jobs:             []models.JobStatus{models.JobStatusNotFound, models.JobStatusNotFound},
			token:            token,
			expectedStatus:   http.StatusOK,
			expectedResynced: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// If we are testing a batch that exists, create one
			if tt.batchID == "" && tt.name != "resync non-existent batch" && tt.name != "resync without authentication" {
				batch := createTestBatch(t, database, userID, "Resync Batch", "https://open.spotify.com/track/123", "track")
				tt.batchID = batch.ID

				// Create jobs for the batch with the specified statuses
				for _, status := range tt.jobs {
					createTestJob(t, database, batch.ID, userID, status)
				}
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/batches/"+tt.batchID+"/resync", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			// Set context with user ID for authenticated requests (except unauthorized which we skip the context)
			if tt.expectedStatus != http.StatusUnauthorized {
				ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			handler.ResyncBatch(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, float64(tt.expectedResynced), response["resynced_count"])
				assert.Equal(t, string(models.BatchStatusProcessing), response["batch_status"])
			}
		})
	}
}
