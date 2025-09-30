package subscription

import (
	"sync"
	"time"
)

type Type int

const (
	TypePull Type = iota
	TypePush
)

type Subscription struct {
	mu              sync.RWMutex
	ID              string            `json:"id"`
	Topic           string            `json:"topic"`
	ConsumerGroup   string            `json:"consumer_group"`
	Type            Type              `json:"type"`
	AckDeadline     time.Duration     `json:"ack_deadline"`
	MaxRetries      int               `json:"max_retries"`
	DeadLetterTopic string            `json:"dead_letter_topic,omitempty"`
	Filter          string            `json:"filter,omitempty"`
	Config          map[string]string `json:"config"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	
	// State
	Active          bool              `json:"active"`
	LastMessageID   string            `json:"last_message_id,omitempty"`
	LastAckTime     *time.Time        `json:"last_ack_time,omitempty"`
	
	// Statistics
	MessagesDelivered int64           `json:"messages_delivered"`
	MessagesAcked     int64           `json:"messages_acked"`
	MessagesNacked    int64           `json:"messages_nacked"`
	MessagesDLQ       int64           `json:"messages_dlq"`
}

func NewSubscription(id, topic, consumerGroup string, subType Type) *Subscription {
	return &Subscription{
		ID:            id,
		Topic:         topic,
		ConsumerGroup: consumerGroup,
		Type:          subType,
		AckDeadline:   30 * time.Second,
		MaxRetries:    3,
		Config:        make(map[string]string),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Active:        true,
	}
}

func (s *Subscription) IncrementDelivered() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MessagesDelivered++
	s.UpdatedAt = time.Now()
}

func (s *Subscription) IncrementAcked() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MessagesAcked++
	now := time.Now()
	s.LastAckTime = &now
	s.UpdatedAt = now
}

func (s *Subscription) IncrementNacked() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MessagesNacked++
	s.UpdatedAt = time.Now()
}

func (s *Subscription) IncrementDLQ() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MessagesDLQ++
	s.UpdatedAt = time.Now()
}

func (s *Subscription) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Active
}

func (s *Subscription) Deactivate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Active = false
	s.UpdatedAt = time.Now()
}