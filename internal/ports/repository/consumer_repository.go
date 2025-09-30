package repository

import (
	"context"
	
	"github.com/zacksfF/PubSubGo/internal/core/consumer"
)

type ConsumerGroupRepository interface {
	CreateGroup(ctx context.Context, group *consumer.ConsumerGroup) error
	GetGroup(ctx context.Context, name string) (*consumer.ConsumerGroup, error)
	UpdateGroup(ctx context.Context, group *consumer.ConsumerGroup) error
	DeleteGroup(ctx context.Context, name string) error
	ListGroups(ctx context.Context, topic string) ([]*consumer.ConsumerGroup, error)
	
	AddConsumer(ctx context.Context, groupName string, consumer *consumer.Consumer) error
	RemoveConsumer(ctx context.Context, groupName, consumerID string) error
	UpdateConsumerHeartbeat(ctx context.Context, groupName, consumerID string) error
	GetActiveConsumers(ctx context.Context, groupName string) ([]*consumer.Consumer, error)
}