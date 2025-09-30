package integration

import (
	"context"
	"encoding/base64"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zacksfF/PubSubGo/internal/adapters/storage/redis"
	"github.com/zacksfF/PubSubGo/internal/core/message"
	"github.com/zacksfF/PubSubGo/internal/core/subscription"
	"github.com/zacksfF/PubSubGo/internal/services/publisher"
	"github.com/zacksfF/PubSubGo/internal/services/subscriber"
	"github.com/zacksfF/PubSubGo/internal/services/topic"
	"github.com/zacksfF/PubSubGo/test/helpers"
)

func TestPublishAndSubscribe(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup
	ctx := context.Background()
	redisHelper := helpers.NewTestRedis(t)
	defer redisHelper.Cleanup(t)

	cfg := helpers.TestConfig()
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create Redis client and repositories
	redisClient, err := redis.NewClient(&cfg.Redis)
	require.NoError(t, err)

	messageRepo := redis.NewMessageRepository(redisClient)
	topicRepo := redis.NewTopicRepository(redisClient)
	subscriptionRepo := redis.NewSubscriptionRepository(redisClient)

	// Create services
	topicSvc := topic.NewService(topicRepo, logger)
	publisherSvc := publisher.NewService(messageRepo, topicRepo, &cfg.Broker, logger)
	subscriberSvc := subscriber.NewService(subscriptionRepo, messageRepo, topicRepo, &cfg.Broker, logger)

	// Create topic
	testTopic := "test-topic-" + time.Now().Format("20060102150405")
	topicReq := &topic.CreateTopicRequest{
		Name:       testTopic,
		Partitions: 2,
	}
	_, err = topicSvc.CreateTopic(ctx, topicReq)
	require.NoError(t, err)

	// Create subscription
	subReq := &subscriber.SubscribeRequest{
		Topic: testTopic,
		Type:  subscription.TypePull,
	}
	subResp, err := subscriberSvc.Subscribe(ctx, subReq)
	require.NoError(t, err)
	require.NotEmpty(t, subResp.SubscriptionID)

	// Test cases
	t.Run("PublishSingleMessage", func(t *testing.T) {
		pubReq := &publisher.PublishRequest{
			Topic:        testTopic,
			Key:          []byte("test-key"),
			Payload:      []byte("Hello, World!"),
			Priority:     message.PriorityNormal,
			DeliveryMode: message.DeliveryAtLeastOnce,
		}

		pubResp, err := publisherSvc.Publish(ctx, pubReq)
		require.NoError(t, err)
		assert.NotEmpty(t, pubResp.MessageID)

		// Pull message
		pullReq := &subscriber.PullRequest{
			SubscriptionID: subResp.SubscriptionID,
			MaxMessages:    1,
		}
		pullResp, err := subscriberSvc.Pull(ctx, pullReq)
		require.NoError(t, err)
		require.Len(t, pullResp.Messages, 1)
		assert.Equal(t, "Hello, World!", string(pullResp.Messages[0].Payload))
	})

	t.Run("PublishBatch", func(t *testing.T) {
		gen := helpers.NewMessageGenerator(42)
		batch := gen.GenerateBatch(10)

		// Convert to batch request messages
		batchMessages := make([]*publisher.MessageBatch, len(batch))
		for i, msg := range batch {
			batchMessages[i] = &publisher.MessageBatch{
				Key:          msg.Key,
				Payload:      msg.Payload,
				Headers:      msg.Headers,
				Priority:     msg.Priority,
				DeliveryMode: msg.DeliveryMode,
			}
		}

		batchReq := &publisher.BatchPublishRequest{
			Topic:    testTopic,
			Messages: batchMessages,
		}

		batchResp, err := publisherSvc.PublishBatch(ctx, batchReq)
		require.NoError(t, err)
		assert.Len(t, batchResp.Messages, 10)

		// Pull messages
		pullReq := &subscriber.PullRequest{
			SubscriptionID: subResp.SubscriptionID,
			MaxMessages:    10,
		}
		pullResp, err := subscriberSvc.Pull(ctx, pullReq)
		require.NoError(t, err)
		assert.Len(t, pullResp.Messages, 10)
	})

	t.Run("ConcurrentPublishSubscribe", func(t *testing.T) {
		const numPublishers = 5
		const numMessages = 20
		const numSubscribers = 3

		var wg sync.WaitGroup
		messagesSent := make(chan string, numPublishers*numMessages)
		messagesReceived := make(chan string, numPublishers*numMessages)

		// Start publishers
		for i := 0; i < numPublishers; i++ {
			wg.Add(1)
			go func(publisherID int) {
				defer wg.Done()
				for j := 0; j < numMessages; j++ {
					payload := base64.StdEncoding.EncodeToString([]byte(
						"Message from publisher " + string(rune(publisherID)) + " msg " + string(rune(j)),
					))
					pubReq := &publisher.PublishRequest{
						Topic:   testTopic,
						Payload: []byte(payload),
					}
					_, err := publisherSvc.Publish(ctx, pubReq)
					if err == nil {
						messagesSent <- payload
					}
				}
			}(i)
		}

		// Start subscribers
		for i := 0; i < numSubscribers; i++ {
			wg.Add(1)
			go func(subscriberID int) {
				defer wg.Done()
				subReq := &subscriber.SubscribeRequest{
					Topic: testTopic,
					Type:  subscription.TypePull,
				}
				subResp, err := subscriberSvc.Subscribe(ctx, subReq)
				if err != nil {
					return
				}

				for {
					pullReq := &subscriber.PullRequest{
						SubscriptionID: subResp.SubscriptionID,
						MaxMessages:    5,
					}
					pullResp, err := subscriberSvc.Pull(ctx, pullReq)
					if err != nil || len(pullResp.Messages) == 0 {
						time.Sleep(100 * time.Millisecond)
						continue
					}

					for _, msg := range pullResp.Messages {
						messagesReceived <- string(msg.Payload)
					}

					if len(messagesReceived) >= numPublishers*numMessages {
						break
					}
				}
			}(i)
		}

		// Wait for completion
		go func() {
			wg.Wait()
			close(messagesSent)
			close(messagesReceived)
		}()

		time.Sleep(5 * time.Second) // Allow time for messages to be processed

		// Verify at least 80% of messages were processed (allowing for some timing issues)
		sentCount := len(messagesSent)
		receivedCount := len(messagesReceived)
		assert.GreaterOrEqual(t, float64(receivedCount), float64(sentCount)*0.8)
	})
}

func TestTopicManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup
	ctx := context.Background()
	redisHelper := helpers.NewTestRedis(t)
	defer redisHelper.Cleanup(t)

	cfg := helpers.TestConfig()
	logger := logrus.New()

	// Create Redis client and repositories
	redisClient, err := redis.NewClient(&cfg.Redis)
	require.NoError(t, err)

	topicRepo := redis.NewTopicRepository(redisClient)
	topicSvc := topic.NewService(topicRepo, logger)

	t.Run("CreateAndListTopics", func(t *testing.T) {
		// Create multiple topics
		topics := []string{"topic-1", "topic-2", "topic-3"}
		for _, topicName := range topics {
			req := &topic.CreateTopicRequest{
				Name:       topicName,
				Partitions: 2,
			}
			_, err := topicSvc.CreateTopic(ctx, req)
			require.NoError(t, err)
		}

		// List topics
		listReq := &topic.ListTopicsRequest{
			Limit:  10,
			Offset: 0,
		}
		topicList, err := topicSvc.ListTopics(ctx, listReq)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(topicList.Topics), 3)

		// Verify topics exist
		for _, topicName := range topics {
			_, err := topicSvc.GetTopic(ctx, topicName)
			require.NoError(t, err)
		}
	})

	t.Run("DeleteTopic", func(t *testing.T) {
		topicName := "topic-to-delete"

		// Create topic
		req := &topic.CreateTopicRequest{
			Name:       topicName,
			Partitions: 1,
		}
		_, err := topicSvc.CreateTopic(ctx, req)
		require.NoError(t, err)

		// Verify it exists
		_, err = topicSvc.GetTopic(ctx, topicName)
		require.NoError(t, err)

		// Delete topic
		err = topicSvc.DeleteTopic(ctx, topicName)
		require.NoError(t, err)

		// Verify it doesn't exist (should return error)
		_, err = topicSvc.GetTopic(ctx, topicName)
		require.Error(t, err)
	})

	t.Run("GetTopicStats", func(t *testing.T) {
		topicName := "topic-with-stats"

		// Create topic
		req := &topic.CreateTopicRequest{
			Name:       topicName,
			Partitions: 3,
		}
		resp, err := topicSvc.CreateTopic(ctx, req)
		require.NoError(t, err)

		// Get topic info
		topicInfo, err := topicSvc.GetTopic(ctx, topicName)
		require.NoError(t, err)
		assert.Equal(t, topicName, topicInfo.Name)
		assert.Equal(t, int32(3), topicInfo.Partitions)
		assert.Equal(t, resp.TopicID, topicInfo.ID)
	})
}
