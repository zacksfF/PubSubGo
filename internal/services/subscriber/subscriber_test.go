package subscriber

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/sirupsen/logrus"
	"github.com/zacksfF/PubSubGo/internal/config"
	"github.com/zacksfF/PubSubGo/internal/core/consumer"
	"github.com/zacksfF/PubSubGo/internal/core/message"
	"github.com/zacksfF/PubSubGo/internal/core/subscription"
	"github.com/zacksfF/PubSubGo/internal/core/topic"
)

// Mock repositories (reusing some from publisher tests)
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

type MockSubscriptionRepository struct {
	mock.Mock
}

func (m *MockSubscriptionRepository) Create(ctx context.Context, sub *subscription.Subscription) error {
	args := m.Called(ctx, sub)
	return args.Error(0)
}

func (m *MockSubscriptionRepository) Get(ctx context.Context, id string) (*subscription.Subscription, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*subscription.Subscription), args.Error(1)
}

func (m *MockSubscriptionRepository) Update(ctx context.Context, sub *subscription.Subscription) error {
	args := m.Called(ctx, sub)
	return args.Error(0)
}

func (m *MockSubscriptionRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSubscriptionRepository) GetByTopic(ctx context.Context, topic string) ([]*subscription.Subscription, error) {
	args := m.Called(ctx, topic)
	return args.Get(0).([]*subscription.Subscription), args.Error(1)
}

func (m *MockSubscriptionRepository) GetByConsumerGroup(ctx context.Context, consumerGroup string) ([]*subscription.Subscription, error) {
	args := m.Called(ctx, consumerGroup)
	return args.Get(0).([]*subscription.Subscription), args.Error(1)
}

func (m *MockSubscriptionRepository) GetActive(ctx context.Context) ([]*subscription.Subscription, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*subscription.Subscription), args.Error(1)
}

func (m *MockSubscriptionRepository) List(ctx context.Context, offset, limit int) ([]*subscription.Subscription, error) {
	args := m.Called(ctx, offset, limit)
	return args.Get(0).([]*subscription.Subscription), args.Error(1)
}

type MockConsumerGroupRepository struct {
	mock.Mock
}

func (m *MockConsumerGroupRepository) CreateGroup(ctx context.Context, group *consumer.ConsumerGroup) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockConsumerGroupRepository) GetGroup(ctx context.Context, name string) (*consumer.ConsumerGroup, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*consumer.ConsumerGroup), args.Error(1)
}

func (m *MockConsumerGroupRepository) UpdateGroup(ctx context.Context, group *consumer.ConsumerGroup) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockConsumerGroupRepository) DeleteGroup(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockConsumerGroupRepository) ListGroups(ctx context.Context, topic string) ([]*consumer.ConsumerGroup, error) {
	args := m.Called(ctx, topic)
	return args.Get(0).([]*consumer.ConsumerGroup), args.Error(1)
}

func (m *MockConsumerGroupRepository) AddConsumer(ctx context.Context, groupName string, consumer *consumer.Consumer) error {
	args := m.Called(ctx, groupName, consumer)
	return args.Error(0)
}

func (m *MockConsumerGroupRepository) RemoveConsumer(ctx context.Context, groupName, consumerID string) error {
	args := m.Called(ctx, groupName, consumerID)
	return args.Error(0)
}

func (m *MockConsumerGroupRepository) UpdateConsumerHeartbeat(ctx context.Context, groupName, consumerID string) error {
	args := m.Called(ctx, groupName, consumerID)
	return args.Error(0)
}

func (m *MockConsumerGroupRepository) GetActiveConsumers(ctx context.Context, groupName string) ([]*consumer.Consumer, error) {
	args := m.Called(ctx, groupName)
	return args.Get(0).([]*consumer.Consumer), args.Error(1)
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
func setupSubscriberTestService() (*service, *MockMessageRepository, *MockSubscriptionRepository, *MockConsumerGroupRepository, *MockTopicRepository) {
	messageRepo := &MockMessageRepository{}
	subscriptionRepo := &MockSubscriptionRepository{}
	consumerRepo := &MockConsumerGroupRepository{}
	topicRepo := &MockTopicRepository{}
	
	config := &config.BrokerConfig{
		DefaultAckDeadline: 30 * time.Second,
		MaxRetries:         3,
	}
	
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce noise in tests
	
	// Create service without starting background workers for tests
	svc := &service{
		messageRepo:      messageRepo,
		subscriptionRepo: subscriptionRepo,
		consumerRepo:     consumerRepo,
		topicRepo:        topicRepo,
		config:           config,
		logger:           logger,
		subscriptions:    make(map[string]*activeSubscription),
		consumerGroups:   make(map[string]*activeConsumerGroup),
		workers:          make(map[string]context.CancelFunc),
	}
	
	return svc, messageRepo, subscriptionRepo, consumerRepo, topicRepo
}

func TestSubscribe_PullMode_Success(t *testing.T) {
	svc, _, subscriptionRepo, _, topicRepo := setupSubscriberTestService()
	ctx := context.Background()
	
	// Mock topic exists
	topicRepo.On("Exists", ctx, "test-topic").Return(true, nil)
	
	// Mock subscription creation
	subscriptionRepo.On("Create", ctx, mock.AnythingOfType("*subscription.Subscription")).Return(nil)
	
	// Test request
	req := &SubscribeRequest{
		Topic: "test-topic",
		Type:  subscription.TypePull,
	}
	
	// Execute
	resp, err := svc.Subscribe(ctx, req)
	
	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.SubscriptionID)
	assert.Equal(t, "test-topic", resp.Topic)
	assert.Equal(t, "pull", resp.Type)
	assert.Nil(t, resp.MessageChan) // No channel for pull mode
	
	// Verify subscription was stored
	assert.Contains(t, svc.subscriptions, resp.SubscriptionID)
	
	// Verify mocks
	subscriptionRepo.AssertExpectations(t)
	topicRepo.AssertExpectations(t)
}

func TestSubscribe_PushMode_Success(t *testing.T) {
	svc, _, subscriptionRepo, _, topicRepo := setupSubscriberTestService()
	ctx := context.Background()
	
	// Mock topic exists
	topicRepo.On("Exists", ctx, "test-topic").Return(true, nil)
	
	// Mock subscription creation
	subscriptionRepo.On("Create", ctx, mock.AnythingOfType("*subscription.Subscription")).Return(nil)
	
	// Test request
	req := &SubscribeRequest{
		Topic:         "test-topic",
		Type:          subscription.TypePush,
		ConsumerGroup: "test-group",
	}
	
	// Execute
	resp, err := svc.Subscribe(ctx, req)
	
	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.SubscriptionID)
	assert.Equal(t, "test-topic", resp.Topic)
	assert.Equal(t, "test-group", resp.ConsumerGroup)
	assert.Equal(t, "push", resp.Type)
	assert.NotNil(t, resp.MessageChan) // Channel for push mode
	
	// Verify subscription was stored
	activeSub, exists := svc.subscriptions[resp.SubscriptionID]
	assert.True(t, exists)
	assert.NotNil(t, activeSub.cancelFunc) // Worker should be started
	
	// Cleanup
	if activeSub.cancelFunc != nil {
		activeSub.cancelFunc()
	}
	
	// Verify mocks
	subscriptionRepo.AssertExpectations(t)
	topicRepo.AssertExpectations(t)
}

func TestSubscribe_TopicNotExists(t *testing.T) {
	svc, _, _, _, topicRepo := setupSubscriberTestService()
	ctx := context.Background()
	
	// Mock topic doesn't exist
	topicRepo.On("Exists", ctx, "nonexistent-topic").Return(false, nil)
	
	// Test request
	req := &SubscribeRequest{
		Topic: "nonexistent-topic",
		Type:  subscription.TypePull,
	}
	
	// Execute
	_, err := svc.Subscribe(ctx, req)
	
	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
	
	// Verify mocks
	topicRepo.AssertExpectations(t)
}

func TestPull_Success(t *testing.T) {
	svc, messageRepo, _, _, _ := setupSubscriberTestService()
	ctx := context.Background()
	
	// Create test messages
	testMessages := []*message.Message{
		{
			ID:      "msg-1",
			Topic:   "test-topic",
			Payload: []byte("Hello 1"),
			Offset:  0,
		},
		{
			ID:      "msg-2",
			Topic:   "test-topic",
			Payload: []byte("Hello 2"),
			Offset:  1,
		},
	}
	
	// Mock message retrieval
	messageRepo.On("GetByTopic", ctx, "test-topic", 10, int64(0)).Return(testMessages, nil)
	
	// Test request
	req := &PullRequest{
		Topic: "test-topic",
		Limit: 10,
	}
	
	// Execute
	resp, err := svc.Pull(ctx, req)
	
	// Assert
	assert.NoError(t, err)
	assert.Len(t, resp.Messages, 2)
	assert.Equal(t, "msg-1", resp.Messages[0].ID)
	assert.Equal(t, "msg-2", resp.Messages[1].ID)
	assert.NotNil(t, resp.NextOffset)
	assert.Equal(t, int64(2), *resp.NextOffset)
	assert.False(t, resp.HasMore) // We got 2 messages but limit was 10
	
	// Verify mocks
	messageRepo.AssertExpectations(t)
}

func TestPullWithConsumerGroup_Success(t *testing.T) {
	svc, messageRepo, _, consumerRepo, _ := setupSubscriberTestService()
	ctx := context.Background()
	
	// Mock consumer group exists
	testGroup := consumer.NewConsumerGroup("test-group", "test-topic")
	consumerRepo.On("GetGroup", ctx, "test-group").Return(testGroup, nil)
	consumerRepo.On("AddConsumer", ctx, "test-group", mock.AnythingOfType("*consumer.Consumer")).Return(nil)
	
	// Create test messages
	testMessages := []*message.Message{
		{
			ID:        "msg-1",
			Topic:     "test-topic",
			Payload:   []byte("Hello 1"),
			Partition: 0,
		},
	}
	
	// Mock message retrieval
	messageRepo.On("GetForConsumerGroup", ctx, "test-topic", "test-group", 5).Return(testMessages, nil)
	
	// Test request
	req := &ConsumerGroupPullRequest{
		Topic:         "test-topic",
		ConsumerGroup: "test-group",
		ConsumerID:    "consumer-1",
		Limit:         5,
	}
	
	// Execute
	resp, err := svc.PullWithConsumerGroup(ctx, req)
	
	// Assert
	assert.NoError(t, err)
	assert.Len(t, resp.Messages, 1)
	assert.Equal(t, "msg-1", resp.Messages[0].ID)
	assert.Equal(t, "consumer-1", resp.ConsumerID)
	
	// Verify mocks
	messageRepo.AssertExpectations(t)
	consumerRepo.AssertExpectations(t)
}

func TestAcknowledge_Success(t *testing.T) {
	svc, messageRepo, _, _, _ := setupSubscriberTestService()
	ctx := context.Background()
	
	// Mock acknowledgment
	messageRepo.On("Acknowledge", ctx, mock.AnythingOfType("*message.Acknowledgment")).Return(nil)
	
	// Test request
	req := &AcknowledgeRequest{
		MessageID:     "msg-1",
		ConsumerID:    "consumer-1",
		ConsumerGroup: "test-group",
	}
	
	// Execute
	err := svc.Acknowledge(ctx, req)
	
	// Assert
	assert.NoError(t, err)
	
	// Verify mocks
	messageRepo.AssertExpectations(t)
}

func TestUnsubscribe_Success(t *testing.T) {
	svc, _, subscriptionRepo, _, topicRepo := setupSubscriberTestService()
	ctx := context.Background()
	
	// First create a subscription
	topicRepo.On("Exists", ctx, "test-topic").Return(true, nil)
	subscriptionRepo.On("Create", ctx, mock.AnythingOfType("*subscription.Subscription")).Return(nil)
	
	req := &SubscribeRequest{
		Topic: "test-topic",
		Type:  subscription.TypePull,
	}
	
	resp, err := svc.Subscribe(ctx, req)
	assert.NoError(t, err)
	
	// Mock subscription update
	subscriptionRepo.On("Update", ctx, mock.AnythingOfType("*subscription.Subscription")).Return(nil)
	
	// Execute unsubscribe
	err = svc.Unsubscribe(ctx, resp.SubscriptionID)
	
	// Assert
	assert.NoError(t, err)
	
	// Verify subscription was removed
	_, exists := svc.subscriptions[resp.SubscriptionID]
	assert.False(t, exists)
	
	// Verify mocks
	subscriptionRepo.AssertExpectations(t)
	topicRepo.AssertExpectations(t)
}

func TestJoinConsumerGroup_Success(t *testing.T) {
	svc, _, _, consumerRepo, _ := setupSubscriberTestService()
	ctx := context.Background()
	
	// Mock consumer group creation
	consumerRepo.On("GetGroup", ctx, "test-group").Return(nil, assert.AnError).Once() // Group doesn't exist
	consumerRepo.On("CreateGroup", ctx, mock.AnythingOfType("*consumer.ConsumerGroup")).Return(nil)
	consumerRepo.On("GetGroup", ctx, "test-group").Return(consumer.NewConsumerGroup("test-group", "test-topic"), nil).Once()
	consumerRepo.On("AddConsumer", ctx, "test-group", mock.AnythingOfType("*consumer.Consumer")).Return(nil).Maybe()
	
	// Test request
	req := &JoinGroupRequest{
		Topic:         "test-topic",
		ConsumerGroup: "test-group",
		ConsumerID:    "consumer-1",
		Metadata:      map[string]string{"version": "1.0"},
	}
	
	// Execute
	resp, err := svc.JoinConsumerGroup(ctx, req)
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "consumer-1", resp.ConsumerID)
	assert.Equal(t, "test-group", resp.ConsumerGroup)
	
	// Verify consumer was added to active groups
	assert.Contains(t, svc.consumerGroups, "test-group")
	assert.Contains(t, svc.consumerGroups["test-group"].consumers, "consumer-1")
	
	// Verify mocks
	consumerRepo.AssertExpectations(t)
}

func TestValidateRequests(t *testing.T) {
	svc, _, _, _, _ := setupSubscriberTestService()
	
	tests := []struct {
		name    string
		request interface{}
		fn      func() error
	}{
		{
			name: "empty topic in subscribe",
			request: &SubscribeRequest{
				Topic: "",
				Type:  subscription.TypePull,
			},
			fn: func() error {
				return svc.validateSubscribeRequest(&SubscribeRequest{
					Topic: "",
					Type:  subscription.TypePull,
				})
			},
		},
		{
			name: "invalid subscription type",
			request: &SubscribeRequest{
				Topic: "test-topic",
				Type:  99, // Invalid type
			},
			fn: func() error {
				return svc.validateSubscribeRequest(&SubscribeRequest{
					Topic: "test-topic",
					Type:  99,
				})
			},
		},
		{
			name: "empty topic in pull",
			request: &PullRequest{
				Topic: "",
				Limit: 10,
			},
			fn: func() error {
				return svc.validatePullRequest(&PullRequest{
					Topic: "",
					Limit: 10,
				})
			},
		},
		{
			name: "invalid limit in pull",
			request: &PullRequest{
				Topic: "test-topic",
				Limit: 0,
			},
			fn: func() error {
				return svc.validatePullRequest(&PullRequest{
					Topic: "test-topic",
					Limit: 0,
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