package redis

import (
	"context"
	"fmt"
	
	"github.com/redis/go-redis/v9"
	"github.com/zacksfF/PubSubGo/internal/config"
)

type Client struct {
	client redis.UniversalClient
	config *config.RedisConfig
}

func NewClient(cfg *config.RedisConfig) (*Client, error) {
	var client redis.UniversalClient
	
	if cfg.EnableCluster {
		client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:              cfg.Addresses,
			Password:           cfg.Password,
			MaxRetries:         cfg.MaxRetries,
			PoolSize:           cfg.PoolSize,
			MinIdleConns:       cfg.MinIdleConns,
			DialTimeout:        cfg.DialTimeout,
			ReadTimeout:        cfg.ReadTimeout,
			WriteTimeout:       cfg.WriteTimeout,
			PoolTimeout:        cfg.PoolTimeout,
			ConnMaxIdleTime:    cfg.IdleTimeout,
		})
	} else if len(cfg.Addresses) > 1 {
		client = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:         "master",
			SentinelAddrs:      cfg.Addresses,
			Password:           cfg.Password,
			DB:                 cfg.DB,
			MaxRetries:         cfg.MaxRetries,
			PoolSize:           cfg.PoolSize,
			MinIdleConns:       cfg.MinIdleConns,
			DialTimeout:        cfg.DialTimeout,
			ReadTimeout:        cfg.ReadTimeout,
			WriteTimeout:       cfg.WriteTimeout,
			PoolTimeout:        cfg.PoolTimeout,
			ConnMaxIdleTime:    cfg.IdleTimeout,
		})
	} else {
		client = redis.NewClient(&redis.Options{
			Addr:               cfg.Addresses[0],
			Password:           cfg.Password,
			DB:                 cfg.DB,
			MaxRetries:         cfg.MaxRetries,
			PoolSize:           cfg.PoolSize,
			MinIdleConns:       cfg.MinIdleConns,
			DialTimeout:        cfg.DialTimeout,
			ReadTimeout:        cfg.ReadTimeout,
			WriteTimeout:       cfg.WriteTimeout,
			PoolTimeout:        cfg.PoolTimeout,
			ConnMaxIdleTime:    cfg.IdleTimeout,
		})
	}
	
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	
	return &Client{
		client: client,
		config: cfg,
	}, nil
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) GetClient() redis.UniversalClient {
	return c.client
}