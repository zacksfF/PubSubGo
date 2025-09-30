# HTTP/WebSocket/cURL Usage Examples

This example demonstrates interacting with PubSubGo using HTTP REST API, WebSocket, and cURL commands.

## Features

- **HTTP REST API**: RESTful endpoints for all operations
- **WebSocket**: Real-time bidirectional communication
- **Server-Sent Events (SSE)**: One-way real-time updates
- **Interactive Dashboard**: Web-based UI for testing
- **Client Examples**: Python, Node.js, and browser clients

## Prerequisites

1. Start Redis:
```bash
docker run -d -p 6379:6379 redis:7-alpine
```

2. Start PubSubGo server:
```bash
cd ../..
make build-server
./bin/pubsubgo-server
```

## Running Examples

### 1. Interactive Web Dashboard

Open the HTML file in your browser:
```bash
open client.html
# Or
python3 -m http.server 8000
# Then navigate to http://localhost:8000/client.html
```

Features:
- Real-time WebSocket connection
- Publish messages via WebSocket or HTTP
- Subscribe to topics
- View message statistics
- cURL command examples

### 2. API Examples Script

```bash
chmod +x api-examples.sh
./api-examples.sh
```

This script demonstrates:
- Health checks
- Topic management
- Message publishing (single and batch)
- HTTP long polling
- SSE streaming
- Performance testing

### 3. WebSocket Testing

Install wscat:
```bash
npm install -g wscat
```

Connect to WebSocket:
```bash
wscat -c ws://localhost:8081/ws
```

Commands:
```json
# Subscribe
{"action":"subscribe","topic":"events","consumer_id":"ws-test"}

# Publish
{"action":"publish","topic":"events","payload":"Hello WebSocket"}

# Acknowledge
{"action":"ack","message_id":"msg_123"}

# Unsubscribe
{"action":"unsubscribe","topic":"events"}
```

## HTTP API Reference

### Health Check
```bash
curl http://localhost:8081/health
```

### Topic Management

```bash
# Create topic
curl -X POST http://localhost:8081/api/v1/topics \
  -H "Content-Type: application/json" \
  -d '{
    "name": "events",
    "partitions": 3,
    "retention": "24h"
  }'

# List topics
curl http://localhost:8081/api/v1/topics

# Get topic details
curl http://localhost:8081/api/v1/topics/events

# Delete topic
curl -X DELETE http://localhost:8081/api/v1/topics/events
```

### Publishing Messages

```bash
# Single message
curl -X POST http://localhost:8081/api/v1/topics/events/messages \
  -H "Content-Type: application/json" \
  -d '{
    "payload": "Event data",
    "headers": {"type": "user.signup"}
  }'

# Batch publish
curl -X POST http://localhost:8081/api/v1/topics/events/messages/batch \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {"payload": "Message 1"},
      {"payload": "Message 2"},
      {"payload": "Message 3"}
    ]
  }'
```

### Subscribing

```bash
# HTTP Long Polling
curl "http://localhost:8081/api/v1/topics/events/messages?consumer=curl&timeout=30s"

# With consumer group
curl "http://localhost:8081/api/v1/topics/events/messages?consumer=worker&group=processors"
```

### Server-Sent Events (SSE)

```bash
# Subscribe via SSE
curl -N http://localhost:8081/api/v1/topics/events/stream?consumer=sse-client
```

## Client Libraries

### Python Client

```python
import requests
import websocket
import json

# HTTP Client
class HTTPClient:
    def __init__(self, base_url="http://localhost:8081"):
        self.base_url = base_url
    
    def publish(self, topic, message):
        return requests.post(
            f"{self.base_url}/api/v1/topics/{topic}/messages",
            json={"payload": message}
        ).json()

# WebSocket Client
def on_message(ws, message):
    print(f"Received: {message}")

ws = websocket.WebSocketApp("ws://localhost:8081/ws",
                            on_message=on_message)
ws.run_forever()
```

### JavaScript Client

```javascript
// HTTP
fetch('http://localhost:8081/api/v1/topics/events/messages', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({payload: 'Hello from JavaScript'})
});

// WebSocket
const ws = new WebSocket('ws://localhost:8081/ws');
ws.onmessage = (event) => {
    console.log('Received:', JSON.parse(event.data));
};

// SSE
const eventSource = new EventSource('/api/v1/topics/events/stream');
eventSource.onmessage = (event) => {
    console.log('SSE:', JSON.parse(event.data));
};
```

### Go Client

```go
// HTTP
resp, _ := http.Post(
    "http://localhost:8081/api/v1/topics/events/messages",
    "application/json",
    bytes.NewBuffer([]byte(`{"payload":"Go message"}`)),
)

// WebSocket
conn, _, _ := websocket.DefaultDialer.Dial("ws://localhost:8081/ws", nil)
conn.WriteJSON(map[string]string{
    "action": "subscribe",
    "topic": "events",
})
```

## Performance Testing

### Using curl and GNU parallel
```bash
# Install parallel
brew install parallel

# Send 1000 messages in parallel
seq 1 1000 | parallel -j 50 \
  "curl -s -X POST http://localhost:8081/api/v1/topics/test/messages \
   -d '{\"payload\":\"Message {}\"}'"
```

### Using Apache Bench (ab)
```bash
# 10000 requests with 100 concurrent
ab -n 10000 -c 100 -p message.json -T application/json \
   http://localhost:8081/api/v1/topics/test/messages
```

### Using k6
```javascript
import http from 'k6/http';
import { check } from 'k6';

export default function() {
  const res = http.post('http://localhost:8081/api/v1/topics/test/messages', 
    JSON.stringify({payload: 'Load test'}),
    {headers: {'Content-Type': 'application/json'}}
  );
  check(res, {'status is 200': (r) => r.status === 200});
}
```

## Monitoring

### Metrics Endpoint
```bash
curl http://localhost:9092/metrics
```

### Key Metrics
- `pubsubgo_messages_published_total`: Total published messages
- `pubsubgo_messages_consumed_total`: Total consumed messages
- `pubsubgo_active_connections`: Active WebSocket connections
- `pubsubgo_request_duration_seconds`: Request latency histogram

## Troubleshooting

### Connection Refused
- Check if PubSubGo server is running: `curl http://localhost:8081/health`
- Verify Redis is running: `redis-cli ping`

### WebSocket Not Connecting
- Check browser console for errors
- Verify WebSocket endpoint: `ws://localhost:8081/ws`
- Check CORS settings if accessing from different origin

### Messages Not Received
- Verify topic exists: `curl http://localhost:8081/api/v1/topics`
- Check consumer group status
- Verify subscription is active