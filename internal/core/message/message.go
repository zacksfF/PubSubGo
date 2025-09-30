package message

import (
	"encoding/json"
	"time"
)

type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

type DeliveryMode int

const (
	DeliveryAtMostOnce DeliveryMode = iota
	DeliveryAtLeastOnce
	DeliveryExactlyOnce
)

type Status int

const (
	StatusPending Status = iota
	StatusDelivered
	StatusAcknowledged
	StatusFailed
	StatusExpired
	StatusDLQ
)

type Message struct {
	ID              string            `json:"id"`
	Topic           string            `json:"topic"`
	Partition       int32             `json:"partition,omitempty"`
	Offset          int64             `json:"offset,omitempty"`
	Key             []byte            `json:"key,omitempty"`
	Payload         []byte            `json:"payload"`
	Headers         map[string]string `json:"headers,omitempty"`
	Priority        Priority          `json:"priority"`
	DeliveryMode    DeliveryMode      `json:"delivery_mode"`
	Status          Status            `json:"status"`
	RetryCount      int               `json:"retry_count"`
	MaxRetries      int               `json:"max_retries"`
	ConsumerGroup   string            `json:"consumer_group,omitempty"`
	ConsumerID      string            `json:"consumer_id,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	ExpiresAt       *time.Time        `json:"expires_at,omitempty"`
	DeliveredAt     *time.Time        `json:"delivered_at,omitempty"`
	AckedAt         *time.Time        `json:"acked_at,omitempty"`
	
	// Compression fields
	Compressed      bool              `json:"compressed,omitempty"`
	CompressionType string            `json:"compression_type,omitempty"`
}

func (m *Message) IsExpired() bool {
	if m.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*m.ExpiresAt)
}

func (m *Message) ShouldRetry() bool {
	return m.Status == StatusFailed && m.RetryCount < m.MaxRetries
}

func (m *Message) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

type Acknowledgment struct {
	MessageID     string    `json:"message_id"`
	ConsumerID    string    `json:"consumer_id"`
	ConsumerGroup string    `json:"consumer_group,omitempty"`
	Success       bool      `json:"success"`
	Reason        string    `json:"reason,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
}