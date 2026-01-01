package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User represents a user in the system
type User struct {
	ID                int64     `json:"id"`
	Username          string    `json:"username"`
	PasswordHash      string    `json:"-"`
	NavidromeURL      string    `json:"navidrome_url,omitempty"`
	NavidromeUsername string    `json:"-"`
	NavidromePassword string    `json:"-"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// SetPassword hashes the password using bcrypt
func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

// VerifyPassword checks if the password matches
func (u *User) VerifyPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// UserResponse is the public-facing user data (without sensitive fields)
type UserResponse struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	NavidromeURL string `json:"navidrome_url,omitempty"`
	CreatedAt    string `json:"created_at"`
}

// ToResponse converts a User to UserResponse
func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID:           u.ID,
		Username:     u.Username,
		NavidromeURL: u.NavidromeURL,
		CreatedAt:    u.CreatedAt.Format(time.RFC3339),
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6"`
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6"`
}

// LoginResponse represents a successful login response
type LoginResponse struct {
	Token string        `json:"token"`
	User  *UserResponse `json:"user"`
}
