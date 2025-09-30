package broker

import (
	"context"
	
	"github.com/zacksfF/PubSubGo/internal/core/message"
	"github.com/zacksfF/PubSubGo/internal/core/subscription"
)

type MessageBroker interface {
	// Publishing
	Publish(ctx context.Context, topic string, msg *message.Message) error
	PublishBatch(ctx context.Context, topic string, messages []*message.Message) error
	
	// Subscribing
	Subscribe(ctx context.Context, sub *subscription.Subscription) (<-chan *message.Message, error)
	Unsubscribe(ctx context.Context, subscriptionID string) error
	
	// Pulling
	Pull(ctx context.Context, topic string, limit int) ([]*message.Message, error)
	PullWithConsumerGroup(ctx context.Context, topic, consumerGroup string, limit int) ([]*message.Message, error)
	
	// Acknowledgments
	Ack(ctx context.Context, ack *message.Acknowledgment) error
	Nack(ctx context.Context, messageID string, reason string) error
	
	// Management
	CreateTopic(ctx context.Context, name string, partitions int32) error
	DeleteTopic(ctx context.Context, name string) error
	ListTopics(ctx context.Context) ([]string, error)
	
	// Health
	Ping(ctx context.Context) error
	GetStats(ctx context.Context) (map[string]interface{}, error)
}