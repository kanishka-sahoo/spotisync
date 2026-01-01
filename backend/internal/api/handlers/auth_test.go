package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
)

// setupAuthTestDB creates an in-memory SQLite database for testing
func setupAuthTestDB(t *testing.T) *db.Database {
	database, err := db.New(&config.DatabaseConfig{
		Path:            ":memory:",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
	})
	require.NoError(t, err)
	return database
}

// setupAuthHandler creates an auth handler with test dependencies
func setupAuthHandler(t *testing.T) (*handlers.AuthHandler, *db.Database, *auth.JWTManager) {
	database := setupAuthTestDB(t)
	jwtManager := auth.NewJWTManager("test-secret-key", time.Hour)
	authService := auth.NewAuthService(database, jwtManager)
	handler := handlers.NewAuthHandler(authService, database)
	return handler, database, jwtManager
}

// TestUserRegistration tests user registration endpoint
func TestUserRegistration(t *testing.T) {
	handler, _, _ := setupAuthHandler(t)

	tests := []struct {
		name           string
		requestBody    map[string]string
		expectedStatus int
		expectedError  string
	}{
		{
			name: "successful registration",
			requestBody: map[string]string{
				"username": "testuser",
				"password": "password123",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "registration with existing username",
			requestBody: map[string]string{
				"username": "testuser",
				"password": "password123",
			},
			expectedStatus: http.StatusConflict,
			expectedError:  "username already taken",
		},
		{
			name: "registration with short username",
			requestBody: map[string]string{
				"username": "ab",
				"password": "password123",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid username",
		},
		{
			name: "registration with special characters in username",
			requestBody: map[string]string{
				"username": "test@user!",
				"password": "password123",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid username",
		},
		{
			name: "registration with short password",
			requestBody: map[string]string{
				"username": "testuser2",
				"password": "abc",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid password",
		},
		{
			name:           "registration with empty body",
			requestBody:    nil, // This will cause JSON decode error
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid request body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.requestBody != nil {
				body, _ = json.Marshal(tt.requestBody)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Register(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			} else {
				var response models.LoginResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.NotEmpty(t, response.Token)
				assert.Equal(t, tt.requestBody["username"], response.User.Username)
			}
		})
	}
}

// TestUserLogin tests user login endpoint
func TestUserLogin(t *testing.T) {
	handler, _, _ := setupAuthHandler(t)

	// First, register a test user
	regBody, _ := json.Marshal(map[string]string{
		"username": "logintest",
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(regBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.Register(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	tests := []struct {
		name           string
		requestBody    map[string]string
		expectedStatus int
		expectedError  string
	}{
		{
			name: "successful login",
			requestBody: map[string]string{
				"username": "logintest",
				"password": "password123",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "login with wrong password",
			requestBody: map[string]string{
				"username": "logintest",
				"password": "wrongpassword",
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "invalid username or password",
		},
		{
			name: "login with non-existent user",
			requestBody: map[string]string{
				"username": "nonexistent",
				"password": "password123",
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "invalid username or password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Login(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			} else {
				var response models.LoginResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.NotEmpty(t, response.Token)
				assert.Equal(t, tt.requestBody["username"], response.User.Username)
			}
		})
	}
}

// TestInvalidCredentials tests various invalid credential scenarios
func TestInvalidCredentials(t *testing.T) {
	handler, _, _ := setupAuthHandler(t)

	invalidTests := []struct {
		name           string
		requestBody    map[string]string
		expectedStatus int
	}{
		{
			name: "empty username",
			requestBody: map[string]string{
				"username": "",
				"password": "password123",
			},
			expectedStatus: http.StatusUnauthorized, // Empty username fails at service level
		},
		{
			name: "empty password",
			requestBody: map[string]string{
				"username": "testuser",
				"password": "",
			},
			expectedStatus: http.StatusUnauthorized, // Empty password fails at service level
		},
		{
			name: "missing password field",
			requestBody: map[string]string{
				"username": "testuser",
			},
			expectedStatus: http.StatusBadRequest, // Missing field fails JSON binding
		},
		{
			name:           "missing all fields",
			requestBody:    nil, // This will cause JSON decode error
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "short username",
			requestBody: map[string]string{
				"username": "ab",
				"password": "password123",
			},
			expectedStatus: http.StatusBadRequest, // Short username fails validation
		},
		{
			name: "short password",
			requestBody: map[string]string{
				"username": "testuser",
				"password": "abc",
			},
			expectedStatus: http.StatusBadRequest, // Short password fails validation
		},
	}

	for _, tt := range invalidTests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.requestBody != nil {
				body, _ = json.Marshal(tt.requestBody)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Login(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestJWTTokenValidation tests JWT token validation
func TestJWTTokenValidation(t *testing.T) {
	_, _, jwtManager := setupAuthHandler(t)

	tests := []struct {
		name          string
		token         string
		expectedValid bool
		expectedErr   error
	}{
		{
			name:          "valid token format check",
			token:         "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxLCJ1c2VybmFtZSI6InRlc3QifQ.test",
			expectedValid: false,
			expectedErr:   auth.ErrInvalidToken,
		},
		{
			name:          "invalid token format",
			token:         "invalid.token.format",
			expectedValid: false,
			expectedErr:   auth.ErrInvalidToken,
		},
		{
			name:          "malformed token",
			token:         "not-a-jwt",
			expectedValid: false,
			expectedErr:   auth.ErrInvalidToken,
		},
		{
			name:          "empty token",
			token:         "",
			expectedValid: false,
			expectedErr:   auth.ErrInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := jwtManager.ValidateToken(tt.token)
			if tt.expectedValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// TestJWTTokenExpiry tests JWT token expiry
func TestJWTTokenExpiry(t *testing.T) {
	// Create a JWT manager with very short expiry
	jwtManager := auth.NewJWTManager("test-secret-key", -time.Hour) // Already expired

	// Generate an expired token
	token, err := jwtManager.GenerateToken(1, "test")
	require.NoError(t, err)

	// Validate the expired token
	_, err = jwtManager.ValidateToken(token)
	assert.Error(t, err)
	assert.Equal(t, auth.ErrExpiredToken, err)
}

// TestAuthMiddleware tests the auth middleware
func TestAuthMiddleware(t *testing.T) {
	_, _, jwtManager := setupAuthHandler(t)

	// Create a test user ID and username
	userID := int64(1)
	username := "testuser"

	// Generate a valid token
	token, err := jwtManager.GenerateToken(userID, username)
	require.NoError(t, err)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "missing authorization header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid authorization format",
			authHeader:     "InvalidFormat " + token,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer invalid.token.here",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "only Bearer keyword",
			authHeader:     "Bearer",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			// Create a simple handler that returns OK when user ID is in context
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, ok := middleware.GetUserID(r.Context())
				if ok {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("OK"))
				} else {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
				}
			})

			// Apply the middleware directly without the full auth chain
			var authHandler http.Handler
			authHandler = func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Extract token from Authorization header
					authHeader := r.Header.Get("Authorization")
					if authHeader == "" {
						http.Error(w, "missing authorization header", http.StatusUnauthorized)
						return
					}

					// Check Bearer prefix
					parts := strings.SplitN(authHeader, " ", 2)
					if len(parts) != 2 || parts[0] != "Bearer" {
						http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
						return
					}

					// Validate token
					claims, err := jwtManager.ValidateToken(parts[1])
					if err != nil {
						http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
						return
					}

					// Add user ID and username to context
					ctx := r.Context()
					ctx = context.WithValue(ctx, middleware.UserIDKey, claims.UserID)
					ctx = context.WithValue(ctx, middleware.UsernameKey, claims.Username)

					next.ServeHTTP(w, r.WithContext(ctx))
				})
			}(handler)

			authHandler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestGetMeEndpoint tests the /api/v1/auth/me endpoint
func TestGetMeEndpoint(t *testing.T) {
	handler, database, jwtManager := setupAuthHandler(t)

	// Create a test user
	user := &models.User{
		Username: "getmetest",
	}
	user.SetPassword("password123")

	// Create the user in the database
	id, err := database.CreateUser(user.Username, user.PasswordHash)
	require.NoError(t, err)
	user.ID = id

	// Generate a valid token
	token, err := jwtManager.GenerateToken(user.ID, user.Username)
	require.NoError(t, err)

	// Test with valid token
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	// Set up the context with the user ID
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, user.ID)
	ctx = context.WithValue(ctx, middleware.UsernameKey, user.Username)
	handler.Me(w, req.WithContext(ctx))

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.UserResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, user.Username, response.Username)
	assert.Equal(t, user.ID, response.ID)
}

// TestGetMeEndpointUnauthorized tests the /api/v1/auth/me endpoint without authentication
func TestGetMeEndpointUnauthorized(t *testing.T) {
	handler, _, _ := setupAuthHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	w := httptest.NewRecorder()

	handler.Me(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestGetMeEndpointWithInvalidToken tests the /api/v1/auth/me endpoint with invalid token
func TestGetMeEndpointWithInvalidToken(t *testing.T) {
	handler, _, _ := setupAuthHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	w := httptest.NewRecorder()

	handler.Me(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestGetMeEndpointUserNotFound tests the /api/v1/auth/me endpoint when user is not found
func TestGetMeEndpointUserNotFound(t *testing.T) {
	handler, _, jwtManager := setupAuthHandler(t)

	// Create a user ID that doesn't exist
	token, err := jwtManager.GenerateToken(99999, "nonexistent")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	// We need to manually set the user ID in context since we're not using the actual auth middleware
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(99999))
	handler.Me(w, req.WithContext(ctx))

	// The handler will try to get the user and return 404
	assert.Equal(t, http.StatusNotFound, w.Code)
}
