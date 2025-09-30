package repository

import (
	"context"

	"github.com/zacksfF/PubSubGo/internal/core/subscription"
)

type SubscriptionRepository interface {
	Create(ctx context.Context, sub *subscription.Subscription) error
	Get(ctx context.Context, id string) (*subscription.Subscription, error)
	Update(ctx context.Context, sub *subscription.Subscription) error
	Delete(ctx context.Context, id string) error
	GetByTopic(ctx context.Context, topic string) ([]*subscription.Subscription, error)
	GetByConsumerGroup(ctx context.Context, consumerGroup string) ([]*subscription.Subscription, error)
	GetActive(ctx context.Context) ([]*subscription.Subscription, error)
	List(ctx context.Context, offset, limit int) ([]*subscription.Subscription, error)
}
