package acknowledgment

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/sirupsen/logrus"
	"github.com/zacksfF/PubSubGo/internal/config"
	"github.com/zacksfF/PubSubGo/internal/core/message"
)

// MockMessageRepository for testing
type MockMessageRepository struct {
	mock.Mock
}

func (m *MockMessageRepository) Store(ctx context.Context, msg *message.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockMessageRepository) Get(ctx context.Context, messageID string) (*message.Message, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*message.Message), args.Error(1)
}

func (m *MockMessageRepository) Delete(ctx context.Context, messageID string) error {
	args := m.Called(ctx, messageID)
	return args.Error(0)
}

func (m *MockMessageRepository) StoreBatch(ctx context.Context, messages []*message.Message) error {
	args := m.Called(ctx, messages)
	return args.Error(0)
}

func (m *MockMessageRepository) GetBatch(ctx context.Context, messageIDs []string) ([]*message.Message, error) {
	args := m.Called(ctx, messageIDs)
	return args.Get(0).([]*message.Message), args.Error(1)
}

func (m *MockMessageRepository) GetByTopic(ctx context.Context, topic string, limit int, offset int64) ([]*message.Message, error) {
	args := m.Called(ctx, topic, limit, offset)
	return args.Get(0).([]*message.Message), args.Error(1)
}

func (m *MockMessageRepository) GetByTopicPartition(ctx context.Context, topic string, partition int32, offset int64, limit int) ([]*message.Message, error) {
	args := m.Called(ctx, topic, partition, offset, limit)
	return args.Get(0).([]*message.Message), args.Error(1)
}

func (m *MockMessageRepository) CountByTopic(ctx context.Context, topic string) (int64, error) {
	args := m.Called(ctx, topic)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockMessageRepository) GetForConsumerGroup(ctx context.Context, topic, consumerGroup string, limit int) ([]*message.Message, error) {
	args := m.Called(ctx, topic, consumerGroup, limit)
	return args.Get(0).([]*message.Message), args.Error(1)
}

func (m *MockMessageRepository) MarkDelivered(ctx context.Context, messageID, consumerID string) error {
	args := m.Called(ctx, messageID, consumerID)
	return args.Error(0)
}

func (m *MockMessageRepository) Acknowledge(ctx context.Context, ack *message.Acknowledgment) error {
	args := m.Called(ctx, ack)
	return args.Error(0)
}

func (m *MockMessageRepository) GetUnacknowledged(ctx context.Context, topic string, deadline time.Duration) ([]*message.Message, error) {
	args := m.Called(ctx, topic, deadline)
	return args.Get(0).([]*message.Message), args.Error(1)
}

func (m *MockMessageRepository) GetFailedMessages(ctx context.Context, topic string, maxRetries int) ([]*message.Message, error) {
	args := m.Called(ctx, topic, maxRetries)
	return args.Get(0).([]*message.Message), args.Error(1)
}

func (m *MockMessageRepository) MoveToDLQ(ctx context.Context, messageID, dlqTopic string) error {
	args := m.Called(ctx, messageID, dlqTopic)
	return args.Error(0)
}

func (m *MockMessageRepository) DeleteExpired(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockMessageRepository) DeleteOlderThan(ctx context.Context, topic string, before time.Time) (int64, error) {
	args := m.Called(ctx, topic, before)
	return args.Get(0).(int64), args.Error(1)
}

// Test setup helper
func setupAckTestService() (*service, *MockMessageRepository) {
	messageRepo := &MockMessageRepository{}
	
	cfg := &config.BrokerConfig{
		DefaultAckDeadline: 30 * time.Second,
		MaxRetries:         3,
		RetryBackoff:       time.Second,
		DLQPrefix:         "dlq-",
		EnableDLQ:         true,
	}
	
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce noise in tests
	
	// Create service without starting background workers for tests
	svc := &service{
		messageRepo:   messageRepo,
		config:        cfg,
		logger:        logger,
		pendingAcks:   make(map[string]*PendingAcknowledgment),
		workers:       make(map[string]context.CancelFunc),
		retryStrategy: NewExponentialBackoffRetry(cfg),
		dlqStrategy:   NewDefaultDLQStrategy(cfg),
		stats: &ServiceStats{
			LastReset: time.Now(),
		},
	}
	
	return svc, messageRepo
}

func TestProcessAcknowledgment_Success(t *testing.T) {
	svc, messageRepo := setupAckTestService()
	ctx := context.Background()
	
	// Create test message
	now := time.Now()
	testMessage := &message.Message{
		ID:          "msg-1",
		Topic:       "test-topic",
		ConsumerID:  "consumer-1",
		Status:      message.StatusDelivered,
		DeliveredAt: &now,
		Headers:     make(map[string]string),
	}
	
	// Mock message retrieval
	messageRepo.On("Get", ctx, "msg-1").Return(testMessage, nil)
	messageRepo.On("Store", ctx, mock.AnythingOfType("*message.Message")).Return(nil)
	messageRepo.On("Acknowledge", ctx, mock.AnythingOfType("*message.Acknowledgment")).Return(nil)
	
	// Test request
	req := &AcknowledgmentRequest{
		MessageID:      "msg-1",
		ConsumerID:     "consumer-1",
		Success:        true,
		ProcessingTime: 100 * time.Millisecond,
	}
	
	// Execute
	resp, err := svc.ProcessAcknowledgment(ctx, req)
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "msg-1", resp.MessageID)
	assert.Equal(t, "acknowledged", resp.Status)
	assert.Equal(t, 0, resp.RetryCount)
	
	// Verify message status was updated
	assert.Equal(t, message.StatusAcknowledged, testMessage.Status)
	assert.NotNil(t, testMessage.AckedAt)
	
	// Verify mocks
	messageRepo.AssertExpectations(t)
}

func TestProcessAcknowledgment_Failed_ShouldRetry(t *testing.T) {
	svc, messageRepo := setupAckTestService()
	ctx := context.Background()
	
	// Create test message with low retry count
	now := time.Now()
	testMessage := &message.Message{
		ID:          "msg-1",
		Topic:       "test-topic",
		ConsumerID:  "consumer-1",
		Status:      message.StatusDelivered,
		DeliveredAt: &now,
		RetryCount:  1,
		MaxRetries:  3,
		Headers:     make(map[string]string),
	}
	
	// Mock message retrieval and storage
	messageRepo.On("Get", ctx, "msg-1").Return(testMessage, nil)
	messageRepo.On("Store", ctx, mock.AnythingOfType("*message.Message")).Return(nil)
	
	// Test request - failed acknowledgment
	req := &AcknowledgmentRequest{
		MessageID:  "msg-1",
		ConsumerID: "consumer-1",
		Success:    false,
		Metadata:   map[string]string{"reason": "processing error"},
	}
	
	// Execute
	resp, err := svc.ProcessAcknowledgment(ctx, req)
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "msg-1", resp.MessageID)
	assert.Equal(t, "requeued", resp.Status)
	assert.Equal(t, 2, resp.RetryCount) // Should increment
	assert.NotNil(t, resp.NextRetryAt)
	
	// Verify message was requeued
	assert.Equal(t, message.StatusPending, testMessage.Status)
	assert.Equal(t, 2, testMessage.RetryCount)
	assert.Equal(t, "", testMessage.ConsumerID) // Should be cleared
	
	// Verify mocks
	messageRepo.AssertExpectations(t)
}

func TestProcessNegativeAcknowledgment_MoveToDLQ(t *testing.T) {
	svc, messageRepo := setupAckTestService()
	ctx := context.Background()
	
	// Create test message that has exceeded retry limit
	now := time.Now()
	testMessage := &message.Message{
		ID:          "msg-1",
		Topic:       "test-topic",
		ConsumerID:  "consumer-1",
		Status:      message.StatusDelivered,
		DeliveredAt: &now,
		RetryCount:  3, // At retry limit
		MaxRetries:  3,
		Headers:     make(map[string]string),
	}
	
	// Mock message retrieval and storage
	messageRepo.On("Get", ctx, "msg-1").Return(testMessage, nil)
	messageRepo.On("Store", ctx, mock.AnythingOfType("*message.Message")).Return(nil).Times(2) // Original + DLQ message
	
	// Test request
	req := &NegativeAcknowledgmentRequest{
		MessageID:  "msg-1",
		ConsumerID: "consumer-1",
		Reason:     "permanent failure",
		Retry:      false,
	}
	
	// Execute
	resp, err := svc.ProcessNegativeAcknowledgment(ctx, req)
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "msg-1", resp.MessageID)
	assert.Equal(t, "dlq", resp.Status)
	assert.NotEmpty(t, resp.DLQTopic)
	assert.Contains(t, resp.DLQTopic, "dlq-") // Should have DLQ prefix
	
	// Verify message was moved to DLQ
	assert.Equal(t, message.StatusDLQ, testMessage.Status)
	assert.Equal(t, "", testMessage.ConsumerID) // Should be cleared
	
	// Verify mocks
	messageRepo.AssertExpectations(t)
}

func TestCheckExpiredAcknowledgments(t *testing.T) {
	svc, messageRepo := setupAckTestService()
	ctx := context.Background()
	
	// Create expired messages
	oldTime := time.Now().Add(-1 * time.Hour) // 1 hour ago
	expiredMessages := []*message.Message{
		{
			ID:          "msg-1",
			Topic:       "test-topic",
			ConsumerID:  "consumer-1",
			Status:      message.StatusDelivered,
			DeliveredAt: &oldTime,
			RetryCount:  1,
			MaxRetries:  3,
			Headers:     make(map[string]string),
		},
		{
			ID:          "msg-2",
			Topic:       "test-topic",
			ConsumerID:  "consumer-2",
			Status:      message.StatusDelivered,
			DeliveredAt: &oldTime,
			RetryCount:  3, // At retry limit
			MaxRetries:  3,
			Headers:     make(map[string]string),
		},
	}
	
	// Mock getting unacknowledged messages
	messageRepo.On("GetUnacknowledged", ctx, "", svc.config.DefaultAckDeadline).Return(expiredMessages, nil)
	
	// Mock message updates for requeue and DLQ
	messageRepo.On("Store", ctx, mock.AnythingOfType("*message.Message")).Return(nil).Times(3) // 2 updates + 1 DLQ message
	
	// Execute
	report, err := svc.CheckExpiredAcknowledgments(ctx)
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 2, report.ExpiredCount)
	assert.Equal(t, 1, report.RequeuedCount)
	assert.Equal(t, 1, report.DLQCount)
	assert.Len(t, report.ExpiredMessages, 2)
	
	// Verify expired message actions
	assert.Equal(t, "requeued", report.ExpiredMessages[0].Action)
	assert.Equal(t, "dlq", report.ExpiredMessages[1].Action)
	
	// Verify mocks
	messageRepo.AssertExpectations(t)
}

func TestGetAcknowledgmentStats(t *testing.T) {
	svc, _ := setupAckTestService()
	ctx := context.Background()
	
	// Update some stats manually for testing
	svc.updateStats("acknowledged", 100*time.Millisecond)
	svc.updateStats("acknowledged", 200*time.Millisecond)
	svc.updateStats("nacked", 0)
	svc.updateStats("requeued", 0)
	svc.updateStats("dlq", 0)
	
	// Execute
	stats, err := svc.GetAcknowledgmentStats(ctx, nil)
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(2), stats.TotalAcknowledged)
	assert.Equal(t, int64(1), stats.TotalNacked)
	assert.Equal(t, int64(1), stats.TotalRequeued)
	assert.Equal(t, int64(1), stats.TotalDLQ)
	assert.Equal(t, 150*time.Millisecond, stats.AvgProcessingTime) // (100+200)/2
	assert.Equal(t, float64(2)/float64(3), stats.AckRate) // 2 acked out of 3 total (acked+nacked)
	assert.NotNil(t, stats.TopicStats)
}

func TestMoveToDLQ(t *testing.T) {
	svc, messageRepo := setupAckTestService()
	ctx := context.Background()
	
	// Create test message
	testMessage := &message.Message{
		ID:         "msg-1",
		Topic:      "test-topic",
		RetryCount: 3,
		Headers:    make(map[string]string),
	}
	
	// Mock message retrieval and storage
	messageRepo.On("Get", ctx, "msg-1").Return(testMessage, nil)
	messageRepo.On("Store", ctx, mock.AnythingOfType("*message.Message")).Return(nil).Times(2) // Original + DLQ message
	
	// Test request
	req := &DLQRequest{
		MessageID: "msg-1",
		Reason:    "max retries exceeded",
	}
	
	// Execute
	err := svc.MoveToDLQ(ctx, req)
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, message.StatusDLQ, testMessage.Status)
	assert.Equal(t, "", testMessage.ConsumerID) // Should be cleared
	assert.Contains(t, testMessage.Headers, "dlq_moved_at")
	assert.Contains(t, testMessage.Headers, "dlq_reason")
	
	// Verify mocks
	messageRepo.AssertExpectations(t)
}

func TestValidateRequests(t *testing.T) {
	svc, _ := setupAckTestService()
	
	tests := []struct {
		name    string
		request interface{}
		fn      func() error
	}{
		{
			name: "empty message ID in ack",
			request: &AcknowledgmentRequest{
				MessageID:  "",
				ConsumerID: "consumer-1",
			},
			fn: func() error {
				return svc.validateAckRequest(&AcknowledgmentRequest{
					MessageID:  "",
					ConsumerID: "consumer-1",
				})
			},
		},
		{
			name: "empty consumer ID in ack",
			request: &AcknowledgmentRequest{
				MessageID:  "msg-1",
				ConsumerID: "",
			},
			fn: func() error {
				return svc.validateAckRequest(&AcknowledgmentRequest{
					MessageID:  "msg-1",
					ConsumerID: "",
				})
			},
		},
		{
			name: "empty message ID in nack",
			request: &NegativeAcknowledgmentRequest{
				MessageID:  "",
				ConsumerID: "consumer-1",
			},
			fn: func() error {
				return svc.validateNackRequest(&NegativeAcknowledgmentRequest{
					MessageID:  "",
					ConsumerID: "consumer-1",
				})
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			assert.Error(t, err)
		})
	}
}

func TestRetryStrategies(t *testing.T) {
	cfg := &config.BrokerConfig{
		RetryBackoff: time.Second,
		MaxRetries:   3,
	}
	
	t.Run("ExponentialBackoffRetry", func(t *testing.T) {
		strategy := NewExponentialBackoffRetry(cfg)
		
		msg := &message.Message{MaxRetries: 3}
		
		// Test retry decisions
		assert.True(t, strategy.ShouldRetry(msg, 1))
		assert.True(t, strategy.ShouldRetry(msg, 2))
		assert.True(t, strategy.ShouldRetry(msg, 3))
		assert.False(t, strategy.ShouldRetry(msg, 4))
		
		// Test delay calculation
		delay1 := strategy.GetRetryDelay(msg, 1)
		delay2 := strategy.GetRetryDelay(msg, 2)
		assert.True(t, delay2 > delay1) // Should increase exponentially
		
		// Test max retries
		assert.Equal(t, 3, strategy.GetMaxRetries(msg))
	})
	
	t.Run("LinearBackoffRetry", func(t *testing.T) {
		strategy := NewLinearBackoffRetry(cfg)
		
		msg := &message.Message{MaxRetries: 3}
		
		// Test retry decisions
		assert.True(t, strategy.ShouldRetry(msg, 1))
		assert.False(t, strategy.ShouldRetry(msg, 4))
		
		// Test delay calculation
		delay1 := strategy.GetRetryDelay(msg, 1)
		delay2 := strategy.GetRetryDelay(msg, 2)
		assert.Equal(t, time.Second, delay2-delay1) // Should increase linearly
	})
	
	t.Run("FixedDelayRetry", func(t *testing.T) {
		strategy := NewFixedDelayRetry(cfg)
		
		msg := &message.Message{MaxRetries: 3}
		
		// Test delay is always the same
		delay1 := strategy.GetRetryDelay(msg, 1)
		delay2 := strategy.GetRetryDelay(msg, 2)
		assert.Equal(t, delay1, delay2)
		assert.Equal(t, time.Second, delay1)
	})
}

func TestDLQStrategies(t *testing.T) {
	cfg := &config.BrokerConfig{
		DLQPrefix:  "dlq-",
		EnableDLQ:  true,
		MaxRetries: 3,
	}
	
	t.Run("DefaultDLQStrategy", func(t *testing.T) {
		strategy := NewDefaultDLQStrategy(cfg)
		
		// Test message that should move to DLQ
		msg := &message.Message{
			Topic:      "test-topic",
			RetryCount: 3,
		}
		assert.True(t, strategy.ShouldMoveToDLQ(msg))
		
		// Test message that shouldn't move to DLQ
		msg.RetryCount = 1
		assert.False(t, strategy.ShouldMoveToDLQ(msg))
		
		// Test DLQ topic name generation
		dlqTopic := strategy.GetDLQTopic("test-topic")
		assert.Equal(t, "dlq-test-topic", dlqTopic)
		
		// Test message enrichment
		enrichedMsg := strategy.EnrichDLQMessage(msg, "test reason")
		assert.Equal(t, "dlq-test-topic", enrichedMsg.Topic)
		assert.Equal(t, message.StatusDLQ, enrichedMsg.Status)
		assert.Equal(t, message.PriorityHigh, enrichedMsg.Priority)
		assert.Contains(t, enrichedMsg.Headers, "dlq_reason")
		assert.Contains(t, enrichedMsg.Headers, "original_topic")
	})
	
	t.Run("TopicBasedDLQStrategy", func(t *testing.T) {
		defaultStrategy := NewDefaultDLQStrategy(cfg)
		strategy := NewTopicBasedDLQStrategy(defaultStrategy)
		
		// Add topic-specific strategy
		customConfig := config.BrokerConfig{
			DLQPrefix:  "custom-dlq-",
			EnableDLQ:  true,
			MaxRetries: 5,
		}
		customStrategy := NewDefaultDLQStrategy(&customConfig)
		strategy.AddTopicStrategy("special-topic", customStrategy)
		
		// Test default behavior
		msg := &message.Message{Topic: "regular-topic", RetryCount: 3}
		assert.True(t, strategy.ShouldMoveToDLQ(msg))
		assert.Equal(t, "dlq-regular-topic", strategy.GetDLQTopic("regular-topic"))
		
		// Test topic-specific behavior
		specialMsg := &message.Message{Topic: "special-topic", RetryCount: 3}
		assert.False(t, strategy.ShouldMoveToDLQ(specialMsg)) // Custom strategy has MaxRetries=5
		assert.Equal(t, "custom-dlq-special-topic", strategy.GetDLQTopic("special-topic"))
	})
}

func TestWorkerManagement(t *testing.T) {
	svc, _ := setupAckTestService()
	ctx := context.Background()
	
	// Test starting workers
	err := svc.StartAcknowledgmentWorker(ctx)
	assert.NoError(t, err)
	
	// Verify workers are running
	svc.workersMutex.RLock()
	workerCount := len(svc.workers)
	svc.workersMutex.RUnlock()
	assert.Equal(t, 3, workerCount) // expiration_checker, retry_processor, pending_cleaner
	
	// Test stopping workers
	err = svc.StopAcknowledgmentWorker()
	assert.NoError(t, err)
	
	// Verify workers are stopped
	svc.workersMutex.RLock()
	workerCount = len(svc.workers)
	svc.workersMutex.RUnlock()
	assert.Equal(t, 0, workerCount)
}