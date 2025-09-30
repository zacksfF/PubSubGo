package websocket

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// WebSocket configuration
	writeWait      = 10 * time.Second    // Time allowed to write a message
	pongWait       = 60 * time.Second    // Time allowed to read the next pong message
	pingPeriod     = (pongWait * 9) / 10 // Send pings to peer with this period
	maxMessageSize = 1024 * 1024         // Maximum message size (1MB)
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from any origin - configure appropriately for production
		return true
	},
}

// upgradeConnection upgrades HTTP connection to WebSocket
func (s *Server) upgradeConnection(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade connection: %w", err)
	}
	return conn, nil
}

// createConnection creates a new Connection object
func (s *Server) createConnection(conn *websocket.Conn, r *http.Request) *Connection {
	ctx, cancel := context.WithCancel(context.Background())

	connectionID := generateConnectionID(r)

	return &Connection{
		ID:            connectionID,
		RemoteAddr:    getClientIP(r),
		UserAgent:     r.UserAgent(),
		ConnectedAt:   time.Now(),
		LastPingAt:    time.Now(),
		IsAlive:       true,
		send:          make(chan []byte, 256),
		receive:       make(chan []byte, 256),
		close:         make(chan struct{}),
		ctx:           ctx,
		cancel:        cancel,
		subscriptions: make(map[string]*Subscription),
		server:        s,
		conn:          conn,
	}
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *Connection) readPump() {
	defer func() {
		c.Close()
	}()

	// Set read deadline and message size limit
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetReadLimit(maxMessageSize)

	// Set pong handler
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			// Read message from WebSocket
			_, messageData, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.server.logger.WithError(err).Debug("WebSocket read error")
				}
				return
			}

			// Process the message
			c.handleMessage(messageData)
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Connection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.server.logger.WithError(err).Debug("WebSocket write error")
				return
			}

			// Write queued messages to the current WebSocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				if err := c.conn.WriteMessage(websocket.TextMessage, <-c.send); err != nil {
					return
				}
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
			c.LastPingAt = time.Now()
		}
	}
}

// handleMessage processes incoming WebSocket messages
func (c *Connection) handleMessage(data []byte) {
	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.sendError("Invalid message format", err)
		return
	}

	// Update last activity
	c.LastPingAt = time.Now()

	switch msg.Type {
	case MsgTypeSubscribe:
		c.handleSubscribe(&msg)
	case MsgTypeUnsubscribe:
		c.handleUnsubscribe(&msg)
	case MsgTypePublish:
		c.handlePublish(&msg)
	case MsgTypeAck:
		c.handleAck(&msg)
	case MsgTypePing:
		c.handlePing(&msg)
	default:
		c.sendError("Unknown message type", fmt.Errorf("unsupported message type: %s", msg.Type))
	}
}

// sendMessage sends a message to the WebSocket connection
func (c *Connection) sendMessage(msg *WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		c.server.logger.WithError(err).Error("Failed to marshal WebSocket message")
		return
	}

	select {
	case c.send <- data:
	case <-c.ctx.Done():
	default:
		// Channel is full, connection is slow
		c.server.logger.WithField("connection_id", c.ID).Warn("WebSocket send channel full")
		c.Close()
	}
}

// sendError sends an error message to the client
func (c *Connection) sendError(message string, err error) {
	errorMsg := &WSMessage{
		Type:      MsgTypeError,
		Error:     fmt.Sprintf("%s: %v", message, err),
		Timestamp: time.Now(),
	}
	c.sendMessage(errorMsg)
}

// Close closes the WebSocket connection
func (c *Connection) Close() {
	c.cancel()
	if !c.IsAlive {
		return
	}
	c.IsAlive = false

	close(c.close)
	close(c.send)
	c.conn.Close()
}

// Helper functions

func generateConnectionID(r *http.Request) string {
	// Generate a unique connection ID based on request info and UUID
	data := fmt.Sprintf("%s:%s:%s:%d",
		r.RemoteAddr,
		r.UserAgent(),
		uuid.New().String(),
		time.Now().UnixNano())

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes for shorter ID
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}

	return ip
}
