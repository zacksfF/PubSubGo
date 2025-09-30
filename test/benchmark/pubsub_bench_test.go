package benchmark

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zacksfF/PubSubGo/internal/core/message"
)

// BenchmarkMessageCreation benchmarks message creation
func BenchmarkMessageCreation(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = &message.Message{
			ID:       uuid.New().String(),
			Topic:    "benchmark-topic",
			Key:      []byte("benchmark-key"),
			Payload:  []byte("benchmark payload data"),
			Headers:  map[string]string{"bench": "mark"},
			Priority: message.PriorityNormal,
			Status:   message.StatusPending,
		}
	}
}

// BenchmarkMessageSerialization benchmarks message serialization/deserialization
func BenchmarkMessageSerialization(b *testing.B) {
	msg := &message.Message{
		ID:       uuid.New().String(),
		Topic:    "benchmark-topic",
		Key:      []byte("benchmark-key"),
		Payload:  []byte("benchmark payload data"),
		Headers:  map[string]string{"bench": "mark", "test": "value"},
		Priority: message.PriorityHigh,
		Status:   message.StatusPending,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate serialization and deserialization
		data := msg.ID + msg.Topic + string(msg.Key) + string(msg.Payload)
		_ = len(data)
	}
}

// BenchmarkConcurrentPublish benchmarks concurrent message publishing
func BenchmarkConcurrentPublish(b *testing.B) {
	benchmarks := []struct {
		name       string
		publishers int
	}{
		{"1-publisher", 1},
		{"10-publishers", 10},
		{"100-publishers", 100},
		{"1000-publishers", 1000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			var counter int64
			var wg sync.WaitGroup

			b.ReportAllocs()
			b.ResetTimer()

			messagesPerPublisher := b.N / bm.publishers
			if messagesPerPublisher == 0 {
				messagesPerPublisher = 1
			}

			for i := 0; i < bm.publishers; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					for j := 0; j < messagesPerPublisher; j++ {
						// Simulate message publishing
						atomic.AddInt64(&counter, 1)
					}
				}(i)
			}

			wg.Wait()
			b.ReportMetric(float64(atomic.LoadInt64(&counter)), "messages")
		})
	}
}

// BenchmarkMessageProcessing benchmarks message processing with different sizes
func BenchmarkMessageProcessing(b *testing.B) {
	sizes := []int{
		100,     // 100 bytes
		1024,    // 1 KB
		10240,   // 10 KB
		102400,  // 100 KB
		1048576, // 1 MB
	}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			payload := make([]byte, size)
			rand.Read(payload)

			msg := &message.Message{
				ID:       uuid.New().String(),
				Topic:    "benchmark-topic",
				Key:      []byte("key"),
				Payload:  payload,
				Priority: message.PriorityNormal,
				Status:   message.StatusPending,
			}

			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Simulate message processing
				processMessage(msg)
			}
		})
	}
}

// BenchmarkTopicOperations benchmarks topic-related operations
func BenchmarkTopicOperations(b *testing.B) {
	b.Run("CreateTopic", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			topicName := fmt.Sprintf("topic-%d", i)
			// Simulate topic creation
			_ = topicName
		}
	})

	b.Run("TopicLookup", func(b *testing.B) {
		// Setup: Create topics
		topics := make(map[string]bool)
		for i := 0; i < 1000; i++ {
			topics[fmt.Sprintf("topic-%d", i)] = true
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			topicName := fmt.Sprintf("topic-%d", i%1000)
			_ = topics[topicName]
		}
	})
}

// BenchmarkSubscriptionManagement benchmarks subscription operations
func BenchmarkSubscriptionManagement(b *testing.B) {
	b.Run("Subscribe", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			subID := uuid.New().String()
			// Simulate subscription creation
			_ = subID
		}
	})

	b.Run("Unsubscribe", func(b *testing.B) {
		// Setup: Create subscriptions
		subs := make([]string, 1000)
		for i := range subs {
			subs[i] = uuid.New().String()
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Simulate unsubscribe
			_ = subs[i%len(subs)]
		}
	})
}

// BenchmarkMessageQueue benchmarks queue operations
func BenchmarkMessageQueue(b *testing.B) {
	b.Run("Enqueue", func(b *testing.B) {
		queue := make(chan *message.Message, 10000)
		msg := createBenchmarkMessage()

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			select {
			case queue <- msg:
			default:
				// Queue full, drain one
				<-queue
				queue <- msg
			}
		}
	})

	b.Run("Dequeue", func(b *testing.B) {
		queue := make(chan *message.Message, 10000)
		msg := createBenchmarkMessage()

		// Fill queue
		for i := 0; i < cap(queue); i++ {
			queue <- msg
		}

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			select {
			case <-queue:
				// Refill to maintain queue
				queue <- msg
			default:
			}
		}
	})
}

// BenchmarkAcknowledgment benchmarks acknowledgment processing
func BenchmarkAcknowledgment(b *testing.B) {
	acks := make(map[string]time.Time)
	mu := sync.RWMutex{}

	b.Run("ACK", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			msgID := fmt.Sprintf("msg-%d", i)
			mu.Lock()
			acks[msgID] = time.Now()
			mu.Unlock()
		}
	})

	b.Run("CheckACK", func(b *testing.B) {
		// Pre-populate
		for i := 0; i < 1000; i++ {
			acks[fmt.Sprintf("msg-%d", i)] = time.Now()
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			msgID := fmt.Sprintf("msg-%d", i%1000)
			mu.RLock()
			_ = acks[msgID]
			mu.RUnlock()
		}
	})
}

// BenchmarkCompression benchmarks different compression algorithms
func BenchmarkCompression(b *testing.B) {
	data := make([]byte, 1024)
	rand.Read(data)

	compressionTypes := []string{"none", "snappy", "gzip", "lz4"}

	for _, compType := range compressionTypes {
		b.Run(compType, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Simulate compression (actual implementation would use real libraries)
				compressed := simulateCompression(data, compType)
				_ = compressed
			}
		})
	}
}

// BenchmarkPartitioning benchmarks partition selection
func BenchmarkPartitioning(b *testing.B) {
	partitionCounts := []int{1, 10, 100, 1000}

	for _, partitions := range partitionCounts {
		b.Run(fmt.Sprintf("%d-partitions", partitions), func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("key-%d", i)
				// Simple hash-based partition selection
				hash := 0
				for _, c := range key {
					hash = hash*31 + int(c)
				}
				partition := hash % partitions
				_ = partition
			}
		})
	}
}

// Helper functions

func processMessage(msg *message.Message) {
	// Simulate message processing
	_ = len(msg.Payload)
	_ = msg.Priority
	_ = msg.Status
}

func createBenchmarkMessage() *message.Message {
	return &message.Message{
		ID:       uuid.New().String(),
		Topic:    "benchmark",
		Key:      []byte("key"),
		Payload:  []byte("benchmark payload"),
		Priority: message.PriorityNormal,
		Status:   message.StatusPending,
	}
}

func simulateCompression(data []byte, compType string) []byte {
	// Simplified compression simulation
	switch compType {
	case "none":
		return data
	default:
		// Very simple simulation - just return a portion
		if len(data) > 10 {
			return data[:len(data)/2]
		}
		return data
	}
}

// BenchmarkEndToEnd benchmarks the full publish-subscribe cycle
func BenchmarkEndToEnd(b *testing.B) {
	ctx := context.Background()

	b.Run("SingleMessage", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Simulate end-to-end flow
			msg := createBenchmarkMessage()

			// Publish
			publishTime := time.Now()
			_ = msg

			// Subscribe and receive
			receiveTime := time.Now()

			// ACK
			ackTime := time.Now()

			// Report latencies
			_ = receiveTime.Sub(publishTime)
			_ = ackTime.Sub(receiveTime)
		}
	})

	b.Run("BatchMessages", func(b *testing.B) {
		batchSize := 100
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			messages := make([]*message.Message, batchSize)
			for j := range messages {
				messages[j] = createBenchmarkMessage()
			}

			// Simulate batch processing
			_ = ctx
			_ = messages
		}
	})
}
