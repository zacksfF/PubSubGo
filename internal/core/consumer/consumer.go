package consumer

import (
	"sync"
	"time"
)

type ConsumerGroup struct {
	mu              sync.RWMutex
	Name            string              `json:"name"`
	Topic           string              `json:"topic"`
	Consumers       map[string]*Consumer `json:"consumers"`
	RebalanceMethod string              `json:"rebalance_method"`
	MaxConsumers    int                 `json:"max_consumers"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

type Consumer struct {
	ID              string              `json:"id"`
	GroupID         string              `json:"group_id"`
	Partitions      []int32             `json:"partitions"`
	LastHeartbeat   time.Time           `json:"last_heartbeat"`
	Status          ConsumerStatus      `json:"status"`
	Metadata        map[string]string   `json:"metadata"`
}

type ConsumerStatus int

const (
	ConsumerActive ConsumerStatus = iota
	ConsumerInactive
	ConsumerRebalancing
)

func NewConsumerGroup(name, topic string) *ConsumerGroup {
	return &ConsumerGroup{
		Name:            name,
		Topic:           topic,
		Consumers:       make(map[string]*Consumer),
		RebalanceMethod: "range",
		MaxConsumers:    100,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

func (cg *ConsumerGroup) AddConsumer(consumer *Consumer) bool {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	
	if len(cg.Consumers) >= cg.MaxConsumers {
		return false
	}
	
	cg.Consumers[consumer.ID] = consumer
	cg.UpdatedAt = time.Now()
	return true
}

func (cg *ConsumerGroup) RemoveConsumer(consumerID string) {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	
	delete(cg.Consumers, consumerID)
	cg.UpdatedAt = time.Now()
}

func (cg *ConsumerGroup) GetActiveConsumers() []*Consumer {
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	
	active := make([]*Consumer, 0)
	for _, c := range cg.Consumers {
		if c.Status == ConsumerActive {
			active = append(active, c)
		}
	}
	return active
}

func (cg *ConsumerGroup) NeedsRebalance() bool {
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	
	for _, c := range cg.Consumers {
		if c.Status == ConsumerRebalancing {
			return false // Already rebalancing
		}
		if time.Since(c.LastHeartbeat) > 30*time.Second {
			return true // Dead consumer detected
		}
	}
	return false
}