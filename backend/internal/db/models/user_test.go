package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUser_SetPassword(t *testing.T) {
	user := &User{}
	password := "testpassword123"

	err := user.SetPassword(password)
	assert.NoError(t, err)
	assert.NotEmpty(t, user.PasswordHash)
	assert.NotEqual(t, password, user.PasswordHash)
}

func TestUser_VerifyPassword(t *testing.T) {
	user := &User{}
	password := "testpassword123"

	// Set password
	err := user.SetPassword(password)
	assert.NoError(t, err)

	// Verify correct password
	assert.True(t, user.VerifyPassword(password))

	// Verify incorrect password
	assert.False(t, user.VerifyPassword("wrongpassword"))
}

func TestUser_VerifyPassword_EmptyHash(t *testing.T) {
	user := &User{
		PasswordHash: "",
	}

	assert.False(t, user.VerifyPassword("anypassword"))
}

func TestUser_ToResponse(t *testing.T) {
	user := &User{
		ID:           1,
		Username:     "testuser",
		NavidromeURL: "http://localhost:4533",
		CreatedAt:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	response := user.ToResponse()

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, "testuser", response.Username)
	assert.Equal(t, "http://localhost:4533", response.NavidromeURL)
	assert.Equal(t, "2025-01-01T00:00:00Z", response.CreatedAt)
}
