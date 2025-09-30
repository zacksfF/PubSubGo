package subscriber

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/zacksfF/PubSubGo/internal/config"
	"github.com/zacksfF/PubSubGo/internal/core/consumer"
	"github.com/zacksfF/PubSubGo/internal/core/message"
	"github.com/zacksfF/PubSubGo/internal/core/subscription"
	"github.com/zacksfF/PubSubGo/internal/ports/repository"
)

// Service defines the subscriber service interface
type Service interface {
	// Subscribe creates a subscription and starts consuming messages
	Subscribe(ctx context.Context, req *SubscribeRequest) (*SubscribeResponse, error)

	// Unsubscribe stops consuming and removes the subscription
	Unsubscribe(ctx context.Context, subscriptionID string) error

	// Pull retrieves messages from a topic (pull mode)
	Pull(ctx context.Context, req *PullRequest) (*PullResponse, error)

	// PullWithConsumerGroup retrieves messages for a consumer group
	PullWithConsumerGroup(ctx context.Context, req *ConsumerGroupPullRequest) (*PullResponse, error)

	// Acknowledge acknowledges message processing
	Acknowledge(ctx context.Context, req *AcknowledgeRequest) error

	// NegativeAcknowledge marks message as failed for retry
	NegativeAcknowledge(ctx context.Context, req *NackRequest) error

	// GetSubscription returns subscription details
	GetSubscription(ctx context.Context, subscriptionID string) (*SubscriptionInfo, error)

	// ListSubscriptions returns all subscriptions for a topic
	ListSubscriptions(ctx context.Context, topic string) ([]*SubscriptionInfo, error)

	// JoinConsumerGroup adds a consumer to a group
	JoinConsumerGroup(ctx context.Context, req *JoinGroupRequest) (*JoinGroupResponse, error)

	// LeaveConsumerGroup removes a consumer from a group
	LeaveConsumerGroup(ctx context.Context, groupName, consumerID string) error

	// GetConsumerGroupInfo returns consumer group details
	GetConsumerGroupInfo(ctx context.Context, groupName string) (*ConsumerGroupInfo, error)
}

// service implements the subscriber service
type service struct {
	messageRepo      repository.MessageRepository
	subscriptionRepo repository.SubscriptionRepository
	consumerRepo     repository.ConsumerGroupRepository
	topicRepo        repository.TopicRepository
	config           *config.BrokerConfig
	logger           *logrus.Logger

	// Active subscriptions and their channels
	subscriptions map[string]*activeSubscription
	subsMutex     sync.RWMutex

	// Consumer group management
	consumerGroups map[string]*activeConsumerGroup
	groupsMutex    sync.RWMutex

	// Background workers
	workers      map[string]context.CancelFunc
	workersMutex sync.RWMutex
}

// Request/Response types

type SubscribeRequest struct {
	Topic           string            `json:"topic"`
	ConsumerGroup   string            `json:"consumer_group,omitempty"`
	ConsumerID      string            `json:"consumer_id,omitempty"`
	Type            subscription.Type `json:"type"` // Pull or Push
	AckDeadline     time.Duration     `json:"ack_deadline,omitempty"`
	MaxRetries      int               `json:"max_retries,omitempty"`
	DeadLetterTopic string            `json:"dead_letter_topic,omitempty"`
	Filter          string            `json:"filter,omitempty"`
	Config          map[string]string `json:"config,omitempty"`
}

type SubscribeResponse struct {
	SubscriptionID string                  `json:"subscription_id"`
	Topic          string                  `json:"topic"`
	ConsumerGroup  string                  `json:"consumer_group,omitempty"`
	ConsumerID     string                  `json:"consumer_id,omitempty"`
	Type           string                  `json:"type"`
	MessageChan    <-chan *message.Message `json:"-"` // Only for push subscriptions
}

type PullRequest struct {
	Topic       string        `json:"topic"`
	Partition   *int32        `json:"partition,omitempty"`
	Offset      *int64        `json:"offset,omitempty"`
	Limit       int           `json:"limit"`
	Timeout     time.Duration `json:"timeout,omitempty"`
	AckDeadline time.Duration `json:"ack_deadline,omitempty"`
}

type ConsumerGroupPullRequest struct {
	Topic         string        `json:"topic"`
	ConsumerGroup string        `json:"consumer_group"`
	ConsumerID    string        `json:"consumer_id"`
	Limit         int           `json:"limit"`
	Timeout       time.Duration `json:"timeout,omitempty"`
	AckDeadline   time.Duration `json:"ack_deadline,omitempty"`
}

type PullResponse struct {
	Messages   []*message.Message `json:"messages"`
	NextOffset *int64             `json:"next_offset,omitempty"`
	HasMore    bool               `json:"has_more"`
	ConsumerID string             `json:"consumer_id,omitempty"`
}

type AcknowledgeRequest struct {
	MessageID     string `json:"message_id"`
	ConsumerID    string `json:"consumer_id"`
	ConsumerGroup string `json:"consumer_group,omitempty"`
}

type NackRequest struct {
	MessageID     string `json:"message_id"`
	ConsumerID    string `json:"consumer_id"`
	ConsumerGroup string `json:"consumer_group,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Retry         bool   `json:"retry"`
}

type JoinGroupRequest struct {
	Topic         string            `json:"topic"`
	ConsumerGroup string            `json:"consumer_group"`
	ConsumerID    string            `json:"consumer_id,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type JoinGroupResponse struct {
	ConsumerID    string  `json:"consumer_id"`
	ConsumerGroup string  `json:"consumer_group"`
	Partitions    []int32 `json:"partitions"`
}

type SubscriptionInfo struct {
	ID              string             `json:"id"`
	Topic           string             `json:"topic"`
	ConsumerGroup   string             `json:"consumer_group,omitempty"`
	Type            subscription.Type  `json:"type"`
	Active          bool               `json:"active"`
	AckDeadline     time.Duration      `json:"ack_deadline"`
	MaxRetries      int                `json:"max_retries"`
	DeadLetterTopic string             `json:"dead_letter_topic,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	Stats           *SubscriptionStats `json:"stats"`
}

type SubscriptionStats struct {
	MessagesDelivered int64      `json:"messages_delivered"`
	MessagesAcked     int64      `json:"messages_acked"`
	MessagesNacked    int64      `json:"messages_nacked"`
	MessagesDLQ       int64      `json:"messages_dlq"`
	LastAckTime       *time.Time `json:"last_ack_time,omitempty"`
}

type ConsumerGroupInfo struct {
	Name         string          `json:"name"`
	Topic        string          `json:"topic"`
	Consumers    []*ConsumerInfo `json:"consumers"`
	MaxConsumers int             `json:"max_consumers"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type ConsumerInfo struct {
	ID            string                  `json:"id"`
	Status        consumer.ConsumerStatus `json:"status"`
	Partitions    []int32                 `json:"partitions"`
	LastHeartbeat time.Time               `json:"last_heartbeat"`
	Metadata      map[string]string       `json:"metadata,omitempty"`
}

// Internal types for active subscriptions and consumer groups

type activeSubscription struct {
	subscription *subscription.Subscription
	messageChan  chan *message.Message
	cancelFunc   context.CancelFunc
	lastActivity time.Time
}

type activeConsumerGroup struct {
	group         *consumer.ConsumerGroup
	consumers     map[string]*activeConsumer
	rebalancing   bool
	lastRebalance time.Time
}

type activeConsumer struct {
	consumer    *consumer.Consumer
	messageChan chan *message.Message
	lastPull    time.Time
}

// NewService creates a new subscriber service
func NewService(
	messageRepo repository.MessageRepository,
	subscriptionRepo repository.SubscriptionRepository,
	consumerRepo repository.ConsumerGroupRepository,
	topicRepo repository.TopicRepository,
	config *config.BrokerConfig,
	logger *logrus.Logger,
) Service {
	svc := &service{
		messageRepo:      messageRepo,
		subscriptionRepo: subscriptionRepo,
		consumerRepo:     consumerRepo,
		topicRepo:        topicRepo,
		config:           config,
		logger:           logger,
		subscriptions:    make(map[string]*activeSubscription),
		consumerGroups:   make(map[string]*activeConsumerGroup),
		workers:          make(map[string]context.CancelFunc),
	}

	// Start background workers
	go svc.startHeartbeatWorker()
	go svc.startRebalanceWorker()
	go svc.startCleanupWorker()

	return svc
}

// Subscribe creates a new subscription
func (s *service) Subscribe(ctx context.Context, req *SubscribeRequest) (*SubscribeResponse, error) {
	// Validate request
	if err := s.validateSubscribeRequest(req); err != nil {
		return nil, fmt.Errorf("invalid subscribe request: %w", err)
	}

	// Check if topic exists
	exists, err := s.topicRepo.Exists(ctx, req.Topic)
	if err != nil {
		return nil, fmt.Errorf("failed to check topic existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("topic %s does not exist", req.Topic)
	}

	// Generate subscription ID if not provided
	subscriptionID := uuid.New().String()

	// Set defaults
	ackDeadline := req.AckDeadline
	if ackDeadline == 0 {
		ackDeadline = s.config.DefaultAckDeadline
	}

	maxRetries := req.MaxRetries
	if maxRetries == 0 {
		maxRetries = s.config.MaxRetries
	}

	// Create subscription entity
	sub := subscription.NewSubscription(subscriptionID, req.Topic, req.ConsumerGroup, req.Type)
	sub.AckDeadline = ackDeadline
	sub.MaxRetries = maxRetries
	sub.DeadLetterTopic = req.DeadLetterTopic
	sub.Filter = req.Filter
	sub.Config = req.Config

	// Store subscription
	if err := s.subscriptionRepo.Create(ctx, sub); err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	// Create message channel for push subscriptions
	var messageChan chan *message.Message
	if req.Type == subscription.TypePush {
		messageChan = make(chan *message.Message, 100) // Buffered channel
	}

	// Create active subscription
	activeSub := &activeSubscription{
		subscription: sub,
		messageChan:  messageChan,
		lastActivity: time.Now(),
	}

	// Start subscription worker for push mode
	if req.Type == subscription.TypePush {
		workerCtx, cancel := context.WithCancel(context.Background())
		activeSub.cancelFunc = cancel
		go s.subscriptionWorker(workerCtx, activeSub)
	}

	// Store active subscription
	s.subsMutex.Lock()
	s.subscriptions[subscriptionID] = activeSub
	s.subsMutex.Unlock()

	s.logger.WithFields(logrus.Fields{
		"subscription_id": subscriptionID,
		"topic":           req.Topic,
		"consumer_group":  req.ConsumerGroup,
		"type":            req.Type,
	}).Info("Subscription created")

	return &SubscribeResponse{
		SubscriptionID: subscriptionID,
		Topic:          req.Topic,
		ConsumerGroup:  req.ConsumerGroup,
		ConsumerID:     req.ConsumerID,
		Type:           getTypeString(req.Type),
		MessageChan:    messageChan,
	}, nil
}

// Unsubscribe removes a subscription
func (s *service) Unsubscribe(ctx context.Context, subscriptionID string) error {
	s.subsMutex.Lock()
	activeSub, exists := s.subscriptions[subscriptionID]
	if exists {
		delete(s.subscriptions, subscriptionID)
	}
	s.subsMutex.Unlock()

	if !exists {
		return fmt.Errorf("subscription %s not found", subscriptionID)
	}

	// Cancel subscription worker
	if activeSub.cancelFunc != nil {
		activeSub.cancelFunc()
	}

	// Close message channel
	if activeSub.messageChan != nil {
		close(activeSub.messageChan)
	}

	// Deactivate subscription in repository
	activeSub.subscription.Deactivate()
	if err := s.subscriptionRepo.Update(ctx, activeSub.subscription); err != nil {
		s.logger.WithError(err).Warn("Failed to update subscription status")
	}

	s.logger.WithField("subscription_id", subscriptionID).Info("Subscription removed")
	return nil
}

// Pull retrieves messages from a topic
func (s *service) Pull(ctx context.Context, req *PullRequest) (*PullResponse, error) {
	if err := s.validatePullRequest(req); err != nil {
		return nil, fmt.Errorf("invalid pull request: %w", err)
	}

	var messages []*message.Message
	var err error

	if req.Partition != nil {
		// Pull from specific partition
		offset := int64(0)
		if req.Offset != nil {
			offset = *req.Offset
		}
		messages, err = s.messageRepo.GetByTopicPartition(ctx, req.Topic, *req.Partition, offset, req.Limit)
	} else {
		// Pull from topic (all partitions)
		offset := int64(0)
		if req.Offset != nil {
			offset = *req.Offset
		}
		messages, err = s.messageRepo.GetByTopic(ctx, req.Topic, req.Limit, offset)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve messages: %w", err)
	}

	// Filter expired messages
	validMessages := make([]*message.Message, 0, len(messages))
	for _, msg := range messages {
		if !msg.IsExpired() {
			validMessages = append(validMessages, msg)
		}
	}

	// Mark messages as delivered if ack deadline is set
	if req.AckDeadline > 0 {
		consumerID := uuid.New().String()
		now := time.Now()
		for _, msg := range validMessages {
			msg.Status = message.StatusDelivered
			msg.DeliveredAt = &now
			msg.ConsumerID = consumerID
			// Store updated message (in production, you might want to batch this)
			go func(m *message.Message) {
				if err := s.messageRepo.Store(ctx, m); err != nil {
					s.logger.WithError(err).Warn("Failed to update message delivery status")
				}
			}(msg)
		}
	}

	// Calculate next offset
	var nextOffset *int64
	if len(validMessages) > 0 {
		lastOffset := validMessages[len(validMessages)-1].Offset
		next := lastOffset + 1
		nextOffset = &next
	}

	return &PullResponse{
		Messages:   validMessages,
		NextOffset: nextOffset,
		HasMore:    len(validMessages) == req.Limit,
	}, nil
}

// PullWithConsumerGroup retrieves messages for a consumer group
func (s *service) PullWithConsumerGroup(ctx context.Context, req *ConsumerGroupPullRequest) (*PullResponse, error) {
	if err := s.validateConsumerGroupPullRequest(req); err != nil {
		return nil, fmt.Errorf("invalid consumer group pull request: %w", err)
	}

	// Ensure consumer group exists and consumer is part of it
	group, err := s.ensureConsumerInGroup(ctx, req.Topic, req.ConsumerGroup, req.ConsumerID)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure consumer in group: %w", err)
	}

	// Get messages for consumer group
	messages, err := s.messageRepo.GetForConsumerGroup(ctx, req.Topic, req.ConsumerGroup, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve messages for consumer group: %w", err)
	}

	// Filter and assign messages to this consumer
	consumerMessages := make([]*message.Message, 0)
	now := time.Now()

	for _, msg := range messages {
		if !msg.IsExpired() && s.shouldAssignToConsumer(msg, req.ConsumerID, group) {
			msg.Status = message.StatusDelivered
			msg.DeliveredAt = &now
			msg.ConsumerID = req.ConsumerID
			msg.ConsumerGroup = req.ConsumerGroup
			consumerMessages = append(consumerMessages, msg)

			// Mark as delivered
			go func(m *message.Message) {
				if err := s.messageRepo.MarkDelivered(ctx, m.ID, req.ConsumerID); err != nil {
					s.logger.WithError(err).Warn("Failed to mark message as delivered")
				}
			}(msg)
		}
	}

	// Update consumer heartbeat
	s.updateConsumerHeartbeat(req.ConsumerGroup, req.ConsumerID)

	return &PullResponse{
		Messages:   consumerMessages,
		HasMore:    len(messages) == req.Limit,
		ConsumerID: req.ConsumerID,
	}, nil
}

// Acknowledge acknowledges message processing
func (s *service) Acknowledge(ctx context.Context, req *AcknowledgeRequest) error {
	ack := &message.Acknowledgment{
		MessageID:     req.MessageID,
		ConsumerID:    req.ConsumerID,
		ConsumerGroup: req.ConsumerGroup,
		Success:       true,
		Timestamp:     time.Now(),
	}

	if err := s.messageRepo.Acknowledge(ctx, ack); err != nil {
		return fmt.Errorf("failed to acknowledge message: %w", err)
	}

	// Update subscription stats if consumer group is provided
	if req.ConsumerGroup != "" {
		s.updateSubscriptionStats(req.ConsumerGroup, "acked")
	}

	s.logger.WithFields(logrus.Fields{
		"message_id":     req.MessageID,
		"consumer_id":    req.ConsumerID,
		"consumer_group": req.ConsumerGroup,
	}).Debug("Message acknowledged")

	return nil
}

// NegativeAcknowledge marks a message as failed
func (s *service) NegativeAcknowledge(ctx context.Context, req *NackRequest) error {
	ack := &message.Acknowledgment{
		MessageID:     req.MessageID,
		ConsumerID:    req.ConsumerID,
		ConsumerGroup: req.ConsumerGroup,
		Success:       false,
		Reason:        req.Reason,
		Timestamp:     time.Now(),
	}

	if err := s.messageRepo.Acknowledge(ctx, ack); err != nil {
		return fmt.Errorf("failed to negative acknowledge message: %w", err)
	}

	// Update subscription stats
	if req.ConsumerGroup != "" {
		s.updateSubscriptionStats(req.ConsumerGroup, "nacked")
	}

	s.logger.WithFields(logrus.Fields{
		"message_id":     req.MessageID,
		"consumer_id":    req.ConsumerID,
		"consumer_group": req.ConsumerGroup,
		"reason":         req.Reason,
		"retry":          req.Retry,
	}).Debug("Message negative acknowledged")

	return nil
}

// Helper methods

func (s *service) validateSubscribeRequest(req *SubscribeRequest) error {
	if req.Topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}
	if req.Type != subscription.TypePull && req.Type != subscription.TypePush {
		return fmt.Errorf("invalid subscription type")
	}
	return nil
}

func (s *service) validatePullRequest(req *PullRequest) error {
	if req.Topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}
	if req.Limit <= 0 || req.Limit > 1000 {
		return fmt.Errorf("limit must be between 1 and 1000")
	}
	return nil
}

func (s *service) validateConsumerGroupPullRequest(req *ConsumerGroupPullRequest) error {
	if req.Topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}
	if req.ConsumerGroup == "" {
		return fmt.Errorf("consumer group cannot be empty")
	}
	if req.ConsumerID == "" {
		return fmt.Errorf("consumer ID cannot be empty")
	}
	if req.Limit <= 0 || req.Limit > 1000 {
		return fmt.Errorf("limit must be between 1 and 1000")
	}
	return nil
}

func (s *service) ensureConsumerInGroup(ctx context.Context, topic, groupName, consumerID string) (*consumer.ConsumerGroup, error) {
	// Get or create consumer group
	group, err := s.consumerRepo.GetGroup(ctx, groupName)
	if err != nil {
		// Create new consumer group
		group = consumer.NewConsumerGroup(groupName, topic)
		if err := s.consumerRepo.CreateGroup(ctx, group); err != nil {
			return nil, fmt.Errorf("failed to create consumer group: %w", err)
		}
	}

	// Add consumer to group if not already present
	consumerEntity := &consumer.Consumer{
		ID:            consumerID,
		GroupID:       groupName,
		LastHeartbeat: time.Now(),
		Status:        consumer.ConsumerActive,
		Metadata:      make(map[string]string),
	}

	if err := s.consumerRepo.AddConsumer(ctx, groupName, consumerEntity); err != nil {
		s.logger.WithError(err).Debug("Consumer may already be in group")
	}

	return group, nil
}

func (s *service) shouldAssignToConsumer(msg *message.Message, consumerID string, group *consumer.ConsumerGroup) bool {
	// Simple round-robin assignment based on message partition and consumer
	// In production, you'd implement proper partition assignment
	if len(group.Consumers) == 0 {
		return true // If no consumers, assign to any
	}
	return hash([]byte(consumerID))%len(group.Consumers) == int(msg.Partition)%len(group.Consumers)
}

func (s *service) updateConsumerHeartbeat(groupName, consumerID string) {
	s.groupsMutex.Lock()
	defer s.groupsMutex.Unlock()

	if group, exists := s.consumerGroups[groupName]; exists {
		if consumer, exists := group.consumers[consumerID]; exists {
			consumer.lastPull = time.Now()
		}
	}
}

func (s *service) updateSubscriptionStats(consumerGroup, statType string) {
	// This would update subscription statistics
	// Implementation depends on how you want to track stats
}

func getTypeString(t subscription.Type) string {
	switch t {
	case subscription.TypePull:
		return "pull"
	case subscription.TypePush:
		return "push"
	default:
		return "unknown"
	}
}

// Simple hash function for consumer assignment
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
