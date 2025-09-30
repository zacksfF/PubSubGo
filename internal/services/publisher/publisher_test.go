package publisher

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/sirupsen/logrus"
	"github.com/zacksfF/PubSubGo/internal/config"
	"github.com/zacksfF/PubSubGo/internal/core/message"
	"github.com/zacksfF/PubSubGo/internal/core/topic"
)

// Mock repositories
type MockMessageRepository struct {
	mock.Mock
}

func (m *MockMessageRepository) Store(ctx context.Context, msg *message.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockMessageRepository) Get(ctx context.Context, messageID string) (*message.Message, error) {
	args := m.Called(ctx, messageID)
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

type MockTopicRepository struct {
	mock.Mock
}

func (m *MockTopicRepository) Create(ctx context.Context, t *topic.Topic) error {
	args := m.Called(ctx, t)
	return args.Error(0)
}

func (m *MockTopicRepository) Get(ctx context.Context, name string) (*topic.Topic, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*topic.Topic), args.Error(1)
}

func (m *MockTopicRepository) Update(ctx context.Context, t *topic.Topic) error {
	args := m.Called(ctx, t)
	return args.Error(0)
}

func (m *MockTopicRepository) Delete(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockTopicRepository) List(ctx context.Context, offset, limit int) ([]*topic.Topic, error) {
	args := m.Called(ctx, offset, limit)
	return args.Get(0).([]*topic.Topic), args.Error(1)
}

func (m *MockTopicRepository) Exists(ctx context.Context, name string) (bool, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(bool), args.Error(1)
}

// Test setup helper
func setupTestService() (*service, *MockMessageRepository, *MockTopicRepository) {
	messageRepo := &MockMessageRepository{}
	topicRepo := &MockTopicRepository{}
	
	config := &config.BrokerConfig{
		DefaultPartitions:     4,
		MaxMessageBatchSize:   1000,
		CompressionType:       "snappy",
		CompressionLevel:      6,
		MaxRetries:            3,
	}
	
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce noise in tests
	
	svc := &service{
		messageRepo: messageRepo,
		topicRepo:   topicRepo,
		config:      config,
		logger:      logger,
		compressor:  NewCompressor(config.CompressionType, config.CompressionLevel),
	}
	
	return svc, messageRepo, topicRepo
}

func TestPublish_Success(t *testing.T) {
	svc, messageRepo, topicRepo := setupTestService()
	ctx := context.Background()
	
	// Mock topic exists
	testTopic := topic.NewTopic("test-topic", 4)
	topicRepo.On("Get", ctx, "test-topic").Return(testTopic, nil)
	topicRepo.On("Update", ctx, mock.AnythingOfType("*topic.Topic")).Return(nil)
	
	// Mock message storage
	messageRepo.On("Store", ctx, mock.AnythingOfType("*message.Message")).Return(nil)
	
	// Test request
	req := &PublishRequest{
		Topic:   "test-topic",
		Payload: []byte("Hello, World!"),
		Headers: map[string]string{"source": "test"},
	}
	
	// Execute
	resp, err := svc.Publish(ctx, req)
	
	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.MessageID)
	assert.Equal(t, "test-topic", resp.Topic)
	assert.GreaterOrEqual(t, resp.Partition, int32(0))
	assert.Less(t, resp.Partition, int32(4))
	
	// Verify mocks
	messageRepo.AssertExpectations(t)
	topicRepo.AssertExpectations(t)
}

func TestPublish_CreateTopicIfNotExists(t *testing.T) {
	svc, messageRepo, topicRepo := setupTestService()
	ctx := context.Background()
	
	// Mock topic doesn't exist initially
	topicRepo.On("Get", ctx, "new-topic").Return(nil, assert.AnError).Once()
	topicRepo.On("Exists", ctx, "new-topic").Return(false, nil)
	topicRepo.On("Create", ctx, mock.AnythingOfType("*topic.Topic")).Return(nil)
	
	// Mock topic exists after creation
	newTopic := topic.NewTopic("new-topic", 4)
	topicRepo.On("Get", ctx, "new-topic").Return(newTopic, nil).Once()
	topicRepo.On("Update", ctx, mock.AnythingOfType("*topic.Topic")).Return(nil)
	
	// Mock message storage
	messageRepo.On("Store", ctx, mock.AnythingOfType("*message.Message")).Return(nil)
	
	// Test request
	req := &PublishRequest{
		Topic:   "new-topic",
		Payload: []byte("Hello, New Topic!"),
	}
	
	// Execute
	resp, err := svc.Publish(ctx, req)
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "new-topic", resp.Topic)
	
	// Verify mocks
	messageRepo.AssertExpectations(t)
	topicRepo.AssertExpectations(t)
}

func TestPublish_ValidationErrors(t *testing.T) {
	svc, _, _ := setupTestService()
	ctx := context.Background()
	
	tests := []struct {
		name string
		req  *PublishRequest
	}{
		{
			name: "empty topic",
			req: &PublishRequest{
				Topic:   "",
				Payload: []byte("test"),
			},
		},
		{
			name: "empty payload",
			req: &PublishRequest{
				Topic:   "test-topic",
				Payload: []byte{},
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Publish(ctx, tt.req)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid publish request")
		})
	}
}

func TestPublishBatch_Success(t *testing.T) {
	svc, messageRepo, topicRepo := setupTestService()
	ctx := context.Background()
	
	// Mock topic exists
	testTopic := topic.NewTopic("test-topic", 4)
	topicRepo.On("Get", ctx, "test-topic").Return(testTopic, nil)
	topicRepo.On("Update", ctx, mock.AnythingOfType("*topic.Topic")).Return(nil)
	
	// Mock batch storage
	messageRepo.On("StoreBatch", ctx, mock.AnythingOfType("[]*message.Message")).Return(nil)
	
	// Test request
	req := &BatchPublishRequest{
		Topic: "test-topic",
		Messages: []*MessageBatch{
			{Payload: []byte("Message 1")},
			{Payload: []byte("Message 2")},
			{Payload: []byte("Message 3")},
		},
	}
	
	// Execute
	resp, err := svc.PublishBatch(ctx, req)
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 3, resp.Published)
	assert.Equal(t, 0, resp.Failed)
	assert.Len(t, resp.MessageIDs, 3)
	assert.Len(t, resp.Responses, 3)
	
	// Verify mocks
	messageRepo.AssertExpectations(t)
	topicRepo.AssertExpectations(t)
}

func TestPublishBatch_PartialFailure(t *testing.T) {
	svc, messageRepo, topicRepo := setupTestService()
	ctx := context.Background()
	
	// Mock topic exists
	testTopic := topic.NewTopic("test-topic", 4)
	topicRepo.On("Get", ctx, "test-topic").Return(testTopic, nil)
	topicRepo.On("Update", ctx, mock.AnythingOfType("*topic.Topic")).Return(nil)
	
	// Mock batch storage
	messageRepo.On("StoreBatch", ctx, mock.AnythingOfType("[]*message.Message")).Return(nil)
	
	// Test request with one invalid message
	req := &BatchPublishRequest{
		Topic: "test-topic",
		Messages: []*MessageBatch{
			{Payload: []byte("Valid Message 1")},
			{Payload: []byte{}}, // Invalid: empty payload
			{Payload: []byte("Valid Message 2")},
		},
	}
	
	// Execute
	resp, err := svc.PublishBatch(ctx, req)
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 2, resp.Published)
	assert.Equal(t, 1, resp.Failed)
	assert.Len(t, resp.Errors, 1)
	
	// Verify mocks
	messageRepo.AssertExpectations(t)
	topicRepo.AssertExpectations(t)
}

func TestCreateTopic_Success(t *testing.T) {
	svc, _, topicRepo := setupTestService()
	ctx := context.Background()
	
	// Mock topic doesn't exist
	topicRepo.On("Exists", ctx, "new-topic").Return(false, nil)
	topicRepo.On("Create", ctx, mock.AnythingOfType("*topic.Topic")).Return(nil)
	
	// Test request
	req := &CreateTopicRequest{
		Name:       "new-topic",
		Partitions: 8,
		Config:     map[string]interface{}{"retention": "24h"},
	}
	
	// Execute
	err := svc.CreateTopic(ctx, req)
	
	// Assert
	assert.NoError(t, err)
	
	// Verify the topic was created with correct values
	topicRepo.AssertCalled(t, "Create", ctx, mock.MatchedBy(func(t *topic.Topic) bool {
		return t.Name == "new-topic" && t.Partitions == 8
	}))
}

func TestCreateTopic_AlreadyExists(t *testing.T) {
	svc, _, topicRepo := setupTestService()
	ctx := context.Background()
	
	// Mock topic already exists
	topicRepo.On("Exists", ctx, "existing-topic").Return(true, nil)
	
	// Test request
	req := &CreateTopicRequest{
		Name: "existing-topic",
	}
	
	// Execute
	err := svc.CreateTopic(ctx, req)
	
	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	
	// Verify create was not called
	topicRepo.AssertNotCalled(t, "Create")
}

func TestGetTopicStats_Success(t *testing.T) {
	svc, messageRepo, topicRepo := setupTestService()
	ctx := context.Background()
	
	// Mock topic exists
	testTopic := topic.NewTopic("test-topic", 4)
	testTopic.MessageCount = 100
	testTopic.BytesIn = 1024
	testTopic.BytesOut = 512
	topicRepo.On("Get", ctx, "test-topic").Return(testTopic, nil)
	
	// Mock message count
	messageRepo.On("CountByTopic", ctx, "test-topic").Return(int64(150), nil)
	
	// Execute
	stats, err := svc.GetTopicStats(ctx, "test-topic")
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "test-topic", stats.Topic)
	assert.Equal(t, int32(4), stats.Partitions)
	assert.Equal(t, int64(150), stats.Messages) // Uses repo count
	assert.Equal(t, int64(1024), stats.BytesIn)
	assert.Equal(t, int64(512), stats.BytesOut)
	
	// Verify mocks
	messageRepo.AssertExpectations(t)
	topicRepo.AssertExpectations(t)
}

func TestListTopics_Success(t *testing.T) {
	svc, _, topicRepo := setupTestService()
	ctx := context.Background()
	
	// Mock topics
	topics := []*topic.Topic{
		topic.NewTopic("topic1", 2),
		topic.NewTopic("topic2", 4),
	}
	topicRepo.On("List", ctx, 0, 1000).Return(topics, nil)
	
	// Execute
	result, err := svc.ListTopics(ctx)
	
	// Assert
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "topic1", result[0].Name)
	assert.Equal(t, "topic2", result[1].Name)
	
	// Verify mocks
	topicRepo.AssertExpectations(t)
}