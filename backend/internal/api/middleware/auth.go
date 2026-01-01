package middleware

import (
	"context"
	"net/http"
	"spotisync/internal/auth"
	"strings"
)

// ContextKey is a type for context keys
type ContextKey string

const (
	// UserIDKey is the context key for user ID
	UserIDKey ContextKey = "user_id"
	// UsernameKey is the context key for username
	UsernameKey ContextKey = "username"
)

// AuthMiddleware creates JWT authentication middleware
func AuthMiddleware(authService *auth.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header or query param
			var token string

			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				// Check Bearer prefix
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) != 2 || parts[0] != "Bearer" {
					http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
					return
				}
				token = parts[1]
			} else {
				// Try query parameter (for WebSocket connections)
				token = r.URL.Query().Get("token")
			}

			if token == "" {
				http.Error(w, "missing authorization token", http.StatusUnauthorized)
				return
			}

			// Validate token
			claims, err := authService.ValidateToken(token)
			if err != nil {
				http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
				return
			}

			// Add user ID and username to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, UsernameKey, claims.Username)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID retrieves the user ID from context
func GetUserID(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(UserIDKey).(int64)
	return userID, ok
}

// GetUsername retrieves the username from context
func GetUsername(ctx context.Context) (string, bool) {
	username, ok := ctx.Value(UsernameKey).(string)
	return username, ok
}
