package repository

import (
	"context"
	
	"github.com/zacksfF/PubSubGo/internal/core/topic"
)

type TopicRepository interface {
	Create(ctx context.Context, t *topic.Topic) error
	Get(ctx context.Context, name string) (*topic.Topic, error)
	Update(ctx context.Context, t *topic.Topic) error
	Delete(ctx context.Context, name string) error
	List(ctx context.Context, offset, limit int) ([]*topic.Topic, error)
	Exists(ctx context.Context, name string) (bool, error)
}