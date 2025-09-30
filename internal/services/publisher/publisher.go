package publisher

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/zacksfF/PubSubGo/internal/config"
	"github.com/zacksfF/PubSubGo/internal/core/message"
	"github.com/zacksfF/PubSubGo/internal/core/topic"
	"github.com/zacksfF/PubSubGo/internal/ports/repository"
)

// Service defines the publisher service interface
type Service interface {
	// Publish publishes a single message to a topic
	Publish(ctx context.Context, req *PublishRequest) (*PublishResponse, error)
	
	// PublishBatch publishes multiple messages to a topic in a single operation
	PublishBatch(ctx context.Context, req *BatchPublishRequest) (*BatchPublishResponse, error)
	
	// CreateTopic creates a new topic with specified configuration
	CreateTopic(ctx context.Context, req *CreateTopicRequest) error
	
	// DeleteTopic deletes a topic and all its messages
	DeleteTopic(ctx context.Context, topicName string) error
	
	// GetTopicStats returns statistics for a topic
	GetTopicStats(ctx context.Context, topicName string) (*TopicStats, error)
	
	// ListTopics returns all available topics
	ListTopics(ctx context.Context) ([]*TopicInfo, error)
}

// service implements the publisher service
type service struct {
	messageRepo repository.MessageRepository
	topicRepo   repository.TopicRepository
	config      *config.BrokerConfig
	logger      *logrus.Logger
	compressor  Compressor
}

// PublishRequest represents a single message publish request
type PublishRequest struct {
	Topic         string            `json:"topic"`
	Key           []byte            `json:"key,omitempty"`
	Payload       []byte            `json:"payload"`
	Headers       map[string]string `json:"headers,omitempty"`
	Priority      message.Priority  `json:"priority,omitempty"`
	DeliveryMode  message.DeliveryMode `json:"delivery_mode,omitempty"`
	TTL           *time.Duration    `json:"ttl,omitempty"`
	Partition     *int32            `json:"partition,omitempty"`
}

// PublishResponse contains the result of a publish operation
type PublishResponse struct {
	MessageID   string    `json:"message_id"`
	Topic       string    `json:"topic"`
	Partition   int32     `json:"partition"`
	Offset      int64     `json:"offset"`
	Timestamp   time.Time `json:"timestamp"`
	Compressed  bool      `json:"compressed,omitempty"`
}

// BatchPublishRequest represents a batch publish request
type BatchPublishRequest struct {
	Topic    string           `json:"topic"`
	Messages []*MessageBatch  `json:"messages"`
}

// MessageBatch represents a message in a batch
type MessageBatch struct {
	Key          []byte            `json:"key,omitempty"`
	Payload      []byte            `json:"payload"`
	Headers      map[string]string `json:"headers,omitempty"`
	Priority     message.Priority  `json:"priority,omitempty"`
	DeliveryMode message.DeliveryMode `json:"delivery_mode,omitempty"`
	TTL          *time.Duration    `json:"ttl,omitempty"`
}

// BatchPublishResponse contains the result of a batch publish operation
type BatchPublishResponse struct {
	Published   int                 `json:"published"`
	Failed      int                 `json:"failed"`
	MessageIDs  []string            `json:"message_ids"`
	Responses   []*PublishResponse  `json:"responses,omitempty"`
	Errors      []string            `json:"errors,omitempty"`
}

// CreateTopicRequest represents a topic creation request
type CreateTopicRequest struct {
	Name          string                 `json:"name"`
	Partitions    int32                  `json:"partitions,omitempty"`
	Replication   int32                  `json:"replication,omitempty"`
	RetentionTime time.Duration          `json:"retention_time,omitempty"`
	MaxSize       int64                  `json:"max_size,omitempty"`
	Config        map[string]interface{} `json:"config,omitempty"`
}

// TopicStats represents topic statistics
type TopicStats struct {
	Topic        string    `json:"topic"`
	Partitions   int32     `json:"partitions"`
	Messages     int64     `json:"messages"`
	Consumers    int64     `json:"consumers"`
	Rate         string    `json:"rate"`
	Size         string    `json:"size"`
	BytesIn      int64     `json:"bytes_in"`
	BytesOut     int64     `json:"bytes_out"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TopicInfo represents basic topic information
type TopicInfo struct {
	Name          string                 `json:"name"`
	Partitions    int32                  `json:"partitions"`
	MessageCount  int64                  `json:"message_count"`
	RetentionTime time.Duration          `json:"retention_time"`
	CreatedAt     time.Time              `json:"created_at"`
	Config        map[string]interface{} `json:"config,omitempty"`
}

// NewService creates a new publisher service
func NewService(
	messageRepo repository.MessageRepository,
	topicRepo repository.TopicRepository,
	config *config.BrokerConfig,
	logger *logrus.Logger,
) Service {
	return &service{
		messageRepo: messageRepo,
		topicRepo:   topicRepo,
		config:      config,
		logger:      logger,
		compressor:  NewCompressor(config.CompressionType, config.CompressionLevel),
	}
}

// Publish publishes a single message to a topic
func (s *service) Publish(ctx context.Context, req *PublishRequest) (*PublishResponse, error) {
	// Validate request
	if err := s.validatePublishRequest(req); err != nil {
		return nil, fmt.Errorf("invalid publish request: %w", err)
	}

	// Ensure topic exists
	topicEntity, err := s.ensureTopicExists(ctx, req.Topic)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure topic exists: %w", err)
	}

	// Create message
	msg := s.createMessage(req, topicEntity)

	// Compress payload if enabled
	if s.shouldCompress(req.Payload) {
		compressed, err := s.compressor.Compress(req.Payload)
		if err != nil {
			s.logger.WithError(err).Warn("Failed to compress message, using original payload")
		} else {
			msg.Payload = compressed
			msg.Compressed = true
			msg.CompressionType = s.config.CompressionType
		}
	}

	// Store message
	if err := s.messageRepo.Store(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to store message: %w", err)
	}

	// Update topic statistics
	topicEntity.IncrementMessages(1)
	topicEntity.AddBytesIn(int64(len(req.Payload)))
	if err := s.topicRepo.Update(ctx, topicEntity); err != nil {
		s.logger.WithError(err).Warn("Failed to update topic statistics")
	}

	s.logger.WithFields(logrus.Fields{
		"message_id": msg.ID,
		"topic":      msg.Topic,
		"partition":  msg.Partition,
		"size":       len(req.Payload),
		"compressed": msg.Compressed,
	}).Info("Message published successfully")

	return &PublishResponse{
		MessageID:  msg.ID,
		Topic:      msg.Topic,
		Partition:  msg.Partition,
		Offset:     msg.Offset,
		Timestamp:  msg.CreatedAt,
		Compressed: msg.Compressed,
	}, nil
}

// PublishBatch publishes multiple messages in a single operation
func (s *service) PublishBatch(ctx context.Context, req *BatchPublishRequest) (*BatchPublishResponse, error) {
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("batch cannot be empty")
	}

	if len(req.Messages) > s.config.MaxMessageBatchSize {
		return nil, fmt.Errorf("batch size %d exceeds maximum %d", 
			len(req.Messages), s.config.MaxMessageBatchSize)
	}

	// Ensure topic exists
	topicEntity, err := s.ensureTopicExists(ctx, req.Topic)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure topic exists: %w", err)
	}

	// Create messages
	messages := make([]*message.Message, 0, len(req.Messages))
	responses := make([]*PublishResponse, 0, len(req.Messages))
	errors := make([]string, 0)
	published := 0
	failed := 0
	totalBytesIn := int64(0)

	for i, msgReq := range req.Messages {
		// Validate each message
		if err := s.validateBatchMessage(msgReq); err != nil {
			errors = append(errors, fmt.Sprintf("message %d: %v", i, err))
			failed++
			continue
		}

		// Create message from batch request
		publishReq := &PublishRequest{
			Topic:        req.Topic,
			Key:          msgReq.Key,
			Payload:      msgReq.Payload,
			Headers:      msgReq.Headers,
			Priority:     msgReq.Priority,
			DeliveryMode: msgReq.DeliveryMode,
			TTL:          msgReq.TTL,
		}

		msg := s.createMessage(publishReq, topicEntity)

		// Compress if needed
		if s.shouldCompress(msgReq.Payload) {
			compressed, err := s.compressor.Compress(msgReq.Payload)
			if err != nil {
				s.logger.WithError(err).Warn("Failed to compress message in batch")
			} else {
				msg.Payload = compressed
				msg.Compressed = true
				msg.CompressionType = s.config.CompressionType
			}
		}

		messages = append(messages, msg)
		responses = append(responses, &PublishResponse{
			MessageID:  msg.ID,
			Topic:      msg.Topic,
			Partition:  msg.Partition,
			Offset:     msg.Offset,
			Timestamp:  msg.CreatedAt,
			Compressed: msg.Compressed,
		})

		totalBytesIn += int64(len(msgReq.Payload))
		published++
	}

	// Store messages in batch
	if len(messages) > 0 {
		if err := s.messageRepo.StoreBatch(ctx, messages); err != nil {
			return nil, fmt.Errorf("failed to store message batch: %w", err)
		}

		// Update topic statistics
		topicEntity.IncrementMessages(int64(len(messages)))
		topicEntity.AddBytesIn(totalBytesIn)
		if err := s.topicRepo.Update(ctx, topicEntity); err != nil {
			s.logger.WithError(err).Warn("Failed to update topic statistics for batch")
		}
	}

	messageIDs := make([]string, len(responses))
	for i, resp := range responses {
		messageIDs[i] = resp.MessageID
	}

	s.logger.WithFields(logrus.Fields{
		"topic":     req.Topic,
		"published": published,
		"failed":    failed,
		"total":     len(req.Messages),
	}).Info("Batch published")

	return &BatchPublishResponse{
		Published:  published,
		Failed:     failed,
		MessageIDs: messageIDs,
		Responses:  responses,
		Errors:     errors,
	}, nil
}

// CreateTopic creates a new topic
func (s *service) CreateTopic(ctx context.Context, req *CreateTopicRequest) error {
	// Check if topic already exists
	exists, err := s.topicRepo.Exists(ctx, req.Name)
	if err != nil {
		return fmt.Errorf("failed to check topic existence: %w", err)
	}
	if exists {
		return fmt.Errorf("topic %s already exists", req.Name)
	}

	// Set defaults
	partitions := req.Partitions
	if partitions <= 0 {
		partitions = s.config.DefaultPartitions
	}

	replication := req.Replication
	if replication <= 0 {
		replication = s.config.DefaultReplication
	}

	retentionTime := req.RetentionTime
	if retentionTime <= 0 {
		retentionTime = 24 * time.Hour // Default 24 hours
	}

	// Create topic entity
	topicEntity := topic.NewTopic(req.Name, partitions)
	topicEntity.Replication = replication
	topicEntity.RetentionTime = retentionTime
	if req.MaxSize > 0 {
		topicEntity.MaxSize = req.MaxSize
	}
	if req.Config != nil {
		topicEntity.Config = req.Config
	}

	// Store topic
	if err := s.topicRepo.Create(ctx, topicEntity); err != nil {
		return fmt.Errorf("failed to create topic: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"topic":      req.Name,
		"partitions": partitions,
		"retention":  retentionTime,
	}).Info("Topic created successfully")

	return nil
}

// DeleteTopic deletes a topic and all its messages
func (s *service) DeleteTopic(ctx context.Context, topicName string) error {
	// Check if topic exists
	exists, err := s.topicRepo.Exists(ctx, topicName)
	if err != nil {
		return fmt.Errorf("failed to check topic existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("topic %s does not exist", topicName)
	}

	// Delete topic
	if err := s.topicRepo.Delete(ctx, topicName); err != nil {
		return fmt.Errorf("failed to delete topic: %w", err)
	}

	s.logger.WithField("topic", topicName).Info("Topic deleted successfully")
	return nil
}

// GetTopicStats returns statistics for a topic
func (s *service) GetTopicStats(ctx context.Context, topicName string) (*TopicStats, error) {
	topicEntity, err := s.topicRepo.Get(ctx, topicName)
	if err != nil {
		return nil, fmt.Errorf("failed to get topic: %w", err)
	}

	messageCount, err := s.messageRepo.CountByTopic(ctx, topicName)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to get message count for topic")
		messageCount = topicEntity.MessageCount
	}

	_, bytesIn, bytesOut := topicEntity.GetStats()

	return &TopicStats{
		Topic:        topicEntity.Name,
		Partitions:   topicEntity.Partitions,
		Messages:     messageCount,
		Consumers:    0, // TODO: Get from subscription service
		Rate:         "0 msg/s", // TODO: Calculate rate
		Size:         formatBytes(bytesIn),
		BytesIn:      bytesIn,
		BytesOut:     bytesOut,
		CreatedAt:    topicEntity.CreatedAt,
		UpdatedAt:    topicEntity.UpdatedAt,
	}, nil
}

// ListTopics returns all available topics
func (s *service) ListTopics(ctx context.Context) ([]*TopicInfo, error) {
	topics, err := s.topicRepo.List(ctx, 0, 1000) // TODO: Add pagination
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}

	result := make([]*TopicInfo, len(topics))
	for i, t := range topics {
		result[i] = &TopicInfo{
			Name:          t.Name,
			Partitions:    t.Partitions,
			MessageCount:  t.MessageCount,
			RetentionTime: t.RetentionTime,
			CreatedAt:     t.CreatedAt,
			Config:        t.Config,
		}
	}

	return result, nil
}

// Helper methods

func (s *service) validatePublishRequest(req *PublishRequest) error {
	if req.Topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}
	if len(req.Payload) == 0 {
		return fmt.Errorf("payload cannot be empty")
	}
	if len(req.Payload) > int(s.config.MaxMessageBatchSize*1024) { // Rough size check
		return fmt.Errorf("payload too large")
	}
	return nil
}

func (s *service) validateBatchMessage(msg *MessageBatch) error {
	if len(msg.Payload) == 0 {
		return fmt.Errorf("payload cannot be empty")
	}
	return nil
}

func (s *service) ensureTopicExists(ctx context.Context, topicName string) (*topic.Topic, error) {
	topicEntity, err := s.topicRepo.Get(ctx, topicName)
	if err != nil {
		// Topic doesn't exist, create it with defaults
		createReq := &CreateTopicRequest{
			Name:       topicName,
			Partitions: s.config.DefaultPartitions,
		}
		if err := s.CreateTopic(ctx, createReq); err != nil {
			return nil, err
		}
		return s.topicRepo.Get(ctx, topicName)
	}
	return topicEntity, nil
}

func (s *service) createMessage(req *PublishRequest, topicEntity *topic.Topic) *message.Message {
	msg := &message.Message{
		ID:           uuid.New().String(),
		Topic:        req.Topic,
		Key:          req.Key,
		Payload:      req.Payload,
		Headers:      req.Headers,
		Priority:     req.Priority,
		DeliveryMode: req.DeliveryMode,
		Status:       message.StatusPending,
		MaxRetries:   s.config.MaxRetries,
		CreatedAt:    time.Now().UTC(),
	}

	// Set defaults
	if msg.Priority == 0 {
		msg.Priority = message.PriorityNormal
	}
	if msg.DeliveryMode == 0 {
		msg.DeliveryMode = message.DeliveryAtLeastOnce
	}

	// Set expiry if TTL is specified
	if req.TTL != nil {
		expiry := time.Now().UTC().Add(*req.TTL)
		msg.ExpiresAt = &expiry
	}

	// Determine partition
	if req.Partition != nil {
		msg.Partition = *req.Partition
	} else {
		// Simple hash-based partitioning (you might want to improve this)
		if len(req.Key) > 0 {
			msg.Partition = int32(hash(req.Key) % int(topicEntity.Partitions))
		} else {
			msg.Partition = int32(hash([]byte(msg.ID)) % int(topicEntity.Partitions))
		}
	}

	return msg
}

func (s *service) shouldCompress(payload []byte) bool {
	return s.config.CompressionType != "none" && len(payload) > 1024 // Compress if > 1KB
}

// Simple hash function for partitioning
func hash(data []byte) int {
	h := 0
	for _, b := range data {
		h = h*31 + int(b)
	}
	if h < 0 {
		h = -h
	}
	return h
}

// Format bytes into human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}