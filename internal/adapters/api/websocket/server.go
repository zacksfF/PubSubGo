package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/zacksfF/PubSubGo/internal/config"
	"github.com/zacksfF/PubSubGo/internal/services/publisher"
	"github.com/zacksfF/PubSubGo/internal/services/subscriber"
	"github.com/zacksfF/PubSubGo/internal/services/topic"
)

// Server represents the WebSocket server
type Server struct {
	publisherSvc  publisher.Service
	subscriberSvc subscriber.Service
	topicSvc      topic.Service
	config        *config.BrokerConfig
	logger        *logrus.Logger

	// Connection management
	connections map[string]*Connection
	connMutex   sync.RWMutex

	// Subscription management
	subscriptions map[string]map[string]*Subscription // topic -> connection_id -> subscription
	subsMutex     sync.RWMutex
}

// Connection represents a WebSocket connection
type Connection struct {
	ID           string
	RemoteAddr   string
	UserAgent    string
	ConnectedAt  time.Time
	LastPingAt   time.Time
	IsAlive      bool
	
	// Communication channels
	send      chan []byte
	receive   chan []byte
	close     chan struct{}
	
	// Connection context
	ctx    context.Context
	cancel context.CancelFunc
	
	// Subscriptions for this connection
	subscriptions map[string]*Subscription
	subsMutex     sync.RWMutex
	
	// WebSocket connection
	conn   *websocket.Conn
	server *Server
}

// Subscription represents a topic subscription
type Subscription struct {
	ID            string
	ConnectionID  string
	Topic         string
	ConsumerGroup string
	Filter        string
	CreatedAt     time.Time
	MessageCount  int64
	LastMessage   *time.Time
}

// Message types for WebSocket communication
type MessageType string

const (
	// Client to Server messages
	MsgTypeSubscribe   MessageType = "subscribe"
	MsgTypeUnsubscribe MessageType = "unsubscribe"
	MsgTypePublish     MessageType = "publish"
	MsgTypeAck         MessageType = "ack"
	MsgTypePing        MessageType = "ping"
	
	// Server to Client messages
	MsgTypeMessage     MessageType = "message"
	MsgTypeError       MessageType = "error"
	MsgTypePong        MessageType = "pong"
	MsgTypeSubscribed  MessageType = "subscribed"
	MsgTypeUnsubscribed MessageType = "unsubscribed"
	MsgTypePublished   MessageType = "published"
)

// WebSocket message structure
type WSMessage struct {
	Type      MessageType     `json:"type"`
	ID        string          `json:"id,omitempty"`        // Message ID for tracking
	Topic     string          `json:"topic,omitempty"`     // Topic name
	Payload   json.RawMessage `json:"payload,omitempty"`   // Message payload
	Headers   map[string]string `json:"headers,omitempty"` // Message headers
	Timestamp time.Time       `json:"timestamp"`
	Error     string          `json:"error,omitempty"`     // Error message
}

// Subscribe message payload
type SubscribePayload struct {
	Topic         string `json:"topic"`
	ConsumerGroup string `json:"consumer_group,omitempty"`
	Filter        string `json:"filter,omitempty"`
	MaxMessages   int    `json:"max_messages,omitempty"`
}

// Publish message payload
type PublishPayload struct {
	Key      string            `json:"key,omitempty"`
	Payload  string            `json:"payload"`            // Base64 encoded
	Headers  map[string]string `json:"headers,omitempty"`
	Priority string            `json:"priority,omitempty"`
}

// Message delivery payload
type MessagePayload struct {
	ID            string            `json:"id"`
	Key           string            `json:"key,omitempty"`
	Payload       string            `json:"payload"`            // Base64 encoded
	Headers       map[string]string `json:"headers,omitempty"`
	Priority      string            `json:"priority"`
	DeliveryMode  string            `json:"delivery_mode"`
	Topic         string            `json:"topic"`
	Partition     int32             `json:"partition"`
	Offset        int64             `json:"offset"`
	ConsumerGroup string            `json:"consumer_group,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	ExpiresAt     *time.Time        `json:"expires_at,omitempty"`
}

// NewServer creates a new WebSocket server
func NewServer(
	publisherSvc publisher.Service,
	subscriberSvc subscriber.Service,
	topicSvc topic.Service,
	config *config.BrokerConfig,
	logger *logrus.Logger,
) *Server {
	return &Server{
		publisherSvc:  publisherSvc,
		subscriberSvc: subscriberSvc,
		topicSvc:      topicSvc,
		config:        config,
		logger:        logger,
		connections:   make(map[string]*Connection),
		subscriptions: make(map[string]map[string]*Subscription),
	}
}

// HandleWebSocket handles WebSocket upgrade and connection management
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := s.upgradeConnection(w, r)
	if err != nil {
		s.logger.WithError(err).Error("Failed to upgrade WebSocket connection")
		http.Error(w, "Failed to upgrade connection", http.StatusBadRequest)
		return
	}

	// Create connection object
	wsConn := s.createConnection(conn, r)
	
	// Register connection
	s.registerConnection(wsConn)
	
	// Start connection handlers
	go wsConn.readPump()
	go wsConn.writePump()
	
	// Wait for connection to close
	<-wsConn.close
	
	// Cleanup
	s.unregisterConnection(wsConn)
}

// GetConnectionStats returns connection statistics
func (s *Server) GetConnectionStats() map[string]interface{} {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()
	
	totalConnections := len(s.connections)
	aliveConnections := 0
	
	for _, conn := range s.connections {
		if conn.IsAlive {
			aliveConnections++
		}
	}
	
	s.subsMutex.RLock()
	totalSubscriptions := 0
	for _, topicSubs := range s.subscriptions {
		totalSubscriptions += len(topicSubs)
	}
	s.subsMutex.RUnlock()
	
	return map[string]interface{}{
		"total_connections":    totalConnections,
		"alive_connections":    aliveConnections,
		"total_subscriptions":  totalSubscriptions,
		"topics_subscribed":    len(s.subscriptions),
	}
}

// BroadcastToTopic broadcasts a message to all subscribers of a topic
func (s *Server) BroadcastToTopic(topic string, message *MessagePayload) {
	s.subsMutex.RLock()
	topicSubs, exists := s.subscriptions[topic]
	if !exists {
		s.subsMutex.RUnlock()
		return
	}
	
	// Create list of connections to broadcast to
	connIDs := make([]string, 0, len(topicSubs))
	for connID := range topicSubs {
		connIDs = append(connIDs, connID)
	}
	s.subsMutex.RUnlock()
	
	// Broadcast to each connection
	wsMsg := &WSMessage{
		Type:      MsgTypeMessage,
		Topic:     topic,
		Timestamp: time.Now(),
	}
	
	payload, err := json.Marshal(message)
	if err != nil {
		s.logger.WithError(err).Error("Failed to marshal message payload")
		return
	}
	wsMsg.Payload = payload
	
	s.connMutex.RLock()
	for _, connID := range connIDs {
		if conn, exists := s.connections[connID]; exists && conn.IsAlive {
			conn.sendMessage(wsMsg)
		}
	}
	s.connMutex.RUnlock()
}

// Shutdown gracefully shuts down the WebSocket server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down WebSocket server")
	
	s.connMutex.RLock()
	connections := make([]*Connection, 0, len(s.connections))
	for _, conn := range s.connections {
		connections = append(connections, conn)
	}
	s.connMutex.RUnlock()
	
	// Close all connections
	for _, conn := range connections {
		conn.Close()
	}
	
	s.logger.Info("WebSocket server shutdown complete")
	return nil
}

// Helper methods

func (s *Server) registerConnection(conn *Connection) {
	s.connMutex.Lock()
	s.connections[conn.ID] = conn
	s.connMutex.Unlock()
	
	s.logger.WithFields(logrus.Fields{
		"connection_id": conn.ID,
		"remote_addr":   conn.RemoteAddr,
	}).Info("WebSocket connection registered")
}

func (s *Server) unregisterConnection(conn *Connection) {
	// Remove from connections
	s.connMutex.Lock()
	delete(s.connections, conn.ID)
	s.connMutex.Unlock()
	
	// Remove all subscriptions for this connection
	s.removeAllSubscriptions(conn.ID)
	
	s.logger.WithFields(logrus.Fields{
		"connection_id": conn.ID,
		"duration":      time.Since(conn.ConnectedAt),
	}).Info("WebSocket connection unregistered")
}

func (s *Server) removeAllSubscriptions(connectionID string) {
	s.subsMutex.Lock()
	defer s.subsMutex.Unlock()
	
	for topic, topicSubs := range s.subscriptions {
		delete(topicSubs, connectionID)
		if len(topicSubs) == 0 {
			delete(s.subscriptions, topic)
		}
	}
}