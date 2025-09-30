package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zacksfF/PubSubGo/internal/core/consumer"
	"github.com/zacksfF/PubSubGo/internal/ports/repository"
)

type consumerGroupRepository struct {
	client *Client
}

// NewConsumerGroupRepository creates a new Redis-based consumer group repository
func NewConsumerGroupRepository(client *Client) repository.ConsumerGroupRepository {
	return &consumerGroupRepository{
		client: client,
	}
}

func (r *consumerGroupRepository) CreateGroup(ctx context.Context, group *consumer.ConsumerGroup) error {
	key := fmt.Sprintf("consumergroup:%s", group.Name)
	
	data, err := json.Marshal(group)
	if err != nil {
		return fmt.Errorf("failed to marshal consumer group: %w", err)
	}
	
	// Store consumer group data
	err = r.client.client.Set(ctx, key, data, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to store consumer group: %w", err)
	}
	
	// Add to consumer groups set
	err = r.client.client.SAdd(ctx, "consumergroups", group.Name).Err()
	if err != nil {
		return fmt.Errorf("failed to add consumer group to set: %w", err)
	}
	
	// Add to topic's consumer groups set
	topicKey := fmt.Sprintf("topic:%s:consumergroups", group.Topic)
	err = r.client.client.SAdd(ctx, topicKey, group.Name).Err()
	if err != nil {
		return fmt.Errorf("failed to add consumer group to topic: %w", err)
	}
	
	return nil
}

func (r *consumerGroupRepository) GetGroup(ctx context.Context, name string) (*consumer.ConsumerGroup, error) {
	key := fmt.Sprintf("consumergroup:%s", name)
	
	data, err := r.client.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("consumer group not found: %s", name)
		}
		return nil, fmt.Errorf("failed to get consumer group: %w", err)
	}
	
	var group consumer.ConsumerGroup
	if err := json.Unmarshal([]byte(data), &group); err != nil {
		return nil, fmt.Errorf("failed to unmarshal consumer group: %w", err)
	}
	
	return &group, nil
}

func (r *consumerGroupRepository) UpdateGroup(ctx context.Context, group *consumer.ConsumerGroup) error {
	key := fmt.Sprintf("consumergroup:%s", group.Name)
	
	// Update timestamp
	group.UpdatedAt = time.Now()
	
	data, err := json.Marshal(group)
	if err != nil {
		return fmt.Errorf("failed to marshal consumer group: %w", err)
	}
	
	err = r.client.client.Set(ctx, key, data, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to update consumer group: %w", err)
	}
	
	return nil
}

func (r *consumerGroupRepository) DeleteGroup(ctx context.Context, name string) error {
	// Get group to find topic
	group, err := r.GetGroup(ctx, name)
	if err != nil {
		return err
	}
	
	key := fmt.Sprintf("consumergroup:%s", name)
	
	// Remove from consumer groups set
	err = r.client.client.SRem(ctx, "consumergroups", name).Err()
	if err != nil {
		return fmt.Errorf("failed to remove consumer group from set: %w", err)
	}
	
	// Remove from topic's consumer groups set
	topicKey := fmt.Sprintf("topic:%s:consumergroups", group.Topic)
	err = r.client.client.SRem(ctx, topicKey, name).Err()
	if err != nil {
		return fmt.Errorf("failed to remove consumer group from topic: %w", err)
	}
	
	// Delete all consumers in this group
	consumersKey := fmt.Sprintf("consumergroup:%s:consumers", name)
	consumerIDs, _ := r.client.client.SMembers(ctx, consumersKey).Result()
	for _, consumerID := range consumerIDs {
		r.RemoveConsumer(ctx, name, consumerID)
	}
	
	// Delete consumer group data
	err = r.client.client.Del(ctx, key, consumersKey).Err()
	if err != nil {
		return fmt.Errorf("failed to delete consumer group: %w", err)
	}
	
	return nil
}

func (r *consumerGroupRepository) ListGroups(ctx context.Context, topic string) ([]*consumer.ConsumerGroup, error) {
	if topic != "" {
		return r.GetGroupsByTopic(ctx, topic)
	}
	
	// Get all consumer group names
	groupNames, err := r.client.client.SMembers(ctx, "consumergroups").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list consumer groups: %w", err)
	}
	
	groups := make([]*consumer.ConsumerGroup, 0, len(groupNames))
	for _, name := range groupNames {
		group, err := r.GetGroup(ctx, name)
		if err != nil {
			// Skip groups that can't be retrieved
			continue
		}
		groups = append(groups, group)
	}
	
	return groups, nil
}

func (r *consumerGroupRepository) GetGroupsByTopic(ctx context.Context, topic string) ([]*consumer.ConsumerGroup, error) {
	topicKey := fmt.Sprintf("topic:%s:consumergroups", topic)
	
	// Get all consumer group names for this topic
	groupNames, err := r.client.client.SMembers(ctx, topicKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get consumer groups for topic: %w", err)
	}
	
	groups := make([]*consumer.ConsumerGroup, 0, len(groupNames))
	for _, name := range groupNames {
		group, err := r.GetGroup(ctx, name)
		if err != nil {
			// Skip groups that can't be retrieved
			continue
		}
		groups = append(groups, group)
	}
	
	return groups, nil
}

func (r *consumerGroupRepository) AddConsumer(ctx context.Context, groupName string, c *consumer.Consumer) error {
	// Store consumer data
	consumerKey := fmt.Sprintf("consumer:%s:%s", groupName, c.ID)
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal consumer: %w", err)
	}
	
	err = r.client.client.Set(ctx, consumerKey, data, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to store consumer: %w", err)
	}
	
	// Add to consumer group's consumers set
	consumersKey := fmt.Sprintf("consumergroup:%s:consumers", groupName)
	err = r.client.client.SAdd(ctx, consumersKey, c.ID).Err()
	if err != nil {
		return fmt.Errorf("failed to add consumer to group: %w", err)
	}
	
	// Update consumer group
	group, err := r.GetGroup(ctx, groupName)
	if err != nil {
		return err
	}
	
	// Add consumer to group's consumers map
	if group.Consumers == nil {
		group.Consumers = make(map[string]*consumer.Consumer)
	}
	group.Consumers[c.ID] = c
	return r.UpdateGroup(ctx, group)
}

func (r *consumerGroupRepository) RemoveConsumer(ctx context.Context, groupName, consumerID string) error {
	consumerKey := fmt.Sprintf("consumer:%s:%s", groupName, consumerID)
	
	// Remove from consumer group's consumers set
	consumersKey := fmt.Sprintf("consumergroup:%s:consumers", groupName)
	err := r.client.client.SRem(ctx, consumersKey, consumerID).Err()
	if err != nil {
		return fmt.Errorf("failed to remove consumer from group: %w", err)
	}
	
	// Delete consumer data
	err = r.client.client.Del(ctx, consumerKey).Err()
	if err != nil {
		return fmt.Errorf("failed to delete consumer: %w", err)
	}
	
	// Update consumer group
	group, err := r.GetGroup(ctx, groupName)
	if err != nil {
		return err
	}
	
	// Remove consumer from group's consumers map
	if group.Consumers != nil {
		delete(group.Consumers, consumerID)
	}
	
	return r.UpdateGroup(ctx, group)
}

func (r *consumerGroupRepository) GetConsumer(ctx context.Context, groupName, consumerID string) (*consumer.Consumer, error) {
	consumerKey := fmt.Sprintf("consumer:%s:%s", groupName, consumerID)
	
	data, err := r.client.client.Get(ctx, consumerKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("consumer not found: %s in group %s", consumerID, groupName)
		}
		return nil, fmt.Errorf("failed to get consumer: %w", err)
	}
	
	var c consumer.Consumer
	if err := json.Unmarshal([]byte(data), &c); err != nil {
		return nil, fmt.Errorf("failed to unmarshal consumer: %w", err)
	}
	
	return &c, nil
}

func (r *consumerGroupRepository) UpdateConsumerHeartbeat(ctx context.Context, groupName, consumerID string) error {
	c, err := r.GetConsumer(ctx, groupName, consumerID)
	if err != nil {
		return err
	}
	
	c.LastHeartbeat = time.Now()
	c.Status = consumer.ConsumerActive
	
	// Update consumer data
	consumerKey := fmt.Sprintf("consumer:%s:%s", groupName, consumerID)
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal consumer: %w", err)
	}
	
	err = r.client.client.Set(ctx, consumerKey, data, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to update consumer heartbeat: %w", err)
	}
	
	return nil
}

func (r *consumerGroupRepository) GetActiveConsumers(ctx context.Context, groupName string) ([]*consumer.Consumer, error) {
	consumersKey := fmt.Sprintf("consumergroup:%s:consumers", groupName)
	
	// Get all consumer IDs for this group
	consumerIDs, err := r.client.client.SMembers(ctx, consumersKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get consumers for group: %w", err)
	}
	
	consumers := make([]*consumer.Consumer, 0, len(consumerIDs))
	heartbeatThreshold := time.Now().Add(-30 * time.Second) // Consider active if heartbeat within 30 seconds
	
	for _, id := range consumerIDs {
		c, err := r.GetConsumer(ctx, groupName, id)
		if err != nil {
			continue
		}
		
		// Check if consumer is active
		if c.Status == consumer.ConsumerActive && c.LastHeartbeat.After(heartbeatThreshold) {
			consumers = append(consumers, c)
		}
	}
	
	return consumers, nil
}