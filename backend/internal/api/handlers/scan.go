package handlers

import (
	"encoding/json"
	"net/http"
	"spotisync/internal/api/middleware"
	"spotisync/internal/config"
	"spotisync/internal/db"
	"spotisync/internal/services/navidrome"
)

// ScanResponse represents the response for scan operations
type ScanResponse struct {
	Message  string `json:"message"`
	Scanning bool   `json:"scanning"`
	Count    int64  `json:"count,omitempty"`
}

// ScanHandler handles scan-related endpoints
type ScanHandler struct {
	db  *db.Database
	cfg *config.Config
}

// NewScanHandler creates a new scan handler
func NewScanHandler(database *db.Database, cfg *config.Config) *ScanHandler {
	return &ScanHandler{
		db:  database,
		cfg: cfg,
	}
}

// TriggerScan handles POST /api/v1/scan
func (h *ScanHandler) TriggerScan(w http.ResponseWriter, r *http.Request) {
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

	// Check if Navidrome credentials are configured
	if user.NavidromeURL == "" || user.NavidromeUsername == "" {
		http.Error(w, "Navidrome not configured. Please set your Navidrome credentials in settings.", http.StatusBadRequest)
		return
	}

	// Create Navidrome client with user's credentials
	client := navidrome.NewClient(user.NavidromeURL, user.NavidromeUsername, user.NavidromePassword)

	// Check current scan status first
	status, err := client.GetScanStatus(r.Context())
	if err != nil {
		http.Error(w, "failed to get scan status: "+err.Error(), http.StatusBadGateway)
		return
	}

	// If already scanning, return current status
	if status.Scanning {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ScanResponse{
			Message:  "scan already in progress",
			Scanning: true,
			Count:    status.Count,
		})
		return
	}

	// Start the scan
	if err := client.StartScan(r.Context()); err != nil {
		http.Error(w, "failed to start scan: "+err.Error(), http.StatusBadGateway)
		return
	}

	// Get updated status after starting scan
	status, err = client.GetScanStatus(r.Context())
	if err != nil {
		// Scan was started but we couldn't get status - still consider it a success
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ScanResponse{
			Message:  "scan triggered successfully",
			Scanning: true,
			Count:    0,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ScanResponse{
		Message:  "scan triggered successfully",
		Scanning: status.Scanning,
		Count:    status.Count,
	})
}
