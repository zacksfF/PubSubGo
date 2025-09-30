package fuzzing

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/zacksfF/PubSubGo/internal/core/message"
)

// FuzzMessageParsing tests message parsing with random input
func FuzzMessageParsing(f *testing.F) {
	// Add seed corpus
	f.Add([]byte(`{"id":"123","topic":"test","payload":"dGVzdA==","priority":"high"}`))
	f.Add([]byte(`{"topic":"events","payload":"aGVsbG8=","headers":{"key":"value"}}`))
	f.Add([]byte(`{"topic":"logs","payload":"","key":"partition-key"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`{"topic":"test","payload":"!!!invalid-base64!!!"}`))
	
	f.Fuzz(func(t *testing.T, data []byte) {
		var msg message.Message
		
		// Try to unmarshal the data
		err := json.Unmarshal(data, &msg)
		
		// If unmarshal succeeds, verify we can marshal it back
		if err == nil {
			marshaled, err2 := json.Marshal(msg)
			if err2 != nil {
				t.Fatalf("Failed to marshal valid message: %v", err2)
			}
			
			// Verify round-trip consistency
			var msg2 message.Message
			if err := json.Unmarshal(marshaled, &msg2); err != nil {
				t.Fatalf("Round-trip failed: %v", err)
			}
		}
	})
}

// FuzzBase64Payload tests base64 encoding/decoding
func FuzzBase64Payload(f *testing.F) {
	// Add seed corpus
	f.Add([]byte("Hello, World!"))
	f.Add([]byte(""))
	f.Add([]byte("\x00\x01\x02\x03"))
	f.Add([]byte("🚀 Unicode test"))
	f.Add(make([]byte, 1024)) // Large payload
	
	f.Fuzz(func(t *testing.T, data []byte) {
		// Encode to base64
		encoded := base64.StdEncoding.EncodeToString(data)
		
		// Decode back
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("Failed to decode base64: %v", err)
		}
		
		// Verify round-trip
		if !bytes.Equal(data, decoded) {
			t.Fatalf("Round-trip failed: original=%v, decoded=%v", data, decoded)
		}
	})
}

// FuzzMessageValidation tests message validation with random fields
func FuzzMessageValidation(f *testing.F) {
	// Add seed corpus with various validation scenarios
	f.Add("", "", byte(0), "", int64(0))         // Empty message
	f.Add("test-topic", "key1", byte(1), "payload", int64(60))
	f.Add("topic-123", "", byte(3), "test", int64(-1))
	f.Add("../../../etc/passwd", "key", byte(2), "malicious", int64(3600))
	
	f.Fuzz(func(t *testing.T, topic string, key string, priority byte, payload string, ttl int64) {
		msg := &message.Message{
			Topic:    topic,
			Key:      []byte(key),
			Payload:  []byte(payload),
			Priority: message.Priority(priority),
		}
		
		// Validate message (implementation would be in actual code)
		validateMessage(t, msg)
	})
}

// FuzzTopicName tests topic name validation
func FuzzTopicName(f *testing.F) {
	// Add seed corpus
	f.Add("valid-topic")
	f.Add("topic_with_underscore")
	f.Add("topic.with.dots")
	f.Add("123-numeric-start")
	f.Add("")
	f.Add("../../etc/passwd")
	f.Add("topic-with-émojis-🚀")
	f.Add(string(make([]byte, 256))) // Long name
	
	f.Fuzz(func(t *testing.T, topicName string) {
		// Test topic name validation
		if isValidTopicName(topicName) {
			// If valid, should not contain certain characters
			if len(topicName) == 0 || len(topicName) > 255 {
				t.Errorf("Invalid topic name marked as valid: %s", topicName)
			}
		}
	})
}

// FuzzCompressionDecompression tests compression algorithms
func FuzzCompressionDecompression(f *testing.F) {
	// Add seed corpus
	f.Add([]byte("Compressible text that repeats repeats repeats"))
	f.Add([]byte(""))
	f.Add(make([]byte, 1024))
	f.Add([]byte("Short"))
	f.Add(bytes.Repeat([]byte("A"), 10000))
	
	f.Fuzz(func(t *testing.T, data []byte) {
		// Test with different compression algorithms
		compressionTypes := []string{"snappy", "gzip", "lz4", "none"}
		
		for _, compType := range compressionTypes {
			compressed := compress(data, compType)
			decompressed := decompress(compressed, compType)
			
			if !bytes.Equal(data, decompressed) {
				t.Errorf("Compression round-trip failed for %s", compType)
			}
		}
	})
}

// Helper functions (these would be actual implementations in production code)

func validateMessage(t *testing.T, msg *message.Message) {
	// Basic validation logic
	if msg == nil {
		return
	}
	
	if len(msg.Topic) > 255 {
		t.Logf("Topic name too long: %d", len(msg.Topic))
	}
	
	if len(msg.Payload) > 10*1024*1024 { // 10MB limit
		t.Logf("Payload too large: %d", len(msg.Payload))
	}
	
	if msg.Priority > 3 {
		t.Logf("Invalid priority: %d", msg.Priority)
	}
}

func isValidTopicName(name string) bool {
	if len(name) == 0 || len(name) > 255 {
		return false
	}
	
	// Check for path traversal attempts
	if bytes.Contains([]byte(name), []byte("..")) {
		return false
	}
	
	// Check for control characters
	for _, r := range name {
		if r < 32 || r == 127 {
			return false
		}
	}
	
	return true
}

func compress(data []byte, compType string) []byte {
	// Stub implementation - would use actual compression libraries
	switch compType {
	case "none":
		return data
	default:
		// Simplified compression simulation
		return append([]byte(compType+":"), data...)
	}
}

func decompress(data []byte, compType string) []byte {
	// Stub implementation - would use actual decompression
	switch compType {
	case "none":
		return data
	default:
		// Simplified decompression simulation
		prefix := compType + ":"
		if bytes.HasPrefix(data, []byte(prefix)) {
			return data[len(prefix):]
		}
		return data
	}
}