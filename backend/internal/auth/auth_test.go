package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantErr  error
	}{
		{"valid simple", "testuser", nil},
		{"valid with numbers", "user123", nil},
		{"valid with underscore", "test_user", nil},
		{"too short", "ab", ErrInvalidUsername},
		{"too long", "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz", ErrInvalidUsername},
		{"has hyphen", "test-user", ErrInvalidUsername},
		{"has space", "test user", ErrInvalidUsername},
		{"has special char", "test@user", ErrInvalidUsername},
		{"empty", "", ErrInvalidUsername},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUsername(tt.username)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"valid", "password123", nil},
		{"valid long", "verylongpassword12345", nil},
		{"too short", "12345", ErrWeakPassword},
		{"empty", "", ErrWeakPassword},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePassword(tt.password)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}
