package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zacksfF/PubSubGo/internal/core/subscription"
	"github.com/zacksfF/PubSubGo/internal/ports/repository"
)

type subscriptionRepository struct {
	client *Client
}

// NewSubscriptionRepository creates a new Redis-based subscription repository
func NewSubscriptionRepository(client *Client) repository.SubscriptionRepository {
	return &subscriptionRepository{
		client: client,
	}
}

func (r *subscriptionRepository) Create(ctx context.Context, sub *subscription.Subscription) error {
	key := fmt.Sprintf("subscription:%s", sub.ID)

	data, err := json.Marshal(sub)
	if err != nil {
		return fmt.Errorf("failed to marshal subscription: %w", err)
	}

	// Store subscription data
	err = r.client.client.Set(ctx, key, data, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to store subscription: %w", err)
	}

	// Add to topic's subscription set
	topicKey := fmt.Sprintf("topic:%s:subscriptions", sub.Topic)
	err = r.client.client.SAdd(ctx, topicKey, sub.ID).Err()
	if err != nil {
		return fmt.Errorf("failed to add subscription to topic: %w", err)
	}

	// If consumer group exists, add to consumer group's subscription set
	if sub.ConsumerGroup != "" {
		groupKey := fmt.Sprintf("consumergroup:%s:subscriptions", sub.ConsumerGroup)
		err = r.client.client.SAdd(ctx, groupKey, sub.ID).Err()
		if err != nil {
			return fmt.Errorf("failed to add subscription to consumer group: %w", err)
		}
	}

	return nil
}

func (r *subscriptionRepository) Get(ctx context.Context, id string) (*subscription.Subscription, error) {
	key := fmt.Sprintf("subscription:%s", id)

	data, err := r.client.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("subscription not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	var sub subscription.Subscription
	if err := json.Unmarshal([]byte(data), &sub); err != nil {
		return nil, fmt.Errorf("failed to unmarshal subscription: %w", err)
	}

	return &sub, nil
}

func (r *subscriptionRepository) Update(ctx context.Context, sub *subscription.Subscription) error {
	key := fmt.Sprintf("subscription:%s", sub.ID)

	// Update timestamp
	sub.UpdatedAt = time.Now()

	data, err := json.Marshal(sub)
	if err != nil {
		return fmt.Errorf("failed to marshal subscription: %w", err)
	}

	err = r.client.client.Set(ctx, key, data, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to update subscription: %w", err)
	}

	return nil
}

func (r *subscriptionRepository) Delete(ctx context.Context, id string) error {
	// Get subscription to find topic and consumer group
	sub, err := r.Get(ctx, id)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("subscription:%s", id)

	// Remove from topic's subscription set
	topicKey := fmt.Sprintf("topic:%s:subscriptions", sub.Topic)
	err = r.client.client.SRem(ctx, topicKey, id).Err()
	if err != nil {
		return fmt.Errorf("failed to remove subscription from topic: %w", err)
	}

	// Remove from consumer group's subscription set if applicable
	if sub.ConsumerGroup != "" {
		groupKey := fmt.Sprintf("consumergroup:%s:subscriptions", sub.ConsumerGroup)
		err = r.client.client.SRem(ctx, groupKey, id).Err()
		if err != nil {
			return fmt.Errorf("failed to remove subscription from consumer group: %w", err)
		}
	}

	// Delete subscription data
	err = r.client.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}

	return nil
}

func (r *subscriptionRepository) GetByTopic(ctx context.Context, topic string) ([]*subscription.Subscription, error) {
	topicKey := fmt.Sprintf("topic:%s:subscriptions", topic)

	// Get all subscription IDs for this topic
	subIDs, err := r.client.client.SMembers(ctx, topicKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriptions for topic: %w", err)
	}

	subscriptions := make([]*subscription.Subscription, 0, len(subIDs))
	for _, id := range subIDs {
		sub, err := r.Get(ctx, id)
		if err != nil {
			// Skip subscriptions that can't be retrieved
			continue
		}
		subscriptions = append(subscriptions, sub)
	}

	return subscriptions, nil
}

func (r *subscriptionRepository) GetByConsumerGroup(ctx context.Context, consumerGroup string) ([]*subscription.Subscription, error) {
	groupKey := fmt.Sprintf("consumergroup:%s:subscriptions", consumerGroup)

	// Get all subscription IDs for this consumer group
	subIDs, err := r.client.client.SMembers(ctx, groupKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriptions for consumer group: %w", err)
	}

	subscriptions := make([]*subscription.Subscription, 0, len(subIDs))
	for _, id := range subIDs {
		sub, err := r.Get(ctx, id)
		if err != nil {
			// Skip subscriptions that can't be retrieved
			continue
		}
		subscriptions = append(subscriptions, sub)
	}

	return subscriptions, nil
}

func (r *subscriptionRepository) GetActive(ctx context.Context) ([]*subscription.Subscription, error) {
	// Use SCAN to find all subscription keys
	iter := r.client.client.Scan(ctx, 0, "subscription:*", 0).Iterator()

	subscriptions := make([]*subscription.Subscription, 0)
	for iter.Next(ctx) {
		data, err := r.client.client.Get(ctx, iter.Val()).Result()
		if err != nil {
			continue
		}

		var sub subscription.Subscription
		if err := json.Unmarshal([]byte(data), &sub); err != nil {
			continue
		}

		// Check if subscription is active
		if sub.Active {
			subscriptions = append(subscriptions, &sub)
		}
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate subscriptions: %w", err)
	}

	return subscriptions, nil
}


func (r *subscriptionRepository) List(ctx context.Context, offset, limit int) ([]*subscription.Subscription, error) {
	// Use SCAN to find all subscription keys
	iter := r.client.client.Scan(ctx, 0, "subscription:*", 0).Iterator()

	subscriptions := make([]*subscription.Subscription, 0)
	for iter.Next(ctx) {
		data, err := r.client.client.Get(ctx, iter.Val()).Result()
		if err != nil {
			continue
		}

		var sub subscription.Subscription
		if err := json.Unmarshal([]byte(data), &sub); err != nil {
			continue
		}

		subscriptions = append(subscriptions, &sub)
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate subscriptions: %w", err)
	}

	// Apply pagination
	start := offset
	end := offset + limit
	if start > len(subscriptions) {
		return []*subscription.Subscription{}, nil
	}
	if end > len(subscriptions) {
		end = len(subscriptions)
	}

	return subscriptions[start:end], nil
}

func (r *subscriptionRepository) Exists(ctx context.Context, id string) (bool, error) {
	key := fmt.Sprintf("subscription:%s", id)

	exists, err := r.client.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check subscription existence: %w", err)
	}

	return exists > 0, nil
}
