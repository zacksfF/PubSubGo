package topic

import (
	"sync"
	"time"
)

type Topic struct {
	mu            sync.RWMutex
	Name          string                 `json:"name"`
	Partitions    int32                  `json:"partitions"`
	Replication   int32                  `json:"replication"`
	RetentionTime time.Duration          `json:"retention_time"`
	MaxSize       int64                  `json:"max_size"`
	Config        map[string]interface{} `json:"config"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	
	// Statistics
	MessageCount  int64                  `json:"message_count"`
	BytesIn       int64                  `json:"bytes_in"`
	BytesOut      int64                  `json:"bytes_out"`
}

func NewTopic(name string, partitions int32) *Topic {
	return &Topic{
		Name:          name,
		Partitions:    partitions,
		Replication:   1,
		RetentionTime: 24 * time.Hour,
		MaxSize:       1 << 30, // 1GB default
		Config:        make(map[string]interface{}),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

func (t *Topic) IncrementMessages(count int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.MessageCount += count
	t.UpdatedAt = time.Now()
}

func (t *Topic) AddBytesIn(bytes int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.BytesIn += bytes
}

func (t *Topic) AddBytesOut(bytes int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.BytesOut += bytes
}

func (t *Topic) GetStats() (messages, bytesIn, bytesOut int64) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.MessageCount, t.BytesIn, t.BytesOut
}