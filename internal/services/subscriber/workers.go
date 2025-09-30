package subscriber

import (
	"context"
	"time"

	"github.com/zacksfF/PubSubGo/internal/core/message"
)

// subscriptionWorker handles push-mode message delivery for a subscription
func (s *service) subscriptionWorker(ctx context.Context, activeSub *activeSubscription) {
	ticker := time.NewTicker(100 * time.Millisecond) // Poll every 100ms
	defer ticker.Stop()

	s.logger.WithFields(map[string]interface{}{
		"subscription_id": activeSub.subscription.ID,
		"topic":          activeSub.subscription.Topic,
		"consumer_group": activeSub.subscription.ConsumerGroup,
	}).Info("Starting subscription worker")

	for {
		select {
		case <-ctx.Done():
			s.logger.WithField("subscription_id", activeSub.subscription.ID).Info("Subscription worker stopped")
			return
		case <-ticker.C:
			s.processSubscriptionMessages(ctx, activeSub)
		}
	}
}

// processSubscriptionMessages fetches and delivers messages for a subscription
func (s *service) processSubscriptionMessages(ctx context.Context, activeSub *activeSubscription) {
	sub := activeSub.subscription
	
	// Skip if subscription is not active
	if !sub.IsActive() {
		return
	}

	var messages []*message.Message
	var err error

	if sub.ConsumerGroup != "" {
		// Consumer group mode
		messages, err = s.messageRepo.GetForConsumerGroup(ctx, sub.Topic, sub.ConsumerGroup, 10)
	} else {
		// Individual subscription mode
		messages, err = s.messageRepo.GetByTopic(ctx, sub.Topic, 10, 0)
	}

	if err != nil {
		s.logger.WithError(err).Warn("Failed to fetch messages for subscription")
		return
	}

	// Filter and deliver messages
	for _, msg := range messages {
		if msg.IsExpired() {
			continue
		}

		// Apply filters if configured
		if sub.Filter != "" && !s.matchesFilter(msg, sub.Filter) {
			continue
		}

		// Deliver message to channel
		select {
		case activeSub.messageChan <- msg:
			// Update subscription stats
			sub.IncrementDelivered()
			activeSub.lastActivity = time.Now()
			
			// Mark message as delivered
			now := time.Now()
			msg.Status = message.StatusDelivered
			msg.DeliveredAt = &now
			msg.ConsumerGroup = sub.ConsumerGroup
			
			// Store updated message status
			go func(m *message.Message) {
				if err := s.messageRepo.Store(ctx, m); err != nil {
					s.logger.WithError(err).Warn("Failed to update message delivery status")
				}
			}(msg)
			
		case <-ctx.Done():
			return
		default:
			// Channel is full, skip this message
			s.logger.Warn("Subscription message channel is full, skipping message")
		}
	}
}

// matchesFilter applies basic message filtering
func (s *service) matchesFilter(msg *message.Message, filter string) bool {
	// Simple implementation - in production you'd want a proper filter language
	// For now, just check if filter string exists in message headers or payload
	if msg.Headers != nil {
		for key, value := range msg.Headers {
			if key == filter || value == filter {
				return true
			}
		}
	}
	return false
}

// startHeartbeatWorker manages consumer heartbeats and health checks
func (s *service) startHeartbeatWorker() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.processConsumerHeartbeats()
		}
	}
}

// processConsumerHeartbeats checks and updates consumer heartbeats
func (s *service) processConsumerHeartbeats() {
	s.groupsMutex.RLock()
	groups := make(map[string]*activeConsumerGroup)
	for name, group := range s.consumerGroups {
		groups[name] = group
	}
	s.groupsMutex.RUnlock()

	for groupName, group := range groups {
		deadConsumers := make([]string, 0)
		
		for consumerID, consumer := range group.consumers {
			// Check if consumer is dead (no heartbeat for 60 seconds)
			if time.Since(consumer.lastPull) > 60*time.Second {
				deadConsumers = append(deadConsumers, consumerID)
			}
		}

		// Remove dead consumers
		for _, consumerID := range deadConsumers {
			s.removeDeadConsumer(groupName, consumerID)
		}

		// Trigger rebalance if consumers were removed
		if len(deadConsumers) > 0 {
			group.rebalancing = true
		}
	}
}

// removeDeadConsumer removes a dead consumer from the group
func (s *service) removeDeadConsumer(groupName, consumerID string) {
	ctx := context.Background()
	
	// Remove from repository
	if err := s.consumerRepo.RemoveConsumer(ctx, groupName, consumerID); err != nil {
		s.logger.WithError(err).Warn("Failed to remove dead consumer from repository")
	}

	// Remove from active consumers
	s.groupsMutex.Lock()
	if group, exists := s.consumerGroups[groupName]; exists {
		delete(group.consumers, consumerID)
	}
	s.groupsMutex.Unlock()

	s.logger.WithFields(map[string]interface{}{
		"consumer_group": groupName,
		"consumer_id":    consumerID,
	}).Info("Removed dead consumer")
}

// startRebalanceWorker handles consumer group rebalancing
func (s *service) startRebalanceWorker() {
	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.processRebalancing()
		}
	}
}

// processRebalancing handles consumer group rebalancing
func (s *service) processRebalancing() {
	s.groupsMutex.Lock()
	defer s.groupsMutex.Unlock()

	for groupName, group := range s.consumerGroups {
		if group.rebalancing && time.Since(group.lastRebalance) > 5*time.Second {
			s.rebalanceConsumerGroup(groupName, group)
			group.rebalancing = false
			group.lastRebalance = time.Now()
		}
	}
}

// rebalanceConsumerGroup reassigns partitions to consumers
func (s *service) rebalanceConsumerGroup(groupName string, group *activeConsumerGroup) {
	consumers := make([]*activeConsumer, 0, len(group.consumers))
	for _, consumer := range group.consumers {
		consumers = append(consumers, consumer)
	}

	if len(consumers) == 0 {
		return
	}

	// Get topic partitions
	ctx := context.Background()
	topic, err := s.topicRepo.Get(ctx, group.group.Topic)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to get topic for rebalancing")
		return
	}

	// Simple round-robin partition assignment
	partitionsPerConsumer := int(topic.Partitions) / len(consumers)
	extraPartitions := int(topic.Partitions) % len(consumers)

	partitionIndex := int32(0)
	for i, consumer := range consumers {
		assignedPartitions := make([]int32, 0)
		
		// Assign base partitions
		for j := 0; j < partitionsPerConsumer; j++ {
			assignedPartitions = append(assignedPartitions, partitionIndex)
			partitionIndex++
		}
		
		// Assign extra partition if needed
		if i < extraPartitions {
			assignedPartitions = append(assignedPartitions, partitionIndex)
			partitionIndex++
		}

		// Update consumer partitions
		consumer.consumer.Partitions = assignedPartitions
	}

	s.logger.WithFields(map[string]interface{}{
		"consumer_group": groupName,
		"consumers":      len(consumers),
		"partitions":     topic.Partitions,
	}).Info("Consumer group rebalanced")
}

// startCleanupWorker handles cleanup of inactive subscriptions and expired messages
func (s *service) startCleanupWorker() {
	ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanupInactiveSubscriptions()
			s.cleanupExpiredMessages()
		}
	}
}

// cleanupInactiveSubscriptions removes subscriptions that haven't been active
func (s *service) cleanupInactiveSubscriptions() {
	s.subsMutex.Lock()
	inactiveSubscriptions := make([]string, 0)
	
	for subscriptionID, activeSub := range s.subscriptions {
		// Remove subscriptions inactive for more than 1 hour
		if time.Since(activeSub.lastActivity) > time.Hour {
			inactiveSubscriptions = append(inactiveSubscriptions, subscriptionID)
		}
	}
	
	// Remove inactive subscriptions
	for _, subscriptionID := range inactiveSubscriptions {
		delete(s.subscriptions, subscriptionID)
	}
	s.subsMutex.Unlock()

	// Cancel their workers
	for _, subscriptionID := range inactiveSubscriptions {
		if activeSub := s.subscriptions[subscriptionID]; activeSub != nil && activeSub.cancelFunc != nil {
			activeSub.cancelFunc()
		}
	}

	if len(inactiveSubscriptions) > 0 {
		s.logger.WithField("count", len(inactiveSubscriptions)).Info("Cleaned up inactive subscriptions")
	}
}

// cleanupExpiredMessages removes expired messages from the system
func (s *service) cleanupExpiredMessages() {
	ctx := context.Background()
	
	deleted, err := s.messageRepo.DeleteExpired(ctx)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to delete expired messages")
		return
	}

	if deleted > 0 {
		s.logger.WithField("count", deleted).Info("Cleaned up expired messages")
	}
}