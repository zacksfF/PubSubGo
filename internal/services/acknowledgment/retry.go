package acknowledgment

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/zacksfF/PubSubGo/internal/config"
	"github.com/zacksfF/PubSubGo/internal/core/message"
)

// ExponentialBackoffRetry implements exponential backoff retry strategy
type ExponentialBackoffRetry struct {
	baseDelay    time.Duration
	maxDelay     time.Duration
	maxRetries   int
	multiplier   float64
	jitterFactor float64
}

// NewExponentialBackoffRetry creates a new exponential backoff retry strategy
func NewExponentialBackoffRetry(config *config.BrokerConfig) RetryStrategy {
	return &ExponentialBackoffRetry{
		baseDelay:    config.RetryBackoff,
		maxDelay:     config.RetryBackoff * 60, // Max 60x base delay
		maxRetries:   config.MaxRetries,
		multiplier:   2.0,
		jitterFactor: 0.1,
	}
}

// ShouldRetry determines if a message should be retried
func (r *ExponentialBackoffRetry) ShouldRetry(msg *message.Message, attempt int) bool {
	maxRetries := r.GetMaxRetries(msg)
	return attempt <= maxRetries
}

// GetRetryDelay calculates the delay before the next retry attempt
func (r *ExponentialBackoffRetry) GetRetryDelay(msg *message.Message, attempt int) time.Duration {
	if attempt <= 0 {
		return r.baseDelay
	}

	// Calculate exponential backoff: base * multiplier^(attempt-1)
	delay := float64(r.baseDelay) * math.Pow(r.multiplier, float64(attempt-1))

	// Apply jitter to avoid thundering herd
	jitter := delay * r.jitterFactor * (2*float64(time.Now().UnixNano()%1000)/1000.0 - 1)
	delay += jitter

	// Cap at max delay
	if delay > float64(r.maxDelay) {
		delay = float64(r.maxDelay)
	}

	return time.Duration(delay)
}

// GetMaxRetries returns the maximum number of retries for a message
func (r *ExponentialBackoffRetry) GetMaxRetries(msg *message.Message) int {
	// Use message-specific max retries if set, otherwise use default
	if msg.MaxRetries > 0 {
		return msg.MaxRetries
	}
	return r.maxRetries
}

// LinearBackoffRetry implements linear backoff retry strategy
type LinearBackoffRetry struct {
	baseDelay  time.Duration
	increment  time.Duration
	maxDelay   time.Duration
	maxRetries int
}

// NewLinearBackoffRetry creates a new linear backoff retry strategy
func NewLinearBackoffRetry(config *config.BrokerConfig) RetryStrategy {
	return &LinearBackoffRetry{
		baseDelay:  config.RetryBackoff,
		increment:  config.RetryBackoff,
		maxDelay:   config.RetryBackoff * 10, // Max 10x base delay
		maxRetries: config.MaxRetries,
	}
}

// ShouldRetry determines if a message should be retried
func (r *LinearBackoffRetry) ShouldRetry(msg *message.Message, attempt int) bool {
	maxRetries := r.GetMaxRetries(msg)
	return attempt <= maxRetries
}

// GetRetryDelay calculates the delay before the next retry attempt
func (r *LinearBackoffRetry) GetRetryDelay(msg *message.Message, attempt int) time.Duration {
	delay := r.baseDelay + time.Duration(attempt-1)*r.increment
	if delay > r.maxDelay {
		delay = r.maxDelay
	}
	return delay
}

// GetMaxRetries returns the maximum number of retries for a message
func (r *LinearBackoffRetry) GetMaxRetries(msg *message.Message) int {
	if msg.MaxRetries > 0 {
		return msg.MaxRetries
	}
	return r.maxRetries
}

// FixedDelayRetry implements fixed delay retry strategy
type FixedDelayRetry struct {
	delay      time.Duration
	maxRetries int
}

// NewFixedDelayRetry creates a new fixed delay retry strategy
func NewFixedDelayRetry(config *config.BrokerConfig) RetryStrategy {
	return &FixedDelayRetry{
		delay:      config.RetryBackoff,
		maxRetries: config.MaxRetries,
	}
}

// ShouldRetry determines if a message should be retried
func (r *FixedDelayRetry) ShouldRetry(msg *message.Message, attempt int) bool {
	maxRetries := r.GetMaxRetries(msg)
	return attempt <= maxRetries
}

// GetRetryDelay returns the fixed delay for retry
func (r *FixedDelayRetry) GetRetryDelay(msg *message.Message, attempt int) time.Duration {
	return r.delay
}

// GetMaxRetries returns the maximum number of retries for a message
func (r *FixedDelayRetry) GetMaxRetries(msg *message.Message) int {
	if msg.MaxRetries > 0 {
		return msg.MaxRetries
	}
	return r.maxRetries
}

// Retry processing methods for the service

// RequeueMessage requeues a message for retry
func (s *service) RequeueMessage(ctx context.Context, req *RequeueRequest) error {
	// Get the message
	msg, err := s.messageRepo.Get(ctx, req.MessageID)
	if err != nil {
		return fmt.Errorf("failed to get message for requeue: %w", err)
	}

	// Update retry information
	msg.RetryCount++
	msg.Status = message.StatusPending
	msg.ConsumerID = "" // Clear consumer assignment
	msg.DeliveredAt = nil
	msg.AckedAt = nil

	// Set retry delay
	retryDelay := req.RetryDelay
	if retryDelay == 0 {
		retryDelay = s.retryStrategy.GetRetryDelay(msg, msg.RetryCount)
	}

	// Calculate next retry time
	nextRetryAt := time.Now().Add(retryDelay)

	// Add retry metadata
	if msg.Headers == nil {
		msg.Headers = make(map[string]string)
	}
	msg.Headers["retry_count"] = fmt.Sprintf("%d", msg.RetryCount)
	msg.Headers["retry_reason"] = req.Reason
	msg.Headers["next_retry_at"] = nextRetryAt.Format(time.RFC3339)

	// Store updated message
	if err := s.messageRepo.Store(ctx, msg); err != nil {
		return fmt.Errorf("failed to store requeued message: %w", err)
	}

	s.logger.WithFields(map[string]any{
		"message_id":    req.MessageID,
		"retry_count":   msg.RetryCount,
		"retry_delay":   retryDelay,
		"next_retry_at": nextRetryAt,
		"reason":        req.Reason,
	}).Info("Message requeued for retry")

	return nil
}

// requeueMessageForRetry is an internal helper method
func (s *service) requeueMessageForRetry(ctx context.Context, msg *message.Message, reason string, customDelay time.Duration) (*AcknowledgmentResponse, error) {
	// Update retry information
	msg.RetryCount++
	msg.Status = message.StatusPending
	msg.ConsumerID = "" // Clear consumer assignment
	msg.DeliveredAt = nil
	msg.AckedAt = nil

	// Calculate retry delay
	retryDelay := customDelay
	if retryDelay == 0 {
		retryDelay = s.retryStrategy.GetRetryDelay(msg, msg.RetryCount)
	}

	// Calculate next retry time
	nextRetryAt := time.Now().Add(retryDelay)

	// Add retry metadata
	if msg.Headers == nil {
		msg.Headers = make(map[string]string)
	}
	msg.Headers["retry_count"] = fmt.Sprintf("%d", msg.RetryCount)
	msg.Headers["retry_reason"] = reason
	msg.Headers["next_retry_at"] = nextRetryAt.Format(time.RFC3339)

	// Store updated message
	if err := s.messageRepo.Store(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to store requeued message: %w", err)
	}

	// Update statistics
	s.updateStats("requeued", 0)

	s.logger.WithFields(map[string]any{
		"message_id":    msg.ID,
		"retry_count":   msg.RetryCount,
		"retry_delay":   retryDelay,
		"next_retry_at": nextRetryAt,
		"reason":        reason,
	}).Info("Message requeued for retry")

	return &AcknowledgmentResponse{
		MessageID:   msg.ID,
		Status:      "requeued",
		RetryCount:  msg.RetryCount,
		NextRetryAt: &nextRetryAt,
		ProcessedAt: time.Now(),
	}, nil
}

// markMessageFailed marks a message as permanently failed
func (s *service) markMessageFailed(ctx context.Context, msg *message.Message, reason string) (*AcknowledgmentResponse, error) {
	now := time.Now()

	// Update message status
	msg.Status = message.StatusFailed
	msg.ConsumerID = "" // Clear consumer assignment

	// Add failure metadata
	if msg.Headers == nil {
		msg.Headers = make(map[string]string)
	}
	msg.Headers["failure_reason"] = reason
	msg.Headers["failed_at"] = now.Format(time.RFC3339)
	msg.Headers["final_retry_count"] = fmt.Sprintf("%d", msg.RetryCount)

	// Store updated message
	if err := s.messageRepo.Store(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to store failed message: %w", err)
	}

	// Create acknowledgment record
	ack := &message.Acknowledgment{
		MessageID:  msg.ID,
		ConsumerID: msg.ConsumerID,
		Success:    false,
		Reason:     fmt.Sprintf("permanently failed: %s", reason),
		Timestamp:  now,
	}

	if err := s.messageRepo.Acknowledge(ctx, ack); err != nil {
		s.logger.WithError(err).Warn("Failed to store failure acknowledgment record")
	}

	s.logger.WithFields(map[string]any{
		"message_id":  msg.ID,
		"topic":       msg.Topic,
		"reason":      reason,
		"retry_count": msg.RetryCount,
	}).Warn("Message marked as permanently failed")

	return &AcknowledgmentResponse{
		MessageID:   msg.ID,
		Status:      "failed",
		RetryCount:  msg.RetryCount,
		ProcessedAt: now,
	}, nil
}
