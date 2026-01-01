package auth

import (
	"errors"
	"fmt"
	"regexp"

	"spotisync/internal/db"
	"spotisync/internal/db/models"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidPassword = errors.New("invalid password")
	ErrUserExists      = errors.New("user already exists")
	ErrWeakPassword    = errors.New("password does not meet requirements")
	ErrInvalidUsername = errors.New("username does not meet requirements")
)

// AuthService handles authentication operations
type AuthService struct {
	db         *db.Database
	jwtManager *JWTManager
}

// NewAuthService creates a new authentication service
func NewAuthService(database *db.Database, jwtManager *JWTManager) *AuthService {
	return &AuthService{
		db:         database,
		jwtManager: jwtManager,
	}
}

// Register creates a new user account
func (s *AuthService) Register(req *models.RegisterRequest) (*models.LoginResponse, error) {
	// Validate username
	if err := validateUsername(req.Username); err != nil {
		return nil, err
	}

	// Validate password
	if err := validatePassword(req.Password); err != nil {
		return nil, err
	}

	// Check if user already exists
	existing, err := s.db.GetUserByUsername(req.Username)
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	if existing != nil {
		return nil, ErrUserExists
	}

	// Create new user
	user := &models.User{
		Username: req.Username,
	}
	if err := user.SetPassword(req.Password); err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	id, err := s.db.CreateUser(user.Username, user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	user.ID = id

	// Generate token
	token, err := s.jwtManager.GenerateToken(user.ID, user.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &models.LoginResponse{
		Token: token,
		User:  user.ToResponse(),
	}, nil
}

// Login authenticates a user and returns a token
func (s *AuthService) Login(req *models.LoginRequest) (*models.LoginResponse, error) {
	// Get user by username
	user, err := s.db.GetUserByUsername(req.Username)
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	// Verify password
	if !user.VerifyPassword(req.Password) {
		return nil, ErrInvalidPassword
	}

	// Generate token
	token, err := s.jwtManager.GenerateToken(user.ID, user.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &models.LoginResponse{
		Token: token,
		User:  user.ToResponse(),
	}, nil
}

// ValidateToken validates a token and returns the claims
func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	return s.jwtManager.ValidateToken(tokenString)
}

// validateUsername validates a username
func validateUsername(username string) error {
	if len(username) < 3 || len(username) > 50 {
		return ErrInvalidUsername
	}
	// Only allow alphanumeric and underscore
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_]+$`, username)
	if !matched {
		return ErrInvalidUsername
	}
	return nil
}

// validatePassword validates password strength
func validatePassword(password string) error {
	if len(password) < 6 {
		return ErrWeakPassword
	}
	return nil
}
