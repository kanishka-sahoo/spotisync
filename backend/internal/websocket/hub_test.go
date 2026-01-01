package websocket_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"spotisync/internal/websocket"
)

// TestHubCreation tests hub creation
func TestHubCreation(t *testing.T) {
	hub := websocket.NewHub()

	assert.NotNil(t, hub)
	assert.Equal(t, 0, hub.ClientCount())
}

// TestHubRun tests hub run method (starts the hub)
func TestHubRun(t *testing.T) {
	hub := websocket.NewHub()

	// Start hub in a goroutine
	go hub.Run()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Hub should be running (channels should be ready)
	assert.NotNil(t, hub)
}

// TestHubClientRegistration tests client registration
func TestHubClientRegistration(t *testing.T) {
	hub := websocket.NewHub()

	// Start hub in a goroutine
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Create a mock client
	client := websocket.NewClient(hub, 1)

	// Register the client
	hub.GetRegisterChannel() <- client

	// Wait for registration to process
	time.Sleep(10 * time.Millisecond)

	// Check that client is registered
	assert.Equal(t, 1, hub.ClientCount())
}

// TestHubClientUnregistration tests client unregistration
func TestHubClientUnregistration(t *testing.T) {
	hub := websocket.NewHub()

	// Start hub in a goroutine
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Create a mock client
	client := websocket.NewClient(hub, 1)

	// Register the client
	hub.GetRegisterChannel() <- client
	time.Sleep(10 * time.Millisecond)

	// Unregister the client
	hub.GetUnregisterChannel() <- client
	time.Sleep(10 * time.Millisecond)

	// Check that client is unregistered
	assert.Equal(t, 0, hub.ClientCount())
}

// TestHubBroadcastMessaging tests broadcast messaging
func TestHubBroadcastMessaging(t *testing.T) {
	hub := websocket.NewHub()

	// Start hub in a goroutine
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Create mock clients
	client1 := websocket.NewClient(hub, 1)
	client2 := websocket.NewClient(hub, 2)

	// Register clients
	hub.GetRegisterChannel() <- client1
	hub.GetRegisterChannel() <- client2
	time.Sleep(10 * time.Millisecond)

	// Broadcast a message
	testMessage := []byte(`{"type": "test", "data": "hello"}`)
	hub.GetBroadcastChannel() <- testMessage

	// Wait for message to be sent
	time.Sleep(10 * time.Millisecond)

	// Check that both clients received the message
	select {
	case msg := <-client1.GetSendChannel():
		assert.Equal(t, testMessage, msg)
	default:
		t.Fatal("Client 1 did not receive message")
	}

	select {
	case msg := <-client2.GetSendChannel():
		assert.Equal(t, testMessage, msg)
	default:
		t.Fatal("Client 2 did not receive message")
	}
}

// TestHubBroadcastToUser tests broadcasting to a specific user
func TestHubBroadcastToUser(t *testing.T) {
	hub := websocket.NewHub()

	// Start hub in a goroutine
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Create mock clients for different users
	client1 := websocket.NewClient(hub, 1)
	client2 := websocket.NewClient(hub, 2)

	// Register clients
	hub.GetRegisterChannel() <- client1
	hub.GetRegisterChannel() <- client2
	time.Sleep(10 * time.Millisecond)

	// Broadcast to user 1 only
	testMessage := []byte(`{"type": "test", "data": "user1 only"}`)
	hub.BroadcastToUser(1, testMessage)

	// Wait for message to be sent
	time.Sleep(10 * time.Millisecond)

	// Check that only client1 received the message
	select {
	case msg := <-client1.GetSendChannel():
		assert.Equal(t, testMessage, msg)
	default:
		t.Fatal("Client 1 did not receive message")
	}

	// Client2 should not have received the message
	select {
	case <-client2.GetSendChannel():
		t.Fatal("Client 2 should not have received the message")
	default:
		// Expected - no message
	}
}

// TestHubBroadcastJobUpdate tests broadcasting job updates
func TestHubBroadcastJobUpdate(t *testing.T) {
	hub := websocket.NewHub()

	// Start hub in a goroutine
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Create a mock client
	client := websocket.NewClient(hub, 1)

	// Register client
	hub.GetRegisterChannel() <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast job update
	hub.BroadcastJobUpdate(1, "job-123", "batch-456", 0.5, "in_progress")

	// Wait for message to be sent
	time.Sleep(10 * time.Millisecond)

	// Check that client received the message
	select {
	case msg := <-client.GetSendChannel():
		var data map[string]interface{}
		err := json.Unmarshal(msg, &data)
		require.NoError(t, err)

		assert.Equal(t, "job_update", data["type"])
		payload := data["payload"].(map[string]interface{})
		assert.Equal(t, "job-123", payload["job_id"])
		assert.Equal(t, "batch-456", payload["batch_id"])
		assert.Equal(t, 0.5, payload["progress"])
		assert.Equal(t, "in_progress", payload["status"])
	default:
		t.Fatal("Client did not receive job update")
	}
}

// TestHubBroadcastBatchUpdate tests broadcasting batch updates
func TestHubBroadcastBatchUpdate(t *testing.T) {
	hub := websocket.NewHub()

	// Start hub in a goroutine
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Create a mock client
	client := websocket.NewClient(hub, 1)

	// Register client
	hub.GetRegisterChannel() <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast batch update
	hub.BroadcastBatchUpdate(1, "batch-123", 5, 2)

	// Wait for message to be sent
	time.Sleep(10 * time.Millisecond)

	// Check that client received the message
	select {
	case msg := <-client.GetSendChannel():
		var data map[string]interface{}
		err := json.Unmarshal(msg, &data)
		require.NoError(t, err)

		assert.Equal(t, "batch_update", data["type"])
		payload := data["payload"].(map[string]interface{})
		assert.Equal(t, "batch-123", payload["batch_id"])
		assert.Equal(t, float64(5), payload["completed_jobs"])
		assert.Equal(t, float64(2), payload["failed_jobs"])
	default:
		t.Fatal("Client did not receive batch update")
	}
}

// TestHubMultipleClientsForSameUser tests multiple clients for the same user
func TestHubMultipleClientsForSameUser(t *testing.T) {
	hub := websocket.NewHub()

	// Start hub in a goroutine
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Create multiple clients for the same user
	client1 := websocket.NewClient(hub, 1)
	client2 := websocket.NewClient(hub, 1)

	// Register clients
	hub.GetRegisterChannel() <- client1
	hub.GetRegisterChannel() <- client2
	time.Sleep(10 * time.Millisecond)

	// Broadcast to user 1
	testMessage := []byte(`{"type": "test", "data": "all clients"}`)
	hub.BroadcastToUser(1, testMessage)

	// Wait for messages to be sent
	time.Sleep(10 * time.Millisecond)

	// Check that both clients received the message
	select {
	case msg := <-client1.GetSendChannel():
		assert.Equal(t, testMessage, msg)
	default:
		t.Fatal("Client 1 did not receive message")
	}

	select {
	case msg := <-client2.GetSendChannel():
		assert.Equal(t, testMessage, msg)
	default:
		t.Fatal("Client 2 did not receive message")
	}
}

// TestHubMessageFormat tests message format structure
func TestHubMessageFormat(t *testing.T) {
	hub := websocket.NewHub()

	// Start hub in a goroutine
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Create a mock client
	client := websocket.NewClient(hub, 1)

	// Register client
	hub.GetRegisterChannel() <- client
	time.Sleep(10 * time.Millisecond)

	// Create a custom message
	message := websocket.Message{
		Type:    "custom_type",
		Payload: json.RawMessage(`{"key": "value"}`),
	}
	msgBytes, _ := json.Marshal(message)

	// Broadcast the message
	hub.GetBroadcastChannel() <- msgBytes

	// Wait for message to be sent
	time.Sleep(10 * time.Millisecond)

	// Check that client received the message
	select {
	case msg := <-client.GetSendChannel():
		var receivedMessage websocket.Message
		err := json.Unmarshal(msg, &receivedMessage)
		require.NoError(t, err)

		assert.Equal(t, "custom_type", receivedMessage.Type)
		assert.JSONEq(t, `{"key": "value"}`, string(receivedMessage.Payload))
	default:
		t.Fatal("Client did not receive message")
	}
}

// TestClientGetUserID tests client GetUserID method
func TestClientGetUserID(t *testing.T) {
	client := websocket.NewClient(nil, 123)

	assert.Equal(t, int64(123), client.GetUserID())
}

// TestClientGetSendChannel tests client GetSendChannel method
func TestClientGetSendChannel(t *testing.T) {
	client := websocket.NewClient(nil, 1)

	assert.NotNil(t, client.GetSendChannel())
}

// TestHubGetClients tests hub GetClients method
func TestHubGetClients(t *testing.T) {
	hub := websocket.NewHub()

	// Start hub in a goroutine
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Create mock clients
	client1 := websocket.NewClient(hub, 1)
	client2 := websocket.NewClient(hub, 2)

	// Register clients
	hub.GetRegisterChannel() <- client1
	hub.GetRegisterChannel() <- client2
	time.Sleep(10 * time.Millisecond)

	// Get clients snapshot
	clients := hub.GetClients()
	assert.Equal(t, 2, len(clients))
	assert.True(t, clients[client1])
	assert.True(t, clients[client2])
}

// TestHubClientCount tests hub ClientCount method
func TestHubClientCount(t *testing.T) {
	hub := websocket.NewHub()

	// Start hub in a goroutine
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Initially no clients
	assert.Equal(t, 0, hub.ClientCount())

	// Add a client
	client := websocket.NewClient(hub, 1)
	hub.GetRegisterChannel() <- client
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 1, hub.ClientCount())

	// Add another client
	client2 := websocket.NewClient(hub, 2)
	hub.GetRegisterChannel() <- client2
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 2, hub.ClientCount())

	// Remove a client
	hub.GetUnregisterChannel() <- client
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 1, hub.ClientCount())
}
