package subscriber

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/zacksfF/PubSubGo/internal/core/consumer"
	"github.com/zacksfF/PubSubGo/internal/core/message"
)

// GetSubscription returns subscription details
func (s *service) GetSubscription(ctx context.Context, subscriptionID string) (*SubscriptionInfo, error) {
	// Get from active subscriptions first
	s.subsMutex.RLock()
	activeSub, exists := s.subscriptions[subscriptionID]
	s.subsMutex.RUnlock()

	if exists {
		sub := activeSub.subscription
		return &SubscriptionInfo{
			ID:              sub.ID,
			Topic:           sub.Topic,
			ConsumerGroup:   sub.ConsumerGroup,
			Type:            sub.Type,
			Active:          sub.Active,
			AckDeadline:     sub.AckDeadline,
			MaxRetries:      sub.MaxRetries,
			DeadLetterTopic: sub.DeadLetterTopic,
			CreatedAt:       sub.CreatedAt,
			Stats: &SubscriptionStats{
				MessagesDelivered: sub.MessagesDelivered,
				MessagesAcked:     sub.MessagesAcked,
				MessagesNacked:    sub.MessagesNacked,
				MessagesDLQ:       sub.MessagesDLQ,
				LastAckTime:       sub.LastAckTime,
			},
		}, nil
	}

	// Get from repository
	sub, err := s.subscriptionRepo.Get(ctx, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("subscription not found: %w", err)
	}

	return &SubscriptionInfo{
		ID:              sub.ID,
		Topic:           sub.Topic,
		ConsumerGroup:   sub.ConsumerGroup,
		Type:            sub.Type,
		Active:          sub.Active,
		AckDeadline:     sub.AckDeadline,
		MaxRetries:      sub.MaxRetries,
		DeadLetterTopic: sub.DeadLetterTopic,
		CreatedAt:       sub.CreatedAt,
		Stats: &SubscriptionStats{
			MessagesDelivered: sub.MessagesDelivered,
			MessagesAcked:     sub.MessagesAcked,
			MessagesNacked:    sub.MessagesNacked,
			MessagesDLQ:       sub.MessagesDLQ,
			LastAckTime:       sub.LastAckTime,
		},
	}, nil
}

// ListSubscriptions returns all subscriptions for a topic
func (s *service) ListSubscriptions(ctx context.Context, topic string) ([]*SubscriptionInfo, error) {
	subscriptions, err := s.subscriptionRepo.GetByTopic(ctx, topic)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriptions for topic: %w", err)
	}

	result := make([]*SubscriptionInfo, len(subscriptions))
	for i, sub := range subscriptions {
		result[i] = &SubscriptionInfo{
			ID:              sub.ID,
			Topic:           sub.Topic,
			ConsumerGroup:   sub.ConsumerGroup,
			Type:            sub.Type,
			Active:          sub.Active,
			AckDeadline:     sub.AckDeadline,
			MaxRetries:      sub.MaxRetries,
			DeadLetterTopic: sub.DeadLetterTopic,
			CreatedAt:       sub.CreatedAt,
			Stats: &SubscriptionStats{
				MessagesDelivered: sub.MessagesDelivered,
				MessagesAcked:     sub.MessagesAcked,
				MessagesNacked:    sub.MessagesNacked,
				MessagesDLQ:       sub.MessagesDLQ,
				LastAckTime:       sub.LastAckTime,
			},
		}
	}

	return result, nil
}

// JoinConsumerGroup adds a consumer to a group
func (s *service) JoinConsumerGroup(ctx context.Context, req *JoinGroupRequest) (*JoinGroupResponse, error) {
	if err := s.validateJoinGroupRequest(req); err != nil {
		return nil, fmt.Errorf("invalid join group request: %w", err)
	}

	// Generate consumer ID if not provided
	consumerID := req.ConsumerID
	if consumerID == "" {
		consumerID = uuid.New().String()
	}

	// Ensure consumer group exists
	group, err := s.ensureConsumerInGroup(ctx, req.Topic, req.ConsumerGroup, consumerID)
	if err != nil {
		return nil, fmt.Errorf("failed to join consumer group: %w", err)
	}

	// Create active consumer
	activeCons := &activeConsumer{
		consumer: &consumer.Consumer{
			ID:            consumerID,
			GroupID:       req.ConsumerGroup,
			LastHeartbeat: time.Now(),
			Status:        consumer.ConsumerActive,
			Metadata:      req.Metadata,
		},
		messageChan: make(chan *message.Message, 100),
		lastPull:    time.Now(),
	}

	// Add to active consumer groups
	s.groupsMutex.Lock()
	if _, exists := s.consumerGroups[req.ConsumerGroup]; !exists {
		s.consumerGroups[req.ConsumerGroup] = &activeConsumerGroup{
			group:       group,
			consumers:   make(map[string]*activeConsumer),
			rebalancing: true, // Trigger rebalance
		}
	}
	s.consumerGroups[req.ConsumerGroup].consumers[consumerID] = activeCons
	s.groupsMutex.Unlock()

	// Trigger rebalancing
	s.triggerRebalance(req.ConsumerGroup)

	// Get assigned partitions (after rebalancing)
	partitions := activeCons.consumer.Partitions

	s.logger.WithFields(map[string]interface{}{
		"consumer_id":    consumerID,
		"consumer_group": req.ConsumerGroup,
		"topic":          req.Topic,
		"partitions":     partitions,
	}).Info("Consumer joined group")

	return &JoinGroupResponse{
		ConsumerID:    consumerID,
		ConsumerGroup: req.ConsumerGroup,
		Partitions:    partitions,
	}, nil
}

// LeaveConsumerGroup removes a consumer from a group
func (s *service) LeaveConsumerGroup(ctx context.Context, groupName, consumerID string) error {
	// Remove from repository
	if err := s.consumerRepo.RemoveConsumer(ctx, groupName, consumerID); err != nil {
		s.logger.WithError(err).Warn("Failed to remove consumer from repository")
	}

	// Remove from active consumers
	s.groupsMutex.Lock()
	if group, exists := s.consumerGroups[groupName]; exists {
		if activeConsumer, exists := group.consumers[consumerID]; exists {
			// Close message channel
			if activeConsumer.messageChan != nil {
				close(activeConsumer.messageChan)
			}
			delete(group.consumers, consumerID)
		}
		
		// Trigger rebalance
		group.rebalancing = true
	}
	s.groupsMutex.Unlock()

	s.logger.WithFields(map[string]interface{}{
		"consumer_id":    consumerID,
		"consumer_group": groupName,
	}).Info("Consumer left group")

	return nil
}

// GetConsumerGroupInfo returns consumer group details
func (s *service) GetConsumerGroupInfo(ctx context.Context, groupName string) (*ConsumerGroupInfo, error) {
	// Get from repository
	group, err := s.consumerRepo.GetGroup(ctx, groupName)
	if err != nil {
		return nil, fmt.Errorf("consumer group not found: %w", err)
	}

	// Get active consumers
	activeConsumers, err := s.consumerRepo.GetActiveConsumers(ctx, groupName)
	if err != nil {
		return nil, fmt.Errorf("failed to get active consumers: %w", err)
	}

	// Convert to consumer info
	consumers := make([]*ConsumerInfo, len(activeConsumers))
	for i, c := range activeConsumers {
		consumers[i] = &ConsumerInfo{
			ID:            c.ID,
			Status:        c.Status,
			Partitions:    c.Partitions,
			LastHeartbeat: c.LastHeartbeat,
			Metadata:      c.Metadata,
		}
	}

	return &ConsumerGroupInfo{
		Name:         group.Name,
		Topic:        group.Topic,
		Consumers:    consumers,
		MaxConsumers: group.MaxConsumers,
		CreatedAt:    group.CreatedAt,
		UpdatedAt:    group.UpdatedAt,
	}, nil
}

// Helper methods

func (s *service) validateJoinGroupRequest(req *JoinGroupRequest) error {
	if req.Topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}
	if req.ConsumerGroup == "" {
		return fmt.Errorf("consumer group cannot be empty")
	}
	return nil
}

func (s *service) triggerRebalance(groupName string) {
	s.groupsMutex.Lock()
	defer s.groupsMutex.Unlock()

	if group, exists := s.consumerGroups[groupName]; exists {
		group.rebalancing = true
	}
}