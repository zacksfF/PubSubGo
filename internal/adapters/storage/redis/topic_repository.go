package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zacksfF/PubSubGo/internal/core/topic"
	"github.com/zacksfF/PubSubGo/internal/ports/repository"
)

type topicRepository struct {
	client *Client
}

// NewTopicRepository creates a new Redis-based topic repository
func NewTopicRepository(client *Client) repository.TopicRepository {
	return &topicRepository{
		client: client,
	}
}

func (r *topicRepository) Create(ctx context.Context, t *topic.Topic) error {
	key := fmt.Sprintf("topic:%s", t.Name)
	
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("failed to marshal topic: %w", err)
	}
	
	// Store topic data
	err = r.client.client.Set(ctx, key, data, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to store topic: %w", err)
	}
	
	// Add to topics set
	err = r.client.client.SAdd(ctx, "topics", t.Name).Err()
	if err != nil {
		return fmt.Errorf("failed to add topic to set: %w", err)
	}
	
	return nil
}

func (r *topicRepository) Get(ctx context.Context, name string) (*topic.Topic, error) {
	key := fmt.Sprintf("topic:%s", name)
	
	data, err := r.client.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("topic not found: %s", name)
		}
		return nil, fmt.Errorf("failed to get topic: %w", err)
	}
	
	var t topic.Topic
	if err := json.Unmarshal([]byte(data), &t); err != nil {
		return nil, fmt.Errorf("failed to unmarshal topic: %w", err)
	}
	
	return &t, nil
}

func (r *topicRepository) Update(ctx context.Context, t *topic.Topic) error {
	key := fmt.Sprintf("topic:%s", t.Name)
	
	// Update timestamp
	t.UpdatedAt = time.Now()
	
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("failed to marshal topic: %w", err)
	}
	
	err = r.client.client.Set(ctx, key, data, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to update topic: %w", err)
	}
	
	return nil
}

func (r *topicRepository) Delete(ctx context.Context, name string) error {
	key := fmt.Sprintf("topic:%s", name)
	
	// Remove from topics set
	err := r.client.client.SRem(ctx, "topics", name).Err()
	if err != nil {
		return fmt.Errorf("failed to remove topic from set: %w", err)
	}
	
	// Delete topic data
	err = r.client.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete topic: %w", err)
	}
	
	// Delete all messages for this topic
	// Use SCAN to find all message keys for this topic
	iter := r.client.client.Scan(ctx, 0, fmt.Sprintf("msg:%s:*", name), 0).Iterator()
	for iter.Next(ctx) {
		r.client.client.Del(ctx, iter.Val())
	}
	
	return nil
}

func (r *topicRepository) List(ctx context.Context, offset, limit int) ([]*topic.Topic, error) {
	// Get all topic names
	topicNames, err := r.client.client.SMembers(ctx, "topics").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}
	
	// Apply pagination
	start := offset
	end := offset + limit
	if start > len(topicNames) {
		return []*topic.Topic{}, nil
	}
	if end > len(topicNames) {
		end = len(topicNames)
	}
	
	topics := make([]*topic.Topic, 0, end-start)
	for i := start; i < end; i++ {
		t, err := r.Get(ctx, topicNames[i])
		if err != nil {
			// Skip topics that can't be retrieved
			continue
		}
		topics = append(topics, t)
	}
	
	return topics, nil
}

func (r *topicRepository) Exists(ctx context.Context, name string) (bool, error) {
	key := fmt.Sprintf("topic:%s", name)
	
	exists, err := r.client.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check topic existence: %w", err)
	}
	
	return exists > 0, nil
}

func (r *topicRepository) GetByPartition(ctx context.Context, partition int32) ([]*topic.Topic, error) {
	// Get all topics and filter by partition
	allTopics, err := r.List(ctx, 0, 10000)
	if err != nil {
		return nil, err
	}
	
	topics := make([]*topic.Topic, 0)
	for _, t := range allTopics {
		if t.Partitions > partition {
			topics = append(topics, t)
		}
	}
	
	return topics, nil
}

func (r *topicRepository) IncrementMessageCount(ctx context.Context, name string, delta int64) error {
	key := fmt.Sprintf("topic:stats:%s:messages", name)
	
	err := r.client.client.IncrBy(ctx, key, delta).Err()
	if err != nil {
		return fmt.Errorf("failed to increment message count: %w", err)
	}
	
	return nil
}