package websocket

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	gorilla "github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

type Client struct {
	id       string
	conn     *gorilla.Conn
	send     chan []byte
	hub      *Hub
	channels map[string]bool
	mu       sync.RWMutex
	logger   *zap.Logger
}

func NewClient(conn *gorilla.Conn, hub *Hub, logger *zap.Logger) *Client {
	return &Client{
		id:       uuid.New().String(),
		conn:     conn,
		send:     make(chan []byte, 256),
		hub:      hub,
		channels: make(map[string]bool),
		logger:   logger,
	}
}

func (c *Client) ID() string {
	return c.id
}

func (c *Client) HasChannel(channel string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.channels[channel]
}

func (c *Client) AddChannel(channel string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.channels[channel] = true
}

func (c *Client) RemoveChannel(channel string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.channels, channel)
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if gorilla.IsUnexpectedCloseError(err, gorilla.CloseGoingAway, gorilla.CloseAbnormalClosure) {
				c.logger.Error("websocket error", zap.Error(err))
			}
			break
		}

		c.handleMessage(message)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(gorilla.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(gorilla.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(gorilla.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(data []byte) {
	var msg struct {
		Action   string   `json:"action"`
		Channel  string   `json:"channel,omitempty"`
		Channels []string `json:"channels,omitempty"`
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		c.logger.Error("failed to unmarshal client message", zap.Error(err))
		return
	}

	switch msg.Action {
	case "subscribe":
		if msg.Channel != "" {
			c.AddChannel(msg.Channel)
			c.logger.Debug("client subscribed",
				zap.String("client_id", c.id),
				zap.String("channel", msg.Channel),
			)
			c.sendConfirmation("subscribed", msg.Channel)
		}
		if len(msg.Channels) > 0 {
			for _, ch := range msg.Channels {
				c.AddChannel(ch)
			}
			c.sendConfirmation("subscribed", msg.Channels...)
		}

	case "unsubscribe":
		if msg.Channel != "" {
			c.RemoveChannel(msg.Channel)
			c.sendConfirmation("unsubscribed", msg.Channel)
		}

	case "ping":
		c.sendJSON(Message{Type: "pong"})

	default:
		c.logger.Debug("unknown action",
			zap.String("client_id", c.id),
			zap.String("action", msg.Action),
		)
	}
}

func (c *Client) sendConfirmation(action string, channels ...string) {
	msg := map[string]interface{}{
		"type":    "confirmation",
		"action":  action,
		"channel": channels,
	}
	c.sendJSON(msg)
}

func (c *Client) sendJSON(msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		c.logger.Error("failed to marshal message", zap.Error(err))
		return
	}

	select {
	case c.send <- data:
	default:
		c.logger.Warn("client send buffer full", zap.String("client_id", c.id))
	}
}
