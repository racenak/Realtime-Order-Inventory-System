package websocket_test

import (
	"testing"
	"time"

	"github.com/racenak/Realtime-Order-Inventory-System/pkg/websocket"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func newTestHub() *websocket.Hub {
	logger, _ := zap.NewDevelopment()
	return websocket.NewHub(logger)
}

func newTestClient(hub *websocket.Hub, channels ...string) *websocket.Client {
	logger, _ := zap.NewDevelopment()
	client := websocket.NewClient(nil, hub, logger)
	for _, ch := range channels {
		client.AddChannel(ch)
	}
	return client
}

func TestHub_NewHub(t *testing.T) {
	hub := newTestHub()
	assert.NotNil(t, hub)
	assert.Equal(t, 0, hub.ClientCount())
}

func TestHub_RegisterAndUnregister(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	client := newTestClient(hub, "orders")

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 1, hub.ClientCount())

	hub.Unregister(client)
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 0, hub.ClientCount())
}

func TestHub_MultipleClients(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	clients := make([]*websocket.Client, 5)
	for i := 0; i < 5; i++ {
		clients[i] = newTestClient(hub, "orders")
		hub.Register(clients[i])
	}

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 5, hub.ClientCount())

	hub.Unregister(clients[0])
	hub.Unregister(clients[3])
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 3, hub.ClientCount())
}

func TestHub_GetStats(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	client1 := newTestClient(hub, "orders")
	client2 := newTestClient(hub, "inventory")

	hub.Register(client1)
	hub.Register(client2)
	time.Sleep(50 * time.Millisecond)

	stats := hub.GetStats()
	assert.Equal(t, 2, stats["total_clients"])
	channels := stats["channels"].(map[string]int)
	assert.Equal(t, 1, channels["orders"])
	assert.Equal(t, 1, channels["inventory"])
}

func TestHub_BroadcastToChannel_MultipleSubscribers(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	client1 := newTestClient(hub, "orders")
	client2 := newTestClient(hub, "orders")
	client3 := newTestClient(hub, "inventory")

	hub.Register(client1)
	hub.Register(client2)
	hub.Register(client3)
	time.Sleep(50 * time.Millisecond)

	// Broadcast to "orders" channel - only client1 and client2 should receive
	hub.BroadcastToChannel("orders", websocket.Message{
		Type:    "order.created",
		Payload: map[string]string{"order_id": "123"},
	})

	// Verify clients are still registered (no panic)
	assert.Equal(t, 3, hub.ClientCount())
}

func TestClient_HasChannel(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	hub := newTestHub()
	client := websocket.NewClient(nil, hub, logger)

	assert.False(t, client.HasChannel("orders"))

	client.AddChannel("orders")
	assert.True(t, client.HasChannel("orders"))
	assert.False(t, client.HasChannel("inventory"))

	client.RemoveChannel("orders")
	assert.False(t, client.HasChannel("orders"))
}

func TestClient_ID(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	hub := newTestHub()
	client := websocket.NewClient(nil, hub, logger)

	assert.NotEmpty(t, client.ID())
	assert.Len(t, client.ID(), 36) // UUID format
}
