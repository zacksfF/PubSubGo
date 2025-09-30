package topic

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/zacksfF/PubSubGo/internal/config"
	"github.com/zacksfF/PubSubGo/internal/core/message"
	"github.com/zacksfF/PubSubGo/internal/core/topic"
)

// MockTopicRepository for testing
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
func setupTopicTestService() (*service, *MockTopicRepository, *MockMessageRepository) {
	topicRepo := &MockTopicRepository{}
	messageRepo := &MockMessageRepository{}

	cfg := &config.BrokerConfig{
		DefaultPartitions:  1,
		DefaultReplication: 1,
		MaxRetries:         3,
		RetryBackoff:       time.Second,
		DLQPrefix:          "dlq-",
		EnableDLQ:          true,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce noise in tests

	// Create service without starting background workers for tests
	svc := &service{
		topicRepo:        topicRepo,
		messageRepo:      messageRepo,
		config:           cfg,
		logger:           logger,
		activeTopics:     make(map[string]*activeTopic),
		workers:          make(map[string]context.CancelFunc),
		partitionManager: NewPartitionManager(cfg, logger),
		statsCollector:   NewStatsCollector(cfg, logger),
	}

	return svc, topicRepo, messageRepo
}

func TestCreateTopic_Success(t *testing.T) {
	svc, topicRepo, _ := setupTopicTestService()
	ctx := context.Background()

	// Mock topic doesn't exist
	topicRepo.On("Exists", ctx, "test-topic").Return(false, nil)
	topicRepo.On("Create", ctx, mock.AnythingOfType("*topic.Topic")).Return(nil)

	// Test request
	req := &CreateTopicRequest{
		Name:          "test-topic",
		Partitions:    2,
		Replication:   1,
		RetentionTime: 24 * time.Hour,
		MaxSize:       1 << 30, // 1GB
	}

	// Execute
	resp, err := svc.CreateTopic(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "test-topic", resp.Name)
	assert.Equal(t, int32(2), resp.Partitions)
	assert.Equal(t, int32(1), resp.Replication)
	assert.Equal(t, 24*time.Hour, resp.RetentionTime)
	assert.Equal(t, int64(1<<30), resp.MaxSize)
	assert.NotZero(t, resp.CreatedAt)

	// Verify topic was added to active topics cache
	assert.Contains(t, svc.activeTopics, "test-topic")

	// Verify mocks
	topicRepo.AssertExpectations(t)
}

func TestCreateTopic_AlreadyExists(t *testing.T) {
	svc, topicRepo, _ := setupTopicTestService()
	ctx := context.Background()

	// Mock topic already exists
	topicRepo.On("Exists", ctx, "existing-topic").Return(true, nil)

	// Test request
	req := &CreateTopicRequest{
		Name: "existing-topic",
	}

	// Execute
	_, err := svc.CreateTopic(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Verify mocks
	topicRepo.AssertExpectations(t)
}

func TestCreateTopic_ValidationErrors(t *testing.T) {
	svc, _, _ := setupTopicTestService()
	ctx := context.Background()

	tests := []struct {
		name    string
		request *CreateTopicRequest
	}{
		{
			name: "empty topic name",
			request: &CreateTopicRequest{
				Name: "",
			},
		},
		{
			name: "negative partitions",
			request: &CreateTopicRequest{
				Name:       "test-topic",
				Partitions: -1,
			},
		},
		{
			name: "negative replication",
			request: &CreateTopicRequest{
				Name:        "test-topic",
				Replication: -1,
			},
		},
		{
			name: "negative retention time",
			request: &CreateTopicRequest{
				Name:          "test-topic",
				RetentionTime: -time.Hour,
			},
		},
		{
			name: "negative max size",
			request: &CreateTopicRequest{
				Name:    "test-topic",
				MaxSize: -1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateTopic(ctx, tt.request)
			assert.Error(t, err)
		})
	}
}

func TestGetTopic_Success(t *testing.T) {
	svc, topicRepo, _ := setupTopicTestService()
	ctx := context.Background()

	// Create test topic
	testTopic := topic.NewTopic("test-topic", 2)
	testTopic.Replication = 1
	testTopic.RetentionTime = 24 * time.Hour
	testTopic.MaxSize = 1 << 30

	// Mock topic retrieval
	topicRepo.On("Get", ctx, "test-topic").Return(testTopic, nil)

	// Execute
	topicInfo, err := svc.GetTopic(ctx, "test-topic")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "test-topic", topicInfo.Name)
	assert.Equal(t, int32(2), topicInfo.Partitions)
	assert.Equal(t, int32(1), topicInfo.Replication)
	assert.Equal(t, 24*time.Hour, topicInfo.RetentionTime)
	assert.Equal(t, int64(1<<30), topicInfo.MaxSize)
	assert.NotEmpty(t, topicInfo.Status)
	assert.NotEmpty(t, topicInfo.Health)

	// Verify mocks
	topicRepo.AssertExpectations(t)
}

func TestUpdateTopic_Success(t *testing.T) {
	svc, topicRepo, _ := setupTopicTestService()
	ctx := context.Background()

	// Create test topic
	testTopic := topic.NewTopic("test-topic", 2)
	testTopic.RetentionTime = 24 * time.Hour
	testTopic.MaxSize = 1 << 30

	// Mock topic retrieval and update
	topicRepo.On("Get", ctx, "test-topic").Return(testTopic, nil) // Allow unlimited calls
	topicRepo.On("Update", ctx, mock.AnythingOfType("*topic.Topic")).Return(nil)

	// Test request
	newRetentionTime := 48 * time.Hour
	newMaxSize := int64(2 << 30) // 2GB
	req := &UpdateTopicRequest{
		RetentionTime: &newRetentionTime,
		MaxSize:       &newMaxSize,
		Config: map[string]interface{}{
			"compression": "snappy",
		},
	}

	// Execute
	updatedTopic, err := svc.UpdateTopic(ctx, "test-topic", req)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, newRetentionTime, updatedTopic.RetentionTime)
	assert.Equal(t, newMaxSize, updatedTopic.MaxSize)
	assert.Equal(t, "snappy", updatedTopic.Config["compression"])

	// Verify mocks
	topicRepo.AssertExpectations(t)
}

func TestUpdateTopic_NoUpdates(t *testing.T) {
	svc, topicRepo, _ := setupTopicTestService()
	ctx := context.Background()

	// Create test topic
	testTopic := topic.NewTopic("test-topic", 2)

	// Mock topic retrieval
	topicRepo.On("Get", ctx, "test-topic").Return(testTopic, nil)

	// Test request with no updates
	req := &UpdateTopicRequest{}

	// Execute
	_, err := svc.UpdateTopic(ctx, "test-topic", req)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no updates provided")

	// Verify mocks
	topicRepo.AssertExpectations(t)
}

func TestDeleteTopic_Success(t *testing.T) {
	svc, topicRepo, _ := setupTopicTestService()
	ctx := context.Background()

	// Add topic to active cache first
	testTopic := topic.NewTopic("test-topic", 2)
	svc.addToActiveTopics(testTopic)

	// Mock topic exists and deletion
	topicRepo.On("Exists", ctx, "test-topic").Return(true, nil)
	topicRepo.On("Delete", ctx, "test-topic").Return(nil)

	// Execute
	err := svc.DeleteTopic(ctx, "test-topic")

	// Assert
	assert.NoError(t, err)

	// Verify topic was removed from active cache
	assert.NotContains(t, svc.activeTopics, "test-topic")

	// Verify mocks
	topicRepo.AssertExpectations(t)
}

func TestDeleteTopic_NotExists(t *testing.T) {
	svc, topicRepo, _ := setupTopicTestService()
	ctx := context.Background()

	// Mock topic doesn't exist
	topicRepo.On("Exists", ctx, "nonexistent-topic").Return(false, nil)

	// Execute
	err := svc.DeleteTopic(ctx, "nonexistent-topic")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")

	// Verify mocks
	topicRepo.AssertExpectations(t)
}

func TestListTopics_Success(t *testing.T) {
	svc, topicRepo, _ := setupTopicTestService()
	ctx := context.Background()

	// Create test topics
	testTopics := []*topic.Topic{
		topic.NewTopic("topic-1", 1),
		topic.NewTopic("topic-2", 2),
		topic.NewTopic("topic-3", 1),
	}

	// Mock topic listing
	topicRepo.On("List", ctx, 0, 51).Return(testTopics, nil) // +1 for hasMore check

	// Test request
	req := &ListTopicsRequest{
		Offset: 0,
		Limit:  50,
	}

	// Execute
	resp, err := svc.ListTopics(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, resp.Topics, 3)
	assert.Equal(t, 3, resp.Total)
	assert.Equal(t, 0, resp.Offset)
	assert.Equal(t, 50, resp.Limit)
	assert.False(t, resp.HasMore)

	// Check topic details
	assert.Equal(t, "topic-1", resp.Topics[0].Name)
	assert.Equal(t, int32(1), resp.Topics[0].Partitions)
	assert.Equal(t, "topic-2", resp.Topics[1].Name)
	assert.Equal(t, int32(2), resp.Topics[1].Partitions)

	// Verify mocks
	topicRepo.AssertExpectations(t)
}

func TestListTopics_WithFilter(t *testing.T) {
	svc, topicRepo, _ := setupTopicTestService()
	ctx := context.Background()

	// Create test topics
	testTopics := []*topic.Topic{
		topic.NewTopic("user-events", 1),
		topic.NewTopic("system-logs", 2),
		topic.NewTopic("user-actions", 1),
	}

	// Mock topic listing
	topicRepo.On("List", ctx, 0, 51).Return(testTopics, nil)

	// Test request with filter
	req := &ListTopicsRequest{
		Offset: 0,
		Limit:  50,
		Filter: "user", // Should match user-events and user-actions
	}

	// Execute
	resp, err := svc.ListTopics(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, resp.Topics, 2) // Only topics containing "user"
	assert.Equal(t, "user-events", resp.Topics[0].Name)
	assert.Equal(t, "user-actions", resp.Topics[1].Name)

	// Verify mocks
	topicRepo.AssertExpectations(t)
}

func TestGetTopicStats_Success(t *testing.T) {
	svc, topicRepo, _ := setupTopicTestService()
	ctx := context.Background()

	// Create test topic with some stats
	testTopic := topic.NewTopic("test-topic", 2)
	testTopic.IncrementMessages(100)
	testTopic.AddBytesIn(1024)
	testTopic.AddBytesOut(512)

	// Mock topic retrieval
	topicRepo.On("Get", ctx, "test-topic").Return(testTopic, nil)

	// Execute
	stats, err := svc.GetTopicStats(ctx, "test-topic")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "test-topic", stats.Name)
	assert.Equal(t, int64(100), stats.MessageCount)
	assert.Equal(t, int64(1024), stats.BytesIn)
	assert.Equal(t, int64(512), stats.BytesOut)
	assert.Equal(t, int32(2), stats.Partitions)
	assert.NotZero(t, stats.CollectedAt)

	// Verify mocks
	topicRepo.AssertExpectations(t)
}

func TestGetTopicHealth_Success(t *testing.T) {
	svc, topicRepo, _ := setupTopicTestService()
	ctx := context.Background()

	// Create test topic
	testTopic := topic.NewTopic("test-topic", 2)
	testTopic.RetentionTime = 24 * time.Hour
	testTopic.MaxSize = 1 << 30

	// Initialize partitions for the topic
	err := svc.partitionManager.InitializePartitions(ctx, "test-topic", 2)
	assert.NoError(t, err)

	// Mock topic retrieval
	topicRepo.On("Get", ctx, "test-topic").Return(testTopic, nil)

	// Execute
	health, err := svc.GetTopicHealth(ctx, "test-topic")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "test-topic", health.Name)
	assert.NotEmpty(t, health.Status)
	assert.NotEmpty(t, health.Health)
	assert.NotEmpty(t, health.Checks)
	assert.True(t, health.Score >= 0 && health.Score <= 100)
	assert.NotZero(t, health.LastChecked)

	// Verify mocks
	topicRepo.AssertExpectations(t)
}

func TestPartitionManager_Operations(t *testing.T) {
	svc, _, _ := setupTopicTestService()
	ctx := context.Background()

	topicName := "test-topic"
	partitionCount := int32(3)

	// Test partition initialization
	err := svc.partitionManager.InitializePartitions(ctx, topicName, partitionCount)
	assert.NoError(t, err)

	// Test getting partition info
	partitionInfo, err := svc.GetPartitionInfo(ctx, topicName)
	assert.NoError(t, err)
	assert.Equal(t, topicName, partitionInfo.TopicName)
	assert.Equal(t, partitionCount, partitionInfo.TotalPartitions)
	assert.Len(t, partitionInfo.Partitions, int(partitionCount))
	assert.NotNil(t, partitionInfo.LoadBalance)

	// Test partition assignment
	key := []byte("test-key")
	partitionID := svc.partitionManager.AssignPartition(topicName, key, partitionCount)
	assert.True(t, partitionID >= 0 && partitionID < partitionCount)

	// Test adding consumer
	err = svc.partitionManager.AddConsumer(ctx, topicName, "consumer-1")
	assert.NoError(t, err)

	// Test getting consumer partitions
	partitions, err := svc.partitionManager.GetConsumerPartitions(topicName, "consumer-1")
	assert.NoError(t, err)
	assert.NotNil(t, partitions) // Should have some partitions assigned

	// Test removing consumer
	err = svc.partitionManager.RemoveConsumer(ctx, topicName, "consumer-1")
	assert.NoError(t, err)

	// Test cleanup
	err = svc.partitionManager.CleanupPartitions(ctx, topicName)
	assert.NoError(t, err)
}

func TestWorkerManagement(t *testing.T) {
	svc, _, _ := setupTopicTestService()
	ctx := context.Background()

	// Test starting workers
	err := svc.StartTopicWorkers(ctx)
	assert.NoError(t, err)

	// Verify workers are running
	svc.workersMutex.RLock()
	workerCount := len(svc.workers)
	svc.workersMutex.RUnlock()
	assert.Equal(t, 3, workerCount) // stats_collector, health_monitor, cleanup

	// Test stopping workers
	err = svc.StopTopicWorkers()
	assert.NoError(t, err)

	// Verify workers are stopped
	svc.workersMutex.RLock()
	workerCount = len(svc.workers)
	svc.workersMutex.RUnlock()
	assert.Equal(t, 0, workerCount)
}

func TestStatsCollector_Operations(t *testing.T) {
	svc, _, _ := setupTopicTestService()
	ctx := context.Background()

	// Create test topic with some activity
	testTopic := topic.NewTopic("test-topic", 2)
	testTopic.IncrementMessages(50)
	testTopic.AddBytesIn(2048)
	testTopic.AddBytesOut(1024)

	// Test stats collection
	stats, err := svc.statsCollector.CollectTopicStats(ctx, testTopic, svc.messageRepo)
	assert.NoError(t, err)
	assert.Equal(t, "test-topic", stats.Name)
	assert.Equal(t, int64(50), stats.MessageCount)
	assert.Equal(t, int64(2048), stats.BytesIn)
	assert.Equal(t, int64(1024), stats.BytesOut)

	// Test cached stats
	cachedStats, found := svc.statsCollector.GetCachedStats("test-topic")
	assert.True(t, found)
	assert.Equal(t, stats.MessageCount, cachedStats.MessageCount)

	// Test health collection
	partitionInfo := &PartitionInfo{
		TopicName:       "test-topic",
		TotalPartitions: 2,
		LoadBalance: &LoadBalanceInfo{
			IsBalanced:         true,
			ImbalanceRatio:     0.1,
			RecommendRebalance: false,
		},
	}

	health, err := svc.statsCollector.CollectTopicHealth(ctx, testTopic, partitionInfo)
	assert.NoError(t, err)
	assert.Equal(t, "test-topic", health.Name)
	assert.NotEmpty(t, health.Status)
	assert.NotEmpty(t, health.Health)
	assert.NotEmpty(t, health.Checks)

	// Test cached health
	cachedHealth, found := svc.statsCollector.GetCachedHealth("test-topic")
	assert.True(t, found)
	assert.Equal(t, health.Score, cachedHealth.Score)

	// Test cache clearing
	svc.statsCollector.ClearCache()
	_, found = svc.statsCollector.GetCachedStats("test-topic")
	assert.False(t, found)
	_, found = svc.statsCollector.GetCachedHealth("test-topic")
	assert.False(t, found)
}

func TestTopicFiltering(t *testing.T) {
	svc, _, _ := setupTopicTestService()

	topics := []*topic.Topic{
		topic.NewTopic("user-events", 1),
		topic.NewTopic("system-logs", 2),
		topic.NewTopic("user-actions", 1),
		topic.NewTopic("admin-events", 1),
	}

	// Test filtering with "user"
	filtered := svc.statsCollector.FilterTopics(topics, "user")
	assert.Len(t, filtered, 2)
	assert.Equal(t, "user-events", filtered[0].Name)
	assert.Equal(t, "user-actions", filtered[1].Name)

	// Test filtering with "logs"
	filtered = svc.statsCollector.FilterTopics(topics, "logs")
	assert.Len(t, filtered, 1)
	assert.Equal(t, "system-logs", filtered[0].Name)

	// Test empty filter
	filtered = svc.statsCollector.FilterTopics(topics, "")
	assert.Len(t, filtered, 4)

	// Test no matches
	filtered = svc.statsCollector.FilterTopics(topics, "nonexistent")
	assert.Len(t, filtered, 0)
}
