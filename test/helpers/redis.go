package helpers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// TestRedisClient creates a test Redis client
type TestRedisClient struct {
	Client *redis.Client
	DB     int
}

// NewTestRedis creates a new test Redis instance
func NewTestRedis(t *testing.T) *TestRedisClient {
	t.Helper()
	
	// Use a different DB for testing (default is 0, we use 15 for tests)
	testDB := 15
	
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       testDB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := client.Ping(ctx).Err()
	require.NoError(t, err, "Failed to connect to Redis")

	// Clear the test database
	err = client.FlushDB(ctx).Err()
	require.NoError(t, err, "Failed to flush test Redis DB")

	return &TestRedisClient{
		Client: client,
		DB:     testDB,
	}
}

// Cleanup cleans up the test Redis instance
func (r *TestRedisClient) Cleanup(t *testing.T) {
	t.Helper()
	
	ctx := context.Background()
	
	// Flush test database
	err := r.Client.FlushDB(ctx).Err()
	if err != nil {
		t.Logf("Warning: Failed to flush Redis test DB: %v", err)
	}
	
	// Close connection
	err = r.Client.Close()
	if err != nil {
		t.Logf("Warning: Failed to close Redis connection: %v", err)
	}
}

// SeedData seeds test data into Redis
func (r *TestRedisClient) SeedData(t *testing.T, data map[string]interface{}) {
	t.Helper()
	
	ctx := context.Background()
	pipe := r.Client.Pipeline()
	
	for key, value := range data {
		switch v := value.(type) {
		case string:
			pipe.Set(ctx, key, v, 0)
		case []byte:
			pipe.Set(ctx, key, v, 0)
		case map[string]interface{}:
			pipe.HMSet(ctx, key, v)
		default:
			pipe.Set(ctx, key, fmt.Sprintf("%v", v), 0)
		}
	}
	
	_, err := pipe.Exec(ctx)
	require.NoError(t, err, "Failed to seed test data")
}

// AssertKeyExists asserts that a key exists in Redis
func (r *TestRedisClient) AssertKeyExists(t *testing.T, key string) {
	t.Helper()
	
	ctx := context.Background()
	exists, err := r.Client.Exists(ctx, key).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), exists, "Key %s does not exist", key)
}

// AssertKeyValue asserts that a key has the expected value
func (r *TestRedisClient) AssertKeyValue(t *testing.T, key string, expected string) {
	t.Helper()
	
	ctx := context.Background()
	val, err := r.Client.Get(ctx, key).Result()
	require.NoError(t, err)
	require.Equal(t, expected, val, "Key %s has unexpected value", key)
}