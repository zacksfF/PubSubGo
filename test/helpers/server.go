package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zacksfF/PubSubGo/internal/config"
)

// TestServer represents a test HTTP server
type TestServer struct {
	Server *httptest.Server
	Client *http.Client
}

// NewTestServer creates a new test HTTP server
func NewTestServer(handler http.Handler) *TestServer {
	server := httptest.NewServer(handler)
	
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	
	return &TestServer{
		Server: server,
		Client: client,
	}
}

// Close closes the test server
func (ts *TestServer) Close() {
	ts.Server.Close()
}

// Get performs a GET request
func (ts *TestServer) Get(t *testing.T, path string) *http.Response {
	t.Helper()
	
	resp, err := ts.Client.Get(ts.Server.URL + path)
	require.NoError(t, err)
	
	return resp
}

// Post performs a POST request with JSON body
func (ts *TestServer) Post(t *testing.T, path string, body interface{}) *http.Response {
	t.Helper()
	
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewBuffer(jsonBody)
	}
	
	req, err := http.NewRequest("POST", ts.Server.URL+path, bodyReader)
	require.NoError(t, err)
	
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	resp, err := ts.Client.Do(req)
	require.NoError(t, err)
	
	return resp
}

// Put performs a PUT request with JSON body
func (ts *TestServer) Put(t *testing.T, path string, body interface{}) *http.Response {
	t.Helper()
	
	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)
	
	req, err := http.NewRequest("PUT", ts.Server.URL+path, bytes.NewBuffer(jsonBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := ts.Client.Do(req)
	require.NoError(t, err)
	
	return resp
}

// Delete performs a DELETE request
func (ts *TestServer) Delete(t *testing.T, path string) *http.Response {
	t.Helper()
	
	req, err := http.NewRequest("DELETE", ts.Server.URL+path, nil)
	require.NoError(t, err)
	
	resp, err := ts.Client.Do(req)
	require.NoError(t, err)
	
	return resp
}

// AssertStatus asserts the response status code
func AssertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	require.Equal(t, expected, resp.StatusCode, "Unexpected status code")
}

// ParseJSON parses JSON response body
func ParseJSON(t *testing.T, resp *http.Response, target interface{}) {
	t.Helper()
	
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	
	err = json.Unmarshal(body, target)
	require.NoError(t, err, "Failed to parse JSON: %s", string(body))
}

// TestConfig creates a test configuration
func TestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Host:            "localhost",
			Port:            8080,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			IdleTimeout:     120 * time.Second,
			ShutdownTimeout: 30 * time.Second,
			EnableWebSocket: true,
		},
		Storage: config.StorageConfig{
			Type:            "memory",
			MaxMessageSize:  1048576,
			MaxQueueSize:    1000,
			RetentionTime:   24 * time.Hour,
			CleanupInterval: 1 * time.Hour,
		},
		Redis: config.RedisConfig{
			Addresses:    []string{"localhost:6379"},
			DB:           15, // Test database
			PoolSize:     10,
			MinIdleConns: 2,
		},
		Broker: config.BrokerConfig{
			DefaultPartitions:      2,
			MaxMessageBatchSize:    100,
			BatchFlushInterval:     100 * time.Millisecond,
			CompressionType:        "none",
			EnableDeduplication:    false,
			DefaultAckDeadline:     30 * time.Second,
			MaxRetries:             3,
			RetryBackoff:           1 * time.Second,
			EnableDLQ:              true,
			DLQPrefix:              "dlq-test-",
		},
		Metrics: config.MetricsConfig{
			Enabled:         false,
			Port:            9091,
			Path:            "/metrics",
			CollectInterval: 10 * time.Second,
		},
		Tracing: config.TracingConfig{
			Enabled:      false,
			ServiceName:  "pubsubgo-test",
			OTLPEndpoint: "http://localhost:4318/v1/traces",
			SamplingRate: 1.0,
		},
	}
}

// WaitForServer waits for a server to become available
func WaitForServer(url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("server did not become available within %v", timeout)
		case <-ticker.C:
			resp, err := http.Get(url + "/health")
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return nil
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
	}
}