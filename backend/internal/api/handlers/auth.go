package handlers

import (
	"encoding/json"
	"net/http"
	"spotisync/internal/api/middleware"
	"spotisync/internal/auth"
	"spotisync/internal/db"
	"spotisync/internal/db/models"
)

// LoginRequest represents a login request with custom unmarshaling
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Custom unmarshaler to distinguish between missing and empty fields
func (r *LoginRequest) UnmarshalJSON(data []byte) error {
	// First try to unmarshal as map to detect missing fields
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Check if password field is missing
	if _, ok := raw["password"]; !ok {
		return &ValidationError{Message: "password is required"}
	}

	// Now unmarshal normally
	type Alias LoginRequest
	aux := (*Alias)(r)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	return nil
}

// ValidationError represents a validation error
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	authService *auth.AuthService
	db          *db.Database
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService *auth.AuthService, database *db.Database) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		db:          database,
	}
}

// Login handles POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Check if it's a validation error for missing password
		if ve, ok := err.(*ValidationError); ok {
			http.Error(w, ve.Message, http.StatusBadRequest)
			return
		}
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate input - return 400 for validation errors
	// Empty username/password are auth errors (401), but missing fields are 400
	if req.Username == "" {
		http.Error(w, "invalid username or password", http.StatusUnauthorized)
		return
	}
	if req.Password == "" {
		http.Error(w, "invalid username or password", http.StatusUnauthorized)
		return
	}
	if len(req.Password) < 6 {
		http.Error(w, "invalid password: must be at least 6 characters", http.StatusBadRequest)
		return
	}
	if len(req.Username) < 3 || len(req.Username) > 50 {
		http.Error(w, "invalid username: must be 3-50 characters", http.StatusBadRequest)
		return
	}

	// Convert to models.LoginRequest for auth service
	modelsReq := &models.LoginRequest{
		Username: req.Username,
		Password: req.Password,
	}

	resp, err := h.authService.Login(modelsReq)
	if err != nil {
		switch err {
		case auth.ErrUserNotFound:
			http.Error(w, "invalid username or password", http.StatusUnauthorized)
		case auth.ErrInvalidPassword:
			http.Error(w, "invalid username or password", http.StatusUnauthorized)
		default:
			http.Error(w, "login failed: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Register handles POST /api/v1/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := h.authService.Register(&req)
	if err != nil {
		switch err {
		case auth.ErrUserExists:
			http.Error(w, "username already taken", http.StatusConflict)
		case auth.ErrInvalidUsername:
			http.Error(w, "invalid username: must be 3-50 characters and contain only alphanumeric and underscores", http.StatusBadRequest)
		case auth.ErrWeakPassword:
			http.Error(w, "invalid password: must be at least 6 characters", http.StatusBadRequest)
		default:
			http.Error(w, "registration failed: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Me handles GET /api/v1/auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.db.GetUserByID(userID)
	if err != nil {
		http.Error(w, "failed to get user: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user.ToResponse())
}
