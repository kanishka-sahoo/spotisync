package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTManager_GenerateAndValidate(t *testing.T) {
	manager := NewJWTManager("test-secret-key", time.Hour*24)

	userID := int64(123)
	username := "testuser"

	// Generate token
	token, err := manager.GenerateToken(userID, username)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate token
	claims, err := manager.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, username, claims.Username)
	assert.Equal(t, "spotisync", claims.Issuer)
}

func TestJWTManager_InvalidToken(t *testing.T) {
	manager := NewJWTManager("test-secret-key", time.Hour*24)

	tests := []struct {
		name  string
		token string
	}{
		{"empty token", ""},
		{"invalid format", "not.a.valid.token"},
		{"wrong signature", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxMjN9.wrong_signature"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.ValidateToken(tt.token)
			assert.Error(t, err)
		})
	}
}

func TestJWTManager_WrongSecret(t *testing.T) {
	manager1 := NewJWTManager("secret1", time.Hour*24)
	manager2 := NewJWTManager("secret2", time.Hour*24)

	// Generate token with manager1
	token, err := manager1.GenerateToken(123, "testuser")
	require.NoError(t, err)

	// Try to validate with manager2 (different secret)
	_, err = manager2.ValidateToken(token)
	assert.Error(t, err)
}

func TestJWTManager_ExpiredToken(t *testing.T) {
	// Create manager with very short TTL (already expired)
	manager := NewJWTManager("test-secret-key", -time.Hour)

	token, err := manager.GenerateToken(123, "testuser")
	require.NoError(t, err)

	_, err = manager.ValidateToken(token)
	assert.Error(t, err)
	assert.Equal(t, ErrExpiredToken, err)
}

func TestJWTManager_RefreshToken(t *testing.T) {
	manager := NewJWTManager("test-secret-key", time.Hour*24)

	// Generate initial token
	token1, err := manager.GenerateToken(123, "testuser")
	require.NoError(t, err)

	// Wait a bit to ensure different timestamps
	time.Sleep(time.Second)

	// Refresh token
	token2, err := manager.RefreshToken(token1)
	require.NoError(t, err)

	// Tokens should be different (differentiatin by timestamp)
	assert.NotEqual(t, token1, token2)

	// Validate new token
	claims, err := manager.ValidateToken(token2)
	require.NoError(t, err)
	assert.Equal(t, int64(123), claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
}

func TestJWTManager_RefreshInvalidToken(t *testing.T) {
	manager := NewJWTManager("test-secret-key", time.Hour*24)

	_, err := manager.RefreshToken("invalid-token")
	assert.Error(t, err)
}
