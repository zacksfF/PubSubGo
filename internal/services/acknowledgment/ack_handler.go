package acknowledgment

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/zacksfF/PubSubGo/internal/config"
	"github.com/zacksfF/PubSubGo/internal/core/message"
	"github.com/zacksfF/PubSubGo/internal/ports/repository"
)

// Service defines the acknowledgment service interface
type Service interface {
	// ProcessAcknowledgment processes a message acknowledgment
	ProcessAcknowledgment(ctx context.Context, ack *AcknowledgmentRequest) (*AcknowledgmentResponse, error)
	
	// ProcessNegativeAcknowledgment processes a negative acknowledgment
	ProcessNegativeAcknowledgment(ctx context.Context, nack *NegativeAcknowledgmentRequest) (*AcknowledgmentResponse, error)
	
	// CheckExpiredAcknowledgments checks for messages with expired ack deadlines
	CheckExpiredAcknowledgments(ctx context.Context) (*ExpirationReport, error)
	
	// RequeueMessage requeues a message for retry
	RequeueMessage(ctx context.Context, req *RequeueRequest) error
	
	// MoveToDLQ moves a message to dead letter queue
	MoveToDLQ(ctx context.Context, req *DLQRequest) error
	
	// GetAcknowledgmentStats returns acknowledgment statistics
	GetAcknowledgmentStats(ctx context.Context, filter *StatsFilter) (*AcknowledgmentStats, error)
	
	// StartAcknowledgmentWorker starts background workers for acknowledgment processing
	StartAcknowledgmentWorker(ctx context.Context) error
	
	// StopAcknowledgmentWorker stops background workers
	StopAcknowledgmentWorker() error
}

// service implements the acknowledgment service
type service struct {
	messageRepo repository.MessageRepository
	config      *config.BrokerConfig
	logger      *logrus.Logger
	
	// Pending acknowledgments tracking
	pendingAcks map[string]*PendingAcknowledgment
	acksMutex   sync.RWMutex
	
	// Background workers
	workers      map[string]context.CancelFunc
	workersMutex sync.RWMutex
	
	// Retry configuration
	retryStrategy RetryStrategy
	dlqStrategy   DLQStrategy
	
	// Statistics
	stats *ServiceStats
}

// Request/Response types

type AcknowledgmentRequest struct {
	MessageID     string    `json:"message_id"`
	ConsumerID    string    `json:"consumer_id"`
	ConsumerGroup string    `json:"consumer_group,omitempty"`
	Success       bool      `json:"success"`
	ProcessingTime time.Duration `json:"processing_time,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type NegativeAcknowledgmentRequest struct {
	MessageID     string    `json:"message_id"`
	ConsumerID    string    `json:"consumer_id"`
	ConsumerGroup string    `json:"consumer_group,omitempty"`
	Reason        string    `json:"reason"`
	Retry         bool      `json:"retry"`
	RetryDelay    time.Duration `json:"retry_delay,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type AcknowledgmentResponse struct {
	MessageID    string          `json:"message_id"`
	Status       string          `json:"status"` // acknowledged, requeued, dlq
	RetryCount   int             `json:"retry_count,omitempty"`
	NextRetryAt  *time.Time      `json:"next_retry_at,omitempty"`
	DLQTopic     string          `json:"dlq_topic,omitempty"`
	ProcessedAt  time.Time       `json:"processed_at"`
}

type RequeueRequest struct {
	MessageID   string        `json:"message_id"`
	Reason      string        `json:"reason"`
	RetryDelay  time.Duration `json:"retry_delay,omitempty"`
	MaxRetries  int           `json:"max_retries,omitempty"`
}

type DLQRequest struct {
	MessageID    string `json:"message_id"`
	Reason       string `json:"reason"`
	OriginalTopic string `json:"original_topic"`
	DLQTopic     string `json:"dlq_topic,omitempty"`
}

type ExpirationReport struct {
	ExpiredCount    int                    `json:"expired_count"`
	RequeuedCount   int                    `json:"requeued_count"`
	DLQCount        int                    `json:"dlq_count"`
	ProcessedAt     time.Time              `json:"processed_at"`
	ExpiredMessages []ExpiredMessageInfo   `json:"expired_messages,omitempty"`
}

type ExpiredMessageInfo struct {
	MessageID     string    `json:"message_id"`
	Topic         string    `json:"topic"`
	ConsumerID    string    `json:"consumer_id"`
	DeliveredAt   time.Time `json:"delivered_at"`
	AckDeadline   time.Duration `json:"ack_deadline"`
	Action        string    `json:"action"` // requeued, dlq
}

type StatsFilter struct {
	Topic         string     `json:"topic,omitempty"`
	ConsumerGroup string     `json:"consumer_group,omitempty"`
	TimeFrom      *time.Time `json:"time_from,omitempty"`
	TimeTo        *time.Time `json:"time_to,omitempty"`
}

type AcknowledgmentStats struct {
	TotalAcknowledged   int64     `json:"total_acknowledged"`
	TotalNacked         int64     `json:"total_nacked"`
	TotalRequeued       int64     `json:"total_requeued"`
	TotalDLQ            int64     `json:"total_dlq"`
	TotalExpired        int64     `json:"total_expired"`
	AvgProcessingTime   time.Duration `json:"avg_processing_time"`
	AckRate             float64   `json:"ack_rate"`
	GeneratedAt         time.Time `json:"generated_at"`
	TopicStats          map[string]*TopicAckStats `json:"topic_stats,omitempty"`
}

type TopicAckStats struct {
	Topic             string        `json:"topic"`
	Acknowledged      int64         `json:"acknowledged"`
	Nacked            int64         `json:"nacked"`
	Requeued          int64         `json:"requeued"`
	DLQ               int64         `json:"dlq"`
	AvgProcessingTime time.Duration `json:"avg_processing_time"`
}

// Internal types

type PendingAcknowledgment struct {
	MessageID     string    `json:"message_id"`
	ConsumerID    string    `json:"consumer_id"`
	ConsumerGroup string    `json:"consumer_group"`
	DeliveredAt   time.Time `json:"delivered_at"`
	AckDeadline   time.Time `json:"ack_deadline"`
	RetryCount    int       `json:"retry_count"`
	Topic         string    `json:"topic"`
}

type ServiceStats struct {
	sync.RWMutex
	Acknowledged     int64
	Nacked           int64
	Requeued         int64
	DLQ              int64
	Expired          int64
	ProcessingTimes  []time.Duration
	LastReset        time.Time
}

// Retry strategy interface
type RetryStrategy interface {
	ShouldRetry(msg *message.Message, attempt int) bool
	GetRetryDelay(msg *message.Message, attempt int) time.Duration
	GetMaxRetries(msg *message.Message) int
}

// DLQ strategy interface
type DLQStrategy interface {
	ShouldMoveToDLQ(msg *message.Message) bool
	GetDLQTopic(originalTopic string) string
	EnrichDLQMessage(msg *message.Message, reason string) *message.Message
}

// NewService creates a new acknowledgment service
func NewService(
	messageRepo repository.MessageRepository,
	config *config.BrokerConfig,
	logger *logrus.Logger,
) Service {
	return &service{
		messageRepo:   messageRepo,
		config:        config,
		logger:        logger,
		pendingAcks:   make(map[string]*PendingAcknowledgment),
		workers:       make(map[string]context.CancelFunc),
		retryStrategy: NewExponentialBackoffRetry(config),
		dlqStrategy:   NewDefaultDLQStrategy(config),
		stats: &ServiceStats{
			LastReset: time.Now(),
		},
	}
}

// ProcessAcknowledgment processes a message acknowledgment
func (s *service) ProcessAcknowledgment(ctx context.Context, req *AcknowledgmentRequest) (*AcknowledgmentResponse, error) {
	// Validate request
	if err := s.validateAckRequest(req); err != nil {
		return nil, fmt.Errorf("invalid acknowledgment request: %w", err)
	}

	// Get the message
	msg, err := s.messageRepo.Get(ctx, req.MessageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	// Verify the consumer has the right to acknowledge this message
	if msg.ConsumerID != req.ConsumerID {
		return nil, fmt.Errorf("consumer %s cannot acknowledge message assigned to %s", 
			req.ConsumerID, msg.ConsumerID)
	}

	now := time.Now()
	
	if req.Success {
		// Successful acknowledgment
		msg.Status = message.StatusAcknowledged
		msg.AckedAt = &now
		
		// Remove from pending acknowledgments
		s.removePendingAck(req.MessageID)
		
		// Update statistics
		s.updateStats("acknowledged", req.ProcessingTime)
		
		s.logger.WithFields(logrus.Fields{
			"message_id":    req.MessageID,
			"consumer_id":   req.ConsumerID,
			"topic":         msg.Topic,
			"processing_time": req.ProcessingTime,
		}).Info("Message acknowledged successfully")
		
		// Store updated message
		if err := s.messageRepo.Store(ctx, msg); err != nil {
			return nil, fmt.Errorf("failed to store acknowledged message: %w", err)
		}
		
		// Create acknowledgment record
		ack := &message.Acknowledgment{
			MessageID:     req.MessageID,
			ConsumerID:    req.ConsumerID,
			ConsumerGroup: req.ConsumerGroup,
			Success:       true,
			Timestamp:     now,
		}
		
		if err := s.messageRepo.Acknowledge(ctx, ack); err != nil {
			s.logger.WithError(err).Warn("Failed to store acknowledgment record")
		}
		
		return &AcknowledgmentResponse{
			MessageID:   req.MessageID,
			Status:      "acknowledged",
			ProcessedAt: now,
		}, nil
	} else {
		// Failed acknowledgment - treat as NACK
		return s.processFailedAcknowledgment(ctx, msg, req)
	}
}

// ProcessNegativeAcknowledgment processes a negative acknowledgment
func (s *service) ProcessNegativeAcknowledgment(ctx context.Context, req *NegativeAcknowledgmentRequest) (*AcknowledgmentResponse, error) {
	// Validate request
	if err := s.validateNackRequest(req); err != nil {
		return nil, fmt.Errorf("invalid negative acknowledgment request: %w", err)
	}

	// Get the message
	msg, err := s.messageRepo.Get(ctx, req.MessageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	// Verify the consumer has the right to nack this message
	if msg.ConsumerID != req.ConsumerID {
		return nil, fmt.Errorf("consumer %s cannot nack message assigned to %s", 
			req.ConsumerID, msg.ConsumerID)
	}

	s.logger.WithFields(logrus.Fields{
		"message_id":  req.MessageID,
		"consumer_id": req.ConsumerID,
		"reason":      req.Reason,
		"retry":       req.Retry,
	}).Info("Message negative acknowledged")

	// Remove from pending acknowledgments
	s.removePendingAck(req.MessageID)

	if req.Retry && s.retryStrategy.ShouldRetry(msg, msg.RetryCount+1) {
		// Requeue for retry
		return s.requeueMessageForRetry(ctx, msg, req.Reason, req.RetryDelay)
	} else {
		// Move to DLQ or mark as failed
		if s.dlqStrategy.ShouldMoveToDLQ(msg) {
			return s.moveMessageToDLQ(ctx, msg, req.Reason)
		} else {
			// Mark as permanently failed
			return s.markMessageFailed(ctx, msg, req.Reason)
		}
	}
}

// CheckExpiredAcknowledgments checks for messages with expired ack deadlines
func (s *service) CheckExpiredAcknowledgments(ctx context.Context) (*ExpirationReport, error) {
	now := time.Now()
	report := &ExpirationReport{
		ProcessedAt:     now,
		ExpiredMessages: make([]ExpiredMessageInfo, 0),
	}

	// Get all topics to check for unacknowledged messages
	deadline := s.config.DefaultAckDeadline
	unackedMessages, err := s.messageRepo.GetUnacknowledged(ctx, "", deadline)
	if err != nil {
		return nil, fmt.Errorf("failed to get unacknowledged messages: %w", err)
	}

	for _, msg := range unackedMessages {
		if msg.DeliveredAt == nil {
			continue
		}

		// Check if message has expired ack deadline
		ackDeadline := msg.DeliveredAt.Add(deadline)
		if now.After(ackDeadline) {
			expiredInfo := ExpiredMessageInfo{
				MessageID:   msg.ID,
				Topic:       msg.Topic,
				ConsumerID:  msg.ConsumerID,
				DeliveredAt: *msg.DeliveredAt,
				AckDeadline: deadline,
			}

			// Decide action: retry or DLQ
			if s.retryStrategy.ShouldRetry(msg, msg.RetryCount+1) {
				// Requeue for retry
				_, err := s.requeueMessageForRetry(ctx, msg, "ack timeout", 0)
				if err != nil {
					s.logger.WithError(err).Warn("Failed to requeue expired message")
					continue
				}
				expiredInfo.Action = "requeued"
				report.RequeuedCount++
			} else if s.dlqStrategy.ShouldMoveToDLQ(msg) {
				// Move to DLQ
				_, err := s.moveMessageToDLQ(ctx, msg, "ack timeout")
				if err != nil {
					s.logger.WithError(err).Warn("Failed to move expired message to DLQ")
					continue
				}
				expiredInfo.Action = "dlq"
				report.DLQCount++
			} else {
				// Mark as failed
				_, err := s.markMessageFailed(ctx, msg, "ack timeout")
				if err != nil {
					s.logger.WithError(err).Warn("Failed to mark expired message as failed")
					continue
				}
				expiredInfo.Action = "failed"
			}

			report.ExpiredMessages = append(report.ExpiredMessages, expiredInfo)
			report.ExpiredCount++

			// Remove from pending acknowledgments
			s.removePendingAck(msg.ID)

			// Update statistics
			s.updateStats("expired", 0)
		}
	}

	s.logger.WithFields(logrus.Fields{
		"expired_count":  report.ExpiredCount,
		"requeued_count": report.RequeuedCount,
		"dlq_count":      report.DLQCount,
	}).Info("Processed expired acknowledgments")

	return report, nil
}

// Helper methods

func (s *service) validateAckRequest(req *AcknowledgmentRequest) error {
	if req.MessageID == "" {
		return fmt.Errorf("message ID cannot be empty")
	}
	if req.ConsumerID == "" {
		return fmt.Errorf("consumer ID cannot be empty")
	}
	return nil
}

func (s *service) validateNackRequest(req *NegativeAcknowledgmentRequest) error {
	if req.MessageID == "" {
		return fmt.Errorf("message ID cannot be empty")
	}
	if req.ConsumerID == "" {
		return fmt.Errorf("consumer ID cannot be empty")
	}
	return nil
}

func (s *service) processFailedAcknowledgment(ctx context.Context, msg *message.Message, req *AcknowledgmentRequest) (*AcknowledgmentResponse, error) {
	// Remove from pending acknowledgments
	s.removePendingAck(req.MessageID)

	reason := "processing failed"
	if metadata, ok := req.Metadata["reason"]; ok {
		reason = metadata
	}

	if s.retryStrategy.ShouldRetry(msg, msg.RetryCount+1) {
		// Requeue for retry
		return s.requeueMessageForRetry(ctx, msg, reason, 0)
	} else if s.dlqStrategy.ShouldMoveToDLQ(msg) {
		// Move to DLQ
		return s.moveMessageToDLQ(ctx, msg, reason)
	} else {
		// Mark as permanently failed
		return s.markMessageFailed(ctx, msg, reason)
	}
}

func (s *service) removePendingAck(messageID string) {
	s.acksMutex.Lock()
	defer s.acksMutex.Unlock()
	delete(s.pendingAcks, messageID)
}

func (s *service) updateStats(operation string, processingTime time.Duration) {
	s.stats.Lock()
	defer s.stats.Unlock()

	switch operation {
	case "acknowledged":
		s.stats.Acknowledged++
	case "nacked":
		s.stats.Nacked++
	case "requeued":
		s.stats.Requeued++
	case "dlq":
		s.stats.DLQ++
	case "expired":
		s.stats.Expired++
	}

	if processingTime > 0 {
		s.stats.ProcessingTimes = append(s.stats.ProcessingTimes, processingTime)
		// Keep only last 1000 processing times for average calculation
		if len(s.stats.ProcessingTimes) > 1000 {
			s.stats.ProcessingTimes = s.stats.ProcessingTimes[1:]
		}
	}
}

// GetAcknowledgmentStats returns acknowledgment statistics
func (s *service) GetAcknowledgmentStats(ctx context.Context, filter *StatsFilter) (*AcknowledgmentStats, error) {
	s.stats.RLock()
	defer s.stats.RUnlock()

	stats := &AcknowledgmentStats{
		TotalAcknowledged: s.stats.Acknowledged,
		TotalNacked:       s.stats.Nacked,
		TotalRequeued:     s.stats.Requeued,
		TotalDLQ:          s.stats.DLQ,
		TotalExpired:      s.stats.Expired,
		GeneratedAt:       time.Now(),
		TopicStats:        make(map[string]*TopicAckStats),
	}

	// Calculate average processing time
	if len(s.stats.ProcessingTimes) > 0 {
		total := time.Duration(0)
		for _, pt := range s.stats.ProcessingTimes {
			total += pt
		}
		stats.AvgProcessingTime = total / time.Duration(len(s.stats.ProcessingTimes))
	}

	// Calculate acknowledgment rate
	totalMessages := stats.TotalAcknowledged + stats.TotalNacked
	if totalMessages > 0 {
		stats.AckRate = float64(stats.TotalAcknowledged) / float64(totalMessages)
	}

	// TODO: Implement topic-specific stats based on filter
	// This would require tracking stats per topic

	return stats, nil
}

// StartAcknowledgmentWorker starts background workers for acknowledgment processing
func (s *service) StartAcknowledgmentWorker(ctx context.Context) error {
	s.workersMutex.Lock()
	defer s.workersMutex.Unlock()

	// Start expiration checker worker
	if _, exists := s.workers["expiration_checker"]; !exists {
		workerCtx, cancel := context.WithCancel(ctx)
		s.workers["expiration_checker"] = cancel
		go s.expirationWorker(workerCtx)
	}

	// Start retry processor worker
	if _, exists := s.workers["retry_processor"]; !exists {
		workerCtx, cancel := context.WithCancel(ctx)
		s.workers["retry_processor"] = cancel
		go s.retryWorker(workerCtx)
	}

	// Start pending ack cleaner worker
	if _, exists := s.workers["pending_cleaner"]; !exists {
		workerCtx, cancel := context.WithCancel(ctx)
		s.workers["pending_cleaner"] = cancel
		go s.pendingAckCleanupWorker(workerCtx)
	}

	s.logger.Info("Acknowledgment workers started")
	return nil
}

// StopAcknowledgmentWorker stops background workers
func (s *service) StopAcknowledgmentWorker() error {
	s.workersMutex.Lock()
	defer s.workersMutex.Unlock()

	// Stop all workers
	for name, cancel := range s.workers {
		cancel()
		delete(s.workers, name)
		s.logger.WithField("worker", name).Info("Acknowledgment worker stopped")
	}

	return nil
}

// Background worker methods

// expirationWorker periodically checks for expired acknowledgments
func (s *service) expirationWorker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.CheckExpiredAcknowledgments(ctx); err != nil {
				s.logger.WithError(err).Warn("Failed to check expired acknowledgments")
			}
		}
	}
}

// retryWorker processes retry queues
func (s *service) retryWorker(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second) // Process retries every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Get messages ready for retry
			retryMessages, err := s.messageRepo.GetFailedMessages(ctx, "", s.config.MaxRetries)
			if err != nil {
				s.logger.WithError(err).Warn("Failed to get retry messages")
				continue
			}

			for _, msg := range retryMessages {
				// Check if message is ready for retry based on next_retry_at header
				if nextRetryStr, exists := msg.Headers["next_retry_at"]; exists {
					nextRetryTime, err := time.Parse(time.RFC3339, nextRetryStr)
					if err == nil && time.Now().After(nextRetryTime) {
						// Message is ready for retry
						msg.Status = message.StatusPending
						msg.ConsumerID = ""
						msg.DeliveredAt = nil
						
						if err := s.messageRepo.Store(ctx, msg); err != nil {
							s.logger.WithError(err).Warn("Failed to reset message for retry")
						}
					}
				}
			}
		}
	}
}

// pendingAckCleanupWorker cleans up stale pending acknowledgments
func (s *service) pendingAckCleanupWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanupStalePendingAcks()
		}
	}
}

// cleanupStalePendingAcks removes stale pending acknowledgments
func (s *service) cleanupStalePendingAcks() {
	s.acksMutex.Lock()
	defer s.acksMutex.Unlock()

	now := time.Now()
	staleThreshold := 24 * time.Hour // Remove pending acks older than 24 hours

	for messageID, pendingAck := range s.pendingAcks {
		if now.Sub(pendingAck.DeliveredAt) > staleThreshold {
			delete(s.pendingAcks, messageID)
			s.logger.WithField("message_id", messageID).Debug("Removed stale pending acknowledgment")
		}
	}
}