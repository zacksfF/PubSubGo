package acknowledgment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zacksfF/PubSubGo/internal/config"
	"github.com/zacksfF/PubSubGo/internal/core/message"
)

// DefaultDLQStrategy implements the default dead letter queue strategy
type DefaultDLQStrategy struct {
	dlqPrefix       string
	enableDLQ       bool
	dlqRetryLimit   int
	dlqTTL          time.Duration
	preserveHeaders bool
}

// NewDefaultDLQStrategy creates a new default DLQ strategy
func NewDefaultDLQStrategy(config *config.BrokerConfig) DLQStrategy {
	return &DefaultDLQStrategy{
		dlqPrefix:       config.DLQPrefix,
		enableDLQ:       config.EnableDLQ,
		dlqRetryLimit:   config.MaxRetries,
		dlqTTL:          7 * 24 * time.Hour, // Default 7 days TTL for DLQ messages
		preserveHeaders: true,
	}
}

// ShouldMoveToDLQ determines if a message should be moved to DLQ
func (d *DefaultDLQStrategy) ShouldMoveToDLQ(msg *message.Message) bool {
	if !d.enableDLQ {
		return false
	}

	// Don't move DLQ messages to DLQ again (avoid infinite loops)
	if strings.HasPrefix(msg.Topic, d.dlqPrefix) {
		return false
	}

	// Move to DLQ if retry limit exceeded
	return msg.RetryCount >= d.dlqRetryLimit
}

// GetDLQTopic returns the DLQ topic name for an original topic
func (d *DefaultDLQStrategy) GetDLQTopic(originalTopic string) string {
	return d.dlqPrefix + originalTopic
}

// EnrichDLQMessage enriches a message before moving to DLQ
func (d *DefaultDLQStrategy) EnrichDLQMessage(msg *message.Message, reason string) *message.Message {
	now := time.Now()
	
	// Create a new message for DLQ (don't modify original)
	dlqMsg := &message.Message{
		ID:              uuid.New().String(), // New ID for DLQ message
		Topic:           d.GetDLQTopic(msg.Topic),
		Partition:       msg.Partition,
		Key:             msg.Key,
		Payload:         msg.Payload,
		Priority:        message.PriorityHigh, // High priority for DLQ messages
		DeliveryMode:    message.DeliveryAtMostOnce, // No retries for DLQ
		Status:          message.StatusDLQ,
		RetryCount:      0, // Reset retry count
		MaxRetries:      0, // No more retries
		CreatedAt:       now,
		Headers:         make(map[string]string),
	}

	// Set TTL for DLQ message
	expiryTime := now.Add(d.dlqTTL)
	dlqMsg.ExpiresAt = &expiryTime

	// Preserve original headers if configured
	if d.preserveHeaders && msg.Headers != nil {
		for k, v := range msg.Headers {
			dlqMsg.Headers[k] = v
		}
	}

	// Add DLQ-specific metadata
	dlqMsg.Headers["dlq_reason"] = reason
	dlqMsg.Headers["dlq_timestamp"] = now.Format(time.RFC3339)
	dlqMsg.Headers["original_topic"] = msg.Topic
	dlqMsg.Headers["original_message_id"] = msg.ID
	dlqMsg.Headers["original_retry_count"] = fmt.Sprintf("%d", msg.RetryCount)
	dlqMsg.Headers["original_created_at"] = msg.CreatedAt.Format(time.RFC3339)
	
	if msg.ConsumerID != "" {
		dlqMsg.Headers["original_consumer_id"] = msg.ConsumerID
	}
	if msg.ConsumerGroup != "" {
		dlqMsg.Headers["original_consumer_group"] = msg.ConsumerGroup
	}
	if msg.DeliveredAt != nil {
		dlqMsg.Headers["original_delivered_at"] = msg.DeliveredAt.Format(time.RFC3339)
	}

	return dlqMsg
}

// TopicBasedDLQStrategy implements topic-specific DLQ strategies
type TopicBasedDLQStrategy struct {
	defaultStrategy DLQStrategy
	topicStrategies map[string]DLQStrategy
}

// NewTopicBasedDLQStrategy creates a new topic-based DLQ strategy
func NewTopicBasedDLQStrategy(defaultStrategy DLQStrategy) *TopicBasedDLQStrategy {
	return &TopicBasedDLQStrategy{
		defaultStrategy: defaultStrategy,
		topicStrategies: make(map[string]DLQStrategy),
	}
}

// AddTopicStrategy adds a specific strategy for a topic
func (t *TopicBasedDLQStrategy) AddTopicStrategy(topic string, strategy DLQStrategy) {
	t.topicStrategies[topic] = strategy
}

// ShouldMoveToDLQ determines if a message should be moved to DLQ based on topic
func (t *TopicBasedDLQStrategy) ShouldMoveToDLQ(msg *message.Message) bool {
	if strategy, exists := t.topicStrategies[msg.Topic]; exists {
		return strategy.ShouldMoveToDLQ(msg)
	}
	return t.defaultStrategy.ShouldMoveToDLQ(msg)
}

// GetDLQTopic returns the DLQ topic name based on topic-specific strategy
func (t *TopicBasedDLQStrategy) GetDLQTopic(originalTopic string) string {
	if strategy, exists := t.topicStrategies[originalTopic]; exists {
		return strategy.GetDLQTopic(originalTopic)
	}
	return t.defaultStrategy.GetDLQTopic(originalTopic)
}

// EnrichDLQMessage enriches a message based on topic-specific strategy
func (t *TopicBasedDLQStrategy) EnrichDLQMessage(msg *message.Message, reason string) *message.Message {
	if strategy, exists := t.topicStrategies[msg.Topic]; exists {
		return strategy.EnrichDLQMessage(msg, reason)
	}
	return t.defaultStrategy.EnrichDLQMessage(msg, reason)
}

// DLQ processing methods for the service

// MoveToDLQ moves a message to dead letter queue
func (s *service) MoveToDLQ(ctx context.Context, req *DLQRequest) error {
	// Get the message
	msg, err := s.messageRepo.Get(ctx, req.MessageID)
	if err != nil {
		return fmt.Errorf("failed to get message for DLQ: %w", err)
	}

	// Use provided DLQ topic or generate one
	dlqTopic := req.DLQTopic
	if dlqTopic == "" {
		dlqTopic = s.dlqStrategy.GetDLQTopic(msg.Topic)
	}

	// Create DLQ message
	dlqMsg := s.dlqStrategy.EnrichDLQMessage(msg, req.Reason)
	dlqMsg.Topic = dlqTopic

	// Store DLQ message
	if err := s.messageRepo.Store(ctx, dlqMsg); err != nil {
		return fmt.Errorf("failed to store DLQ message: %w", err)
	}

	// Update original message status
	msg.Status = message.StatusDLQ
	msg.ConsumerID = "" // Clear consumer assignment
	
	// Add DLQ metadata to original message
	if msg.Headers == nil {
		msg.Headers = make(map[string]string)
	}
	msg.Headers["dlq_moved_at"] = time.Now().Format(time.RFC3339)
	msg.Headers["dlq_topic"] = dlqTopic
	msg.Headers["dlq_message_id"] = dlqMsg.ID
	msg.Headers["dlq_reason"] = req.Reason

	// Store updated original message
	if err := s.messageRepo.Store(ctx, msg); err != nil {
		return fmt.Errorf("failed to update original message: %w", err)
	}

	s.logger.WithFields(map[string]any{
		"message_id":      req.MessageID,
		"original_topic":  msg.Topic,
		"dlq_topic":       dlqTopic,
		"dlq_message_id":  dlqMsg.ID,
		"reason":          req.Reason,
		"retry_count":     msg.RetryCount,
	}).Warn("Message moved to dead letter queue")

	return nil
}

// moveMessageToDLQ is an internal helper method
func (s *service) moveMessageToDLQ(ctx context.Context, msg *message.Message, reason string) (*AcknowledgmentResponse, error) {
	// Generate DLQ topic
	dlqTopic := s.dlqStrategy.GetDLQTopic(msg.Topic)

	// Create DLQ message
	dlqMsg := s.dlqStrategy.EnrichDLQMessage(msg, reason)

	// Store DLQ message
	if err := s.messageRepo.Store(ctx, dlqMsg); err != nil {
		return nil, fmt.Errorf("failed to store DLQ message: %w", err)
	}

	// Update original message status
	msg.Status = message.StatusDLQ
	msg.ConsumerID = "" // Clear consumer assignment
	
	// Add DLQ metadata to original message
	if msg.Headers == nil {
		msg.Headers = make(map[string]string)
	}
	msg.Headers["dlq_moved_at"] = time.Now().Format(time.RFC3339)
	msg.Headers["dlq_topic"] = dlqTopic
	msg.Headers["dlq_message_id"] = dlqMsg.ID
	msg.Headers["dlq_reason"] = reason

	// Store updated original message
	if err := s.messageRepo.Store(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to update original message: %w", err)
	}

	// Update statistics
	s.updateStats("dlq", 0)

	s.logger.WithFields(map[string]any{
		"message_id":      msg.ID,
		"original_topic":  msg.Topic,
		"dlq_topic":       dlqTopic,
		"dlq_message_id":  dlqMsg.ID,
		"reason":          reason,
		"retry_count":     msg.RetryCount,
	}).Warn("Message moved to dead letter queue")

	return &AcknowledgmentResponse{
		MessageID:   msg.ID,
		Status:      "dlq",
		RetryCount:  msg.RetryCount,
		DLQTopic:    dlqTopic,
		ProcessedAt: time.Now(),
	}, nil
}

// DLQ utility methods

// GetDLQMessages retrieves messages from a DLQ topic
func (s *service) GetDLQMessages(ctx context.Context, dlqTopic string, limit int, offset int64) ([]*message.Message, error) {
	return s.messageRepo.GetByTopic(ctx, dlqTopic, limit, offset)
}

// ReprocessDLQMessage moves a message from DLQ back to original topic
func (s *service) ReprocessDLQMessage(ctx context.Context, dlqMessageID string) error {
	// Get DLQ message
	dlqMsg, err := s.messageRepo.Get(ctx, dlqMessageID)
	if err != nil {
		return fmt.Errorf("failed to get DLQ message: %w", err)
	}

	// Verify it's a DLQ message
	if dlqMsg.Status != message.StatusDLQ {
		return fmt.Errorf("message %s is not a DLQ message", dlqMessageID)
	}

	// Get original topic from headers
	originalTopic, exists := dlqMsg.Headers["original_topic"]
	if !exists {
		return fmt.Errorf("DLQ message missing original topic information")
	}

	// Create new message for reprocessing
	reprocessMsg := &message.Message{
		ID:           uuid.New().String(),
		Topic:        originalTopic,
		Partition:    dlqMsg.Partition,
		Key:          dlqMsg.Key,
		Payload:      dlqMsg.Payload,
		Priority:     message.PriorityNormal,
		DeliveryMode: message.DeliveryAtLeastOnce,
		Status:       message.StatusPending,
		RetryCount:   0, // Reset retry count
		MaxRetries:   s.config.MaxRetries,
		CreatedAt:    time.Now(),
		Headers:      make(map[string]string),
	}

	// Preserve relevant original headers
	for k, v := range dlqMsg.Headers {
		if !strings.HasPrefix(k, "dlq_") && !strings.HasPrefix(k, "original_") {
			reprocessMsg.Headers[k] = v
		}
	}

	// Add reprocessing metadata
	reprocessMsg.Headers["reprocessed_from_dlq"] = "true"
	reprocessMsg.Headers["reprocessed_at"] = time.Now().Format(time.RFC3339)
	reprocessMsg.Headers["original_dlq_message_id"] = dlqMessageID
	if dlqReason, exists := dlqMsg.Headers["dlq_reason"]; exists {
		reprocessMsg.Headers["original_dlq_reason"] = dlqReason
	}

	// Store reprocessed message
	if err := s.messageRepo.Store(ctx, reprocessMsg); err != nil {
		return fmt.Errorf("failed to store reprocessed message: %w", err)
	}

	// Mark DLQ message as reprocessed (don't delete, keep for audit)
	dlqMsg.Headers["reprocessed"] = "true"
	dlqMsg.Headers["reprocessed_at"] = time.Now().Format(time.RFC3339)
	dlqMsg.Headers["reprocessed_message_id"] = reprocessMsg.ID
	
	if err := s.messageRepo.Store(ctx, dlqMsg); err != nil {
		s.logger.WithError(err).Warn("Failed to update DLQ message reprocessing status")
	}

	s.logger.WithFields(map[string]any{
		"dlq_message_id":        dlqMessageID,
		"reprocessed_message_id": reprocessMsg.ID,
		"original_topic":        originalTopic,
	}).Info("DLQ message reprocessed")

	return nil
}