package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	
	"github.com/redis/go-redis/v9"
	"github.com/zacksfF/PubSubGo/internal/core/message"
	"github.com/zacksfF/PubSubGo/internal/ports/repository"
)

type MessageRepository struct {
	client *Client
}

func NewMessageRepository(client *Client) repository.MessageRepository {
	return &MessageRepository{
		client: client,
	}
}

func (r *MessageRepository) Store(ctx context.Context, msg *message.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	
	pipe := r.client.GetClient().Pipeline()
	
	// Store message by ID
	messageKey := fmt.Sprintf("msg:%s", msg.ID)
	pipe.Set(ctx, messageKey, data, 0)
	
	// Add to topic queue
	topicKey := fmt.Sprintf("topic:%s:messages", msg.Topic)
	pipe.ZAdd(ctx, topicKey, redis.Z{
		Score:  float64(msg.CreatedAt.UnixNano()),
		Member: msg.ID,
	})
	
	// Add to partition if specified
	if msg.Partition >= 0 {
		partitionKey := fmt.Sprintf("topic:%s:partition:%d", msg.Topic, msg.Partition)
		pipe.RPush(ctx, partitionKey, msg.ID)
	}
	
	// Set expiration if specified
	if msg.ExpiresAt != nil {
		ttl := time.Until(*msg.ExpiresAt)
		if ttl > 0 {
			pipe.Expire(ctx, messageKey, ttl)
		}
	}
	
	// Update topic stats
	statsKey := fmt.Sprintf("topic:%s:stats", msg.Topic)
	pipe.HIncrBy(ctx, statsKey, "message_count", 1)
	pipe.HIncrBy(ctx, statsKey, "bytes_in", int64(len(msg.Payload)))
	
	_, err = pipe.Exec(ctx)
	return err
}

func (r *MessageRepository) Get(ctx context.Context, messageID string) (*message.Message, error) {
	messageKey := fmt.Sprintf("msg:%s", messageID)
	data, err := r.client.GetClient().Get(ctx, messageKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("message not found: %s", messageID)
		}
		return nil, err
	}
	
	var msg message.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	
	return &msg, nil
}

func (r *MessageRepository) Delete(ctx context.Context, messageID string) error {
	messageKey := fmt.Sprintf("msg:%s", messageID)
	return r.client.GetClient().Del(ctx, messageKey).Err()
}

func (r *MessageRepository) StoreBatch(ctx context.Context, messages []*message.Message) error {
	pipe := r.client.GetClient().Pipeline()
	
	for _, msg := range messages {
		data, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal message %s: %w", msg.ID, err)
		}
		
		messageKey := fmt.Sprintf("msg:%s", msg.ID)
		pipe.Set(ctx, messageKey, data, 0)
		
		topicKey := fmt.Sprintf("topic:%s:messages", msg.Topic)
		pipe.ZAdd(ctx, topicKey, redis.Z{
			Score:  float64(msg.CreatedAt.UnixNano()),
			Member: msg.ID,
		})
		
		if msg.ExpiresAt != nil {
			ttl := time.Until(*msg.ExpiresAt)
			if ttl > 0 {
				pipe.Expire(ctx, messageKey, ttl)
			}
		}
	}
	
	_, err := pipe.Exec(ctx)
	return err
}

func (r *MessageRepository) GetBatch(ctx context.Context, messageIDs []string) ([]*message.Message, error) {
	pipe := r.client.GetClient().Pipeline()
	
	for _, id := range messageIDs {
		messageKey := fmt.Sprintf("msg:%s", id)
		pipe.Get(ctx, messageKey)
	}
	
	cmds, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}
	
	messages := make([]*message.Message, 0, len(messageIDs))
	for _, cmd := range cmds {
		strCmd := cmd.(*redis.StringCmd)
		data, err := strCmd.Bytes()
		if err != nil {
			continue // Skip missing messages
		}
		
		var msg message.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		messages = append(messages, &msg)
	}
	
	return messages, nil
}

func (r *MessageRepository) GetByTopic(ctx context.Context, topic string, limit int, offset int64) ([]*message.Message, error) {
	topicKey := fmt.Sprintf("topic:%s:messages", topic)
	
	// Get message IDs from sorted set
	ids, err := r.client.GetClient().ZRange(ctx, topicKey, offset, offset+int64(limit)-1).Result()
	if err != nil {
		return nil, err
	}
	
	if len(ids) == 0 {
		return []*message.Message{}, nil
	}
	
	return r.GetBatch(ctx, ids)
}

func (r *MessageRepository) GetByTopicPartition(ctx context.Context, topic string, partition int32, offset int64, limit int) ([]*message.Message, error) {
	partitionKey := fmt.Sprintf("topic:%s:partition:%d", topic, partition)
	
	ids, err := r.client.GetClient().LRange(ctx, partitionKey, offset, offset+int64(limit)-1).Result()
	if err != nil {
		return nil, err
	}
	
	if len(ids) == 0 {
		return []*message.Message{}, nil
	}
	
	return r.GetBatch(ctx, ids)
}

func (r *MessageRepository) CountByTopic(ctx context.Context, topic string) (int64, error) {
	topicKey := fmt.Sprintf("topic:%s:messages", topic)
	return r.client.GetClient().ZCard(ctx, topicKey).Result()
}

func (r *MessageRepository) GetForConsumerGroup(ctx context.Context, topic, consumerGroup string, limit int) ([]*message.Message, error) {
	// Use Redis streams for consumer groups
	streamKey := fmt.Sprintf("stream:%s", topic)
	groupKey := consumerGroup
	consumerKey := fmt.Sprintf("%s-%d", consumerGroup, time.Now().UnixNano())
	
	// Try to create consumer group (ignore error if already exists)
	_ = r.client.GetClient().XGroupCreateMkStream(ctx, streamKey, groupKey, "0").Err()
	
	// Read messages for consumer group
	streams, err := r.client.GetClient().XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    groupKey,
		Consumer: consumerKey,
		Streams:  []string{streamKey, ">"},
		Count:    int64(limit),
		Block:    0,
		NoAck:    false,
	}).Result()
	
	if err != nil {
		return nil, err
	}
	
	messages := make([]*message.Message, 0)
	for _, stream := range streams {
		for _, xmsg := range stream.Messages {
			if msgID, ok := xmsg.Values["message_id"].(string); ok {
				msg, err := r.Get(ctx, msgID)
				if err == nil {
					messages = append(messages, msg)
				}
			}
		}
	}
	
	return messages, nil
}

func (r *MessageRepository) MarkDelivered(ctx context.Context, messageID, consumerID string) error {
	key := fmt.Sprintf("msg:%s:delivered", messageID)
	return r.client.GetClient().Set(ctx, key, consumerID, 24*time.Hour).Err()
}

func (r *MessageRepository) Acknowledge(ctx context.Context, ack *message.Acknowledgment) error {
	pipe := r.client.GetClient().Pipeline()
	
	// Update message acknowledgment status
	msg, err := r.Get(ctx, ack.MessageID)
	if err != nil {
		return err
	}
	
	now := time.Now()
	if ack.Success {
		msg.Status = message.StatusAcknowledged
		msg.AckedAt = &now
	} else {
		msg.Status = message.StatusFailed
		msg.RetryCount++
	}
	
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	
	messageKey := fmt.Sprintf("msg:%s", ack.MessageID)
	pipe.Set(ctx, messageKey, data, 0)
	
	// Store acknowledgment
	ackKey := fmt.Sprintf("ack:%s", ack.MessageID)
	ackData, _ := json.Marshal(ack)
	pipe.Set(ctx, ackKey, ackData, 24*time.Hour)
	
	// Update consumer group stats
	if ack.ConsumerGroup != "" {
		statsKey := fmt.Sprintf("consumer:%s:stats", ack.ConsumerGroup)
		if ack.Success {
			pipe.HIncrBy(ctx, statsKey, "acked", 1)
		} else {
			pipe.HIncrBy(ctx, statsKey, "nacked", 1)
		}
	}
	
	_, err = pipe.Exec(ctx)
	return err
}

func (r *MessageRepository) GetUnacknowledged(ctx context.Context, topic string, deadline time.Duration) ([]*message.Message, error) {
	// Get all messages from topic
	messages, err := r.GetByTopic(ctx, topic, 1000, 0)
	if err != nil {
		return nil, err
	}
	
	unacked := make([]*message.Message, 0)
	for _, msg := range messages {
		if msg.Status == message.StatusDelivered && msg.DeliveredAt != nil {
			if time.Since(*msg.DeliveredAt) > deadline {
				unacked = append(unacked, msg)
			}
		}
	}
	
	return unacked, nil
}

func (r *MessageRepository) GetFailedMessages(ctx context.Context, topic string, maxRetries int) ([]*message.Message, error) {
	messages, err := r.GetByTopic(ctx, topic, 1000, 0)
	if err != nil {
		return nil, err
	}
	
	failed := make([]*message.Message, 0)
	for _, msg := range messages {
		if msg.Status == message.StatusFailed && msg.RetryCount >= maxRetries {
			failed = append(failed, msg)
		}
	}
	
	return failed, nil
}

func (r *MessageRepository) MoveToDLQ(ctx context.Context, messageID, dlqTopic string) error {
	msg, err := r.Get(ctx, messageID)
	if err != nil {
		return err
	}
	
	// Update message topic and status
	msg.Topic = dlqTopic
	msg.Status = message.StatusDLQ
	
	// Store in DLQ topic
	return r.Store(ctx, msg)
}

func (r *MessageRepository) DeleteExpired(ctx context.Context) (int64, error) {
	// This would typically be handled by Redis TTL, but we can scan for expired messages
	// Implementation would involve scanning all messages and checking expiry
	return 0, nil
}

func (r *MessageRepository) DeleteOlderThan(ctx context.Context, topic string, before time.Time) (int64, error) {
	topicKey := fmt.Sprintf("topic:%s:messages", topic)
	
	// Remove from sorted set
	count, err := r.client.GetClient().ZRemRangeByScore(ctx, topicKey, 
		"-inf", 
		fmt.Sprintf("%d", before.UnixNano())).Result()
	
	if err != nil {
		return 0, err
	}
	
	return count, nil
}