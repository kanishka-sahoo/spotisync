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
)

// setupSettingsTestDB creates an in-memory SQLite database for testing
func setupSettingsTestDB(t *testing.T) *db.Database {
	database, err := db.New(&config.DatabaseConfig{
		Path:            ":memory:",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
	})
	require.NoError(t, err)
	return database
}

// setupSettingsHandler creates a settings handler with test dependencies
func setupSettingsHandler(t *testing.T) (*handlers.SettingsHandler, *db.Database, *auth.JWTManager) {
	database := setupSettingsTestDB(t)
	jwtManager := auth.NewJWTManager("test-secret-key", time.Hour)
	cfg := &config.Config{
		Storage: config.StorageConfig{
			MusicRoot: "/tmp/test-music",
		},
	}
	handler := handlers.NewSettingsHandler(database, jwtManager, cfg)
	return handler, database, jwtManager
}

// TestGetSettings tests getting user settings
func TestGetSettings(t *testing.T) {
	handler, database, jwtManager := setupSettingsHandler(t)
	userID, token := createTestUser(t, database, jwtManager)

	tests := []struct {
		name           string
		token          string
		expectedStatus int
		checkFunc      func(*testing.T, string)
	}{
		{
			name:           "get settings with valid token",
			token:          token,
			expectedStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				var response handlers.SettingsResponse
				err := json.Unmarshal([]byte(body), &response)
				assert.NoError(t, err)
				// Initially no Navidrome URL set
				assert.Empty(t, response.NavidromeURL)
			},
		},
		{
			name:           "get settings without token",
			token:          "",
			expectedStatus: http.StatusUnauthorized,
			checkFunc:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			ctx := req.Context()
			if tt.expectedStatus == http.StatusOK {
				ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
			}
			w := httptest.NewRecorder()

			handler.GetSettings(w, req.WithContext(ctx))

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkFunc != nil {
				tt.checkFunc(t, w.Body.String())
			}
		})
	}
}

// TestUpdateSettings tests updating user settings (Navidrome credentials)
func TestUpdateSettings(t *testing.T) {
	handler, database, jwtManager := setupSettingsHandler(t)
	userID, token := createTestUser(t, database, jwtManager)
	_, token2 := createSecondTestUser(t, database, jwtManager)

	tests := []struct {
		name              string
		requestBody       map[string]string
		token             string
		expectedStatus    int
		expectedNavidrome string
	}{
		{
			name: "update settings with all fields",
			requestBody: map[string]string{
				"navidrome_url":      "http://navidrome.local:4533",
				"navidrome_username": "testuser",
				"navidrome_password": "testpass",
			},
			token:             token,
			expectedStatus:    http.StatusOK,
			expectedNavidrome: "http://navidrome.local:4533",
		},
		{
			name: "update settings with empty URL",
			requestBody: map[string]string{
				"navidrome_url":      "",
				"navidrome_username": "",
				"navidrome_password": "",
			},
			token:             token,
			expectedStatus:    http.StatusOK,
			expectedNavidrome: "",
		},
		{
			name: "update settings without authentication",
			requestBody: map[string]string{
				"navidrome_url": "http://navidrome.local",
			},
			token:             "",
			expectedStatus:    http.StatusUnauthorized,
			expectedNavidrome: "",
		},
		{
			name: "update settings for another user",
			requestBody: map[string]string{
				"navidrome_url": "http://navidrome.local",
			},
			token:             token2,
			expectedStatus:    http.StatusUnauthorized,
			expectedNavidrome: "",
		},
		{
			name: "update settings with invalid URL format",
			requestBody: map[string]string{
				"navidrome_url": "invalid-url",
			},
			token:             token,
			expectedStatus:    http.StatusBadRequest,
			expectedNavidrome: "",
		},
		{
			name: "update settings with invalid URL scheme",
			requestBody: map[string]string{
				"navidrome_url": "ftp://navidrome.local:4533",
			},
			token:             token,
			expectedStatus:    http.StatusBadRequest,
			expectedNavidrome: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPut, "/api/v1/settings", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			ctx := req.Context()
			if tt.expectedStatus != http.StatusUnauthorized {
				ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
			}
			w := httptest.NewRecorder()

			handler.UpdateSettings(w, req.WithContext(ctx))

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				// Verify the update in the database
				user, err := database.GetUserByID(userID)
				assert.NoError(t, err)
				assert.NotNil(t, user)
				assert.Equal(t, tt.expectedNavidrome, user.NavidromeURL)
			}
		})
	}
}

// TestUpdateSettingsPartialUpdate tests partial updates to settings
func TestUpdateSettingsPartialUpdate(t *testing.T) {
	handler, database, jwtManager := setupSettingsHandler(t)
	userID, token := createTestUser(t, database, jwtManager)

	// Initially set Navidrome URL
	err := database.UpdateUserNavidrome(userID, "http://old-navidrome.local", "olduser", "oldpass")
	require.NoError(t, err)

	// Update only the URL
	body, _ := json.Marshal(map[string]string{"navidrome_url": "http://new-navidrome.local"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
	w := httptest.NewRecorder()

	handler.UpdateSettings(w, req.WithContext(ctx))

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify only URL was updated, credentials remain unchanged
	user, err := database.GetUserByID(userID)
	assert.NoError(t, err)
	assert.Equal(t, "http://new-navidrome.local", user.NavidromeURL)
	assert.Equal(t, "olduser", user.NavidromeUsername)
	assert.Equal(t, "oldpass", user.NavidromePassword)
}

// TestSettingsIsolation tests that users can only access their own settings
func TestSettingsIsolation(t *testing.T) {
	handler, database, jwtManager := setupSettingsHandler(t)
	userID1, token1 := createTestUser(t, database, jwtManager)
	userID2, token2 := createSecondTestUser(t, database, jwtManager)

	// Set Navidrome URL for user 1
	err := database.UpdateUserNavidrome(userID1, "http://user1-navidrome.local", "user1", "pass1")
	require.NoError(t, err)

	// Set Navidrome URL for user 2
	err = database.UpdateUserNavidrome(userID2, "http://user2-navidrome.local", "user2", "pass2")
	require.NoError(t, err)

	// User 1 should see their own settings
	t.Run("user1 sees own settings", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID1)
		w := httptest.NewRecorder()

		handler.GetSettings(w, req.WithContext(ctx))

		assert.Equal(t, http.StatusOK, w.Code)
		var response handlers.SettingsResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "http://user1-navidrome.local", response.NavidromeURL)
	})

	// User 2 should see their own settings
	t.Run("user2 sees own settings", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
		req.Header.Set("Authorization", "Bearer "+token2)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID2)
		w := httptest.NewRecorder()

		handler.GetSettings(w, req.WithContext(ctx))

		assert.Equal(t, http.StatusOK, w.Code)
		var response handlers.SettingsResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "http://user2-navidrome.local", response.NavidromeURL)
	})

	// User 1 should not be able to access User 2's settings
	t.Run("user1 cannot access user2 settings", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID2) // Pretend to be user2
		w := httptest.NewRecorder()

		handler.GetSettings(w, req.WithContext(ctx))

		// Should return 404 since user2 doesn't exist in context of user1
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
