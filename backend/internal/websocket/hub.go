package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// OriginChecker handles WebSocket origin validation
type OriginChecker struct {
	allowedOrigins  map[string]bool
	developmentMode bool
	mu              sync.RWMutex
}

// NewOriginChecker creates a new origin checker with the specified allowed origins
func NewOriginChecker(allowedOrigins []string, developmentMode bool) *OriginChecker {
	originMap := make(map[string]bool)
	for _, origin := range allowedOrigins {
		originMap[strings.ToLower(origin)] = true
	}

	return &OriginChecker{
		allowedOrigins:  originMap,
		developmentMode: developmentMode,
	}
}

// IsOriginAllowed checks if the origin is allowed
func (oc *OriginChecker) IsOriginAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// No Origin header - allow for same-origin requests
		return true
	}

	oc.mu.RLock()
	defer oc.mu.RUnlock()

	// Check if origin is explicitly allowed
	if oc.allowedOrigins["*"] {
		return true
	}

	normalizedOrigin := strings.ToLower(origin)
	if oc.allowedOrigins[normalizedOrigin] {
		return true
	}

	// In development mode, allow localhost origins
	if oc.developmentMode {
		return isLocalhostOrigin(origin)
	}

	return false
}

// isLocalhostOrigin checks if the origin is a localhost origin
func isLocalhostOrigin(origin string) bool {
	origin = strings.ToLower(origin)
	return strings.HasPrefix(origin, "http://localhost") ||
		strings.HasPrefix(origin, "http://127.0.0.1") ||
		strings.HasPrefix(origin, "http://[::1]")
}

// UpdateAllowedOrigins updates the allowed origins
func (oc *OriginChecker) UpdateAllowedOrigins(origins []string) {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	oc.allowedOrigins = make(map[string]bool)
	for _, origin := range origins {
		oc.allowedOrigins[strings.ToLower(origin)] = true
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Default behavior - will be replaced by configurable origin checker
		return true
	},
}

// Message represents a WebSocket message
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Client represents a connected WebSocket client
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	userID   int64
	workerID int
}

// NewClient creates a new Client instance for testing
func NewClient(hub *Hub, userID int64) *Client {
	return &Client{
		hub:    hub,
		userID: userID,
		send:   make(chan []byte, 256),
	}
}

// GetUserID returns the client user ID
func (c *Client) GetUserID() int64 {
	return c.userID
}

// GetSendChannel returns the client's send channel
func (c *Client) GetSendChannel() chan []byte {
	return c.send
}

// IsConnected checks if the client is still connected
func (c *Client) IsConnected() bool {
	return c.conn != nil
}

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	clients       map[*Client]bool
	broadcast     chan []byte
	register      chan *Client
	unregister    chan *Client
	mu            sync.RWMutex
	originChecker *OriginChecker
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

// NewHubWithConfig creates a new Hub instance with origin checking enabled
func NewHubWithConfig(allowedOrigins []string, developmentMode bool) *Hub {
	hub := &Hub{
		broadcast:     make(chan []byte),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		clients:       make(map[*Client]bool),
		originChecker: NewOriginChecker(allowedOrigins, developmentMode),
	}

	// Update the upgrader to use our origin checker
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return hub.originChecker.IsOriginAllowed(r)
	}

	return hub
}

// SetAllowedOrigins updates the allowed origins at runtime
func (h *Hub) SetAllowedOrigins(origins []string) {
	if h.originChecker != nil {
		h.originChecker.UpdateAllowedOrigins(origins)
	}
}

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetClients returns a snapshot of connected clients
func (h *Hub) GetClients() map[*Client]bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	clientsCopy := make(map[*Client]bool)
	for client := range h.clients {
		clientsCopy[client] = true
	}
	return clientsCopy
}

// GetBroadcastChannel returns the broadcast channel for testing
func (h *Hub) GetBroadcastChannel() chan []byte {
	return h.broadcast
}

// GetRegisterChannel returns the register channel for testing
func (h *Hub) GetRegisterChannel() chan *Client {
	return h.register
}

// GetUnregisterChannel returns the unregister channel for testing
func (h *Hub) GetUnregisterChannel() chan *Client {
	return h.unregister
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// HandleConnection handles a new WebSocket connection
func (h *Hub) HandleConnection(w http.ResponseWriter, r *http.Request, userID int64) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	client := &Client{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: userID,
	}

	h.register <- client

	// Start read/write goroutines
	go client.writePump()
	go client.readPump()
}

// BroadcastToUser sends a message to a specific user
func (h *Hub) BroadcastToUser(userID int64, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.userID == userID {
			client.send <- message
		}
	}
}

// BroadcastJobUpdate sends a job update to the appropriate user
func (h *Hub) BroadcastJobUpdate(userID int64, jobID string, batchID string, progress float64, status string) {
	data := map[string]interface{}{
		"type": "job_update",
		"payload": map[string]interface{}{
			"job_id":   jobID,
			"batch_id": batchID,
			"progress": progress,
			"status":   status,
		},
	}

	msg, _ := json.Marshal(data)
	h.BroadcastToUser(userID, msg)
}

// BroadcastJobUpdateWithGranular sends a job update with granular status to the appropriate user
func (h *Hub) BroadcastJobUpdateWithGranular(userID int64, jobID string, batchID string, progress float64, status, songStatus, lyricsStatus, coverStatus string) {
	data := map[string]interface{}{
		"type": "job_update",
		"payload": map[string]interface{}{
			"job_id":        jobID,
			"batch_id":      batchID,
			"progress":      progress,
			"status":        status,
			"song_status":   songStatus,
			"lyrics_status": lyricsStatus,
			"cover_status":  coverStatus,
		},
	}

	msg, _ := json.Marshal(data)
	h.BroadcastToUser(userID, msg)
}

// BroadcastBatchUpdate sends a batch update to the appropriate user
func (h *Hub) BroadcastBatchUpdate(userID int64, batchID string, completedJobs, failedJobs int) {
	data := map[string]interface{}{
		"type": "batch_update",
		"payload": map[string]interface{}{
			"batch_id":       batchID,
			"completed_jobs": completedJobs,
			"failed_jobs":    failedJobs,
		},
	}

	msg, _ := json.Marshal(data)
	h.BroadcastToUser(userID, msg)
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle incoming messages (e.g., subscribe to specific jobs/batches)
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Invalid message format: %v", err)
			continue
		}

		// Process message based on type
		switch msg.Type {
		case "subscribe_job":
			// Handle job subscription
		case "subscribe_batch":
			// Handle batch subscription
		}
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
