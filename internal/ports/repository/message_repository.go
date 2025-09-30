package repository

import (
	"context"
	"time"
	
	"github.com/zacksfF/PubSubGo/internal/core/message"
)

type MessageRepository interface {
	// Basic CRUD operations
	Store(ctx context.Context, msg *message.Message) error
	Get(ctx context.Context, messageID string) (*message.Message, error)
	Delete(ctx context.Context, messageID string) error
	
	// Batch operations
	StoreBatch(ctx context.Context, messages []*message.Message) error
	GetBatch(ctx context.Context, messageIDs []string) ([]*message.Message, error)
	
	// Topic operations
	GetByTopic(ctx context.Context, topic string, limit int, offset int64) ([]*message.Message, error)
	GetByTopicPartition(ctx context.Context, topic string, partition int32, offset int64, limit int) ([]*message.Message, error)
	CountByTopic(ctx context.Context, topic string) (int64, error)
	
	// Consumer group operations
	GetForConsumerGroup(ctx context.Context, topic, consumerGroup string, limit int) ([]*message.Message, error)
	MarkDelivered(ctx context.Context, messageID, consumerID string) error
	
	// Acknowledgment operations
	Acknowledge(ctx context.Context, ack *message.Acknowledgment) error
	GetUnacknowledged(ctx context.Context, topic string, deadline time.Duration) ([]*message.Message, error)
	
	// Retry and DLQ operations
	GetFailedMessages(ctx context.Context, topic string, maxRetries int) ([]*message.Message, error)
	MoveToDLQ(ctx context.Context, messageID, dlqTopic string) error
	
	// Maintenance operations
	DeleteExpired(ctx context.Context) (int64, error)
	DeleteOlderThan(ctx context.Context, topic string, before time.Time) (int64, error)
}