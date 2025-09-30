package helpers

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/zacksfF/PubSubGo/internal/core/message"
)

// MessageGenerator generates test messages
type MessageGenerator struct {
	rand *rand.Rand
}

// NewMessageGenerator creates a new message generator
func NewMessageGenerator(seed int64) *MessageGenerator {
	return &MessageGenerator{
		rand: rand.New(rand.NewSource(seed)),
	}
}

// GenerateMessage generates a random test message
func (g *MessageGenerator) GenerateMessage() *message.Message {
	priorities := []message.Priority{
		message.PriorityLow,
		message.PriorityNormal,
		message.PriorityHigh,
		message.PriorityCritical,
	}

	deliveryModes := []message.DeliveryMode{
		message.DeliveryAtMostOnce,
		message.DeliveryAtLeastOnce,
		message.DeliveryExactlyOnce,
	}

	return &message.Message{
		ID:           uuid.New().String(),
		Topic:        g.RandomTopic(),
		Key:          []byte(g.RandomKey()),
		Payload:      g.RandomPayload(100),
		Headers:      g.RandomHeaders(),
		Priority:     priorities[g.rand.Intn(len(priorities))],
		DeliveryMode: deliveryModes[g.rand.Intn(len(deliveryModes))],
		Status:       message.StatusPending,
		CreatedAt:    time.Now(),
	}
}

// RandomTopic generates a random topic name
func (g *MessageGenerator) RandomTopic() string {
	topics := []string{
		"events", "logs", "metrics", "notifications",
		"orders", "users", "payments", "inventory",
	}
	return topics[g.rand.Intn(len(topics))]
}

// RandomKey generates a random partition key
func (g *MessageGenerator) RandomKey() string {
	if g.rand.Float32() < 0.3 {
		return "" // 30% chance of no key
	}
	return fmt.Sprintf("key-%d", g.rand.Intn(1000))
}

// RandomPayload generates random payload data
func (g *MessageGenerator) RandomPayload(size int) []byte {
	payload := make([]byte, size)
	g.rand.Read(payload)
	return payload
}

// RandomHeaders generates random headers
func (g *MessageGenerator) RandomHeaders() map[string]string {
	if g.rand.Float32() < 0.5 {
		return nil // 50% chance of no headers
	}

	headers := make(map[string]string)
	numHeaders := g.rand.Intn(5) + 1

	for i := 0; i < numHeaders; i++ {
		key := fmt.Sprintf("header-%d", i)
		value := fmt.Sprintf("value-%d", g.rand.Intn(100))
		headers[key] = value
	}

	return headers
}

// GenerateBatch generates a batch of test messages
func (g *MessageGenerator) GenerateBatch(size int) []*message.Message {
	messages := make([]*message.Message, size)
	for i := 0; i < size; i++ {
		messages[i] = g.GenerateMessage()
	}
	return messages
}

// CreatePublishRequest creates a publish request for testing
func CreatePublishRequest(payload string, options ...func(*PublishRequest)) *PublishRequest {
	req := &PublishRequest{
		Payload:      base64.StdEncoding.EncodeToString([]byte(payload)),
		Priority:     "normal",
		DeliveryMode: "at_least_once",
	}

	for _, opt := range options {
		opt(req)
	}

	return req
}

// PublishRequest represents a publish request
type PublishRequest struct {
	Key          string            `json:"key,omitempty"`
	Payload      string            `json:"payload"`
	Headers      map[string]string `json:"headers,omitempty"`
	Priority     string            `json:"priority,omitempty"`
	DeliveryMode string            `json:"delivery_mode,omitempty"`
	TTL          *int64            `json:"ttl,omitempty"`
}

// WithKey sets the key for a publish request
func WithKey(key string) func(*PublishRequest) {
	return func(r *PublishRequest) {
		r.Key = key
	}
}

// WithHeaders sets headers for a publish request
func WithHeaders(headers map[string]string) func(*PublishRequest) {
	return func(r *PublishRequest) {
		r.Headers = headers
	}
}

// WithPriority sets priority for a publish request
func WithPriority(priority string) func(*PublishRequest) {
	return func(r *PublishRequest) {
		r.Priority = priority
	}
}

// WithTTL sets TTL for a publish request
func WithTTL(seconds int64) func(*PublishRequest) {
	return func(r *PublishRequest) {
		r.TTL = &seconds
	}
}
