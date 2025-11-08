#!/bin/bash

# PubSubGo HTTP/WebSocket/cURL Examples
# This script demonstrates various ways to interact with PubSubGo using HTTP and WebSocket

set -e

API_URL="http://localhost:8081"

echo " PubSubGo HTTP/WebSocket/cURL Examples"
echo "========================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored headers
print_header() {
    echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}$1${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
}

# Function to execute and display curl commands
run_curl() {
    echo -e "${YELLOW}$ $1${NC}"
    eval $1
    echo ""
}

print_header "1. HEALTH CHECK"
run_curl "curl -s $API_URL/health | jq '.'"

print_header "2. CREATE TOPICS"

echo "Creating IoT sensors topic..."
run_curl "curl -s -X POST $API_URL/api/v1/topics \
  -H 'Content-Type: application/json' \
  -d '{
    \"name\": \"iot-sensors\",
    \"partitions\": 3,
    \"retention\": \"24h\",
    \"config\": {
      \"compression\": \"snappy\",
      \"max_message_size\": 1048576
    }
  }' | jq '.'"

echo "Creating alerts topic..."
run_curl "curl -s -X POST $API_URL/api/v1/topics \
  -H 'Content-Type: application/json' \
  -d '{
    \"name\": \"alerts\",
    \"partitions\": 1,
    \"retention\": \"7d\"
  }' | jq '.'"

print_header "3. LIST TOPICS"
run_curl "curl -s $API_URL/api/v1/topics | jq '.'"

print_header "4. PUBLISH MESSAGES - Single Message"

echo "Publishing sensor data..."
run_curl "curl -s -X POST $API_URL/api/v1/topics/iot-sensors/messages \
  -H 'Content-Type: application/json' \
  -H 'X-Message-Priority: normal' \
  -d '{
    \"payload\": {
      \"sensor_id\": \"temp-001\",
      \"temperature\": 22.5,
      \"humidity\": 45,
      \"timestamp\": \"'$(date -u +%Y-%m-%dT%H:%M:%SZ)'\"
    },
    \"headers\": {
      \"device_type\": \"temperature_sensor\",
      \"location\": \"room_1\"
    }
  }' | jq '.'"

echo "Publishing alert..."
run_curl "curl -s -X POST $API_URL/api/v1/topics/alerts/messages \
  -H 'Content-Type: application/json' \
  -H 'X-Message-Priority: high' \
  -d '{
    \"payload\": \"High temperature detected in room 1\",
    \"headers\": {
      \"severity\": \"warning\",
      \"source\": \"temp-001\",
      \"threshold\": \"25\"
    }
  }' | jq '.'"

print_header "5. BATCH PUBLISH"

echo "Publishing batch sensor readings..."
run_curl "curl -s -X POST $API_URL/api/v1/topics/iot-sensors/messages/batch \
  -H 'Content-Type: application/json' \
  -d '{
    \"messages\": [
      {
        \"payload\": {\"sensor_id\": \"temp-002\", \"temperature\": 21.0},
        \"headers\": {\"location\": \"room_2\"}
      },
      {
        \"payload\": {\"sensor_id\": \"temp-003\", \"temperature\": 23.5},
        \"headers\": {\"location\": \"room_3\"}
      },
      {
        \"payload\": {\"sensor_id\": \"humidity-001\", \"humidity\": 62},
        \"headers\": {\"location\": \"basement\"}
      }
    ]
  }' | jq '.'"

print_header "6. SUBSCRIBE (HTTP Long Polling)"

echo "Starting subscription in background (5 seconds)..."
(curl -s "$API_URL/api/v1/topics/iot-sensors/messages?consumer=curl-demo&timeout=5s" | jq '.' &)
SUBSCRIBE_PID=$!

# Publish messages while subscribed
sleep 1
echo "Publishing message while subscribed..."
curl -s -X POST $API_URL/api/v1/topics/iot-sensors/messages \
  -H 'Content-Type: application/json' \
  -d '{"payload": "Real-time sensor update"}' > /dev/null

wait $SUBSCRIBE_PID 2>/dev/null || true

print_header "7. TOPIC STATISTICS"
run_curl "curl -s $API_URL/api/v1/topics/iot-sensors/stats | jq '.'"

print_header "8. WEBSOCKET CONNECTION (using wscat)"

echo "To test WebSocket connection, install wscat:"
echo -e "${YELLOW}npm install -g wscat${NC}"
echo ""
echo "Then connect with:"
echo -e "${GREEN}wscat -c ws://localhost:8081/ws${NC}"
echo ""
echo "Example WebSocket commands:"
cat << 'EOF'
# Subscribe to topic
{"action":"subscribe","topic":"iot-sensors","consumer_id":"ws-client"}

# Publish message
{"action":"publish","topic":"iot-sensors","payload":"WebSocket message","headers":{"source":"wscat"}}

# Acknowledge message
{"action":"ack","message_id":"msg_123456"}

# Unsubscribe
{"action":"unsubscribe","topic":"iot-sensors"}
EOF

print_header "9. SERVER-SENT EVENTS (SSE)"

echo "To subscribe via SSE:"
echo -e "${GREEN}curl -N $API_URL/api/v1/topics/iot-sensors/stream?consumer=sse-client${NC}"
echo ""
echo "Or use JavaScript:"
cat << 'EOF'
const eventSource = new EventSource('/api/v1/topics/iot-sensors/stream?consumer=browser');
eventSource.onmessage = (event) => {
  console.log('Received:', JSON.parse(event.data));
};
EOF

print_header "10. PYTHON CLIENT EXAMPLE"

cat << 'EOF' > /tmp/pubsub_client.py
import requests
import json
import time

class PubSubClient:
    def __init__(self, base_url="http://localhost:8081"):
        self.base_url = base_url
        self.session = requests.Session()
    
    def publish(self, topic, message, headers=None):
        """Publish a message to a topic"""
        response = self.session.post(
            f"{self.base_url}/api/v1/topics/{topic}/messages",
            json={
                "payload": message,
                "headers": headers or {}
            }
        )
        return response.json()
    
    def subscribe(self, topic, consumer_id, timeout=5):
        """Subscribe and receive messages"""
        response = self.session.get(
            f"{self.base_url}/api/v1/topics/{topic}/messages",
            params={"consumer": consumer_id, "timeout": f"{timeout}s"}
        )
        if response.status_code == 200:
            return response.json()
        return None

# Example usage
if __name__ == "__main__":
    client = PubSubClient()
    
    # Publish
    result = client.publish("iot-sensors", {
        "sensor_id": "py-001",
        "value": 42.0,
        "timestamp": time.time()
    })
    print(f"Published: {result}")
    
    # Subscribe
    messages = client.subscribe("iot-sensors", "python-consumer", timeout=2)
    if messages:
        print(f"Received: {messages}")
EOF

echo "Python client example saved to /tmp/pubsub_client.py"
echo "Run with: python3 /tmp/pubsub_client.py"

print_header "11. NODE.JS CLIENT EXAMPLE"

cat << 'EOF' > /tmp/pubsub_client.js
const axios = require('axios');
const WebSocket = require('ws');

class PubSubClient {
    constructor(httpUrl = 'http://localhost:8081', wsUrl = 'ws://localhost:8081/ws') {
        this.httpUrl = httpUrl;
        this.wsUrl = wsUrl;
        this.ws = null;
    }
    
    // HTTP Methods
    async publish(topic, payload, headers = {}) {
        const response = await axios.post(
            `${this.httpUrl}/api/v1/topics/${topic}/messages`,
            { payload, headers }
        );
        return response.data;
    }
    
    async subscribe(topic, consumerId, timeout = 5) {
        const response = await axios.get(
            `${this.httpUrl}/api/v1/topics/${topic}/messages`,
            { params: { consumer: consumerId, timeout: `${timeout}s` } }
        );
        return response.data;
    }
    
    // WebSocket Methods
    connectWebSocket() {
        this.ws = new WebSocket(this.wsUrl);
        
        this.ws.on('open', () => {
            console.log('WebSocket connected');
        });
        
        this.ws.on('message', (data) => {
            console.log('Received:', JSON.parse(data));
        });
        
        return new Promise((resolve) => {
            this.ws.on('open', resolve);
        });
    }
    
    subscribeWS(topic, consumerId) {
        if (this.ws) {
            this.ws.send(JSON.stringify({
                action: 'subscribe',
                topic: topic,
                consumer_id: consumerId
            }));
        }
    }
}

// Example usage
(async () => {
    const client = new PubSubClient();
    
    // HTTP publish
    const result = await client.publish('iot-sensors', {
        sensor_id: 'node-001',
        value: 100
    });
    console.log('Published:', result);
    
    // WebSocket subscribe
    await client.connectWebSocket();
    client.subscribeWS('iot-sensors', 'node-consumer');
})();
EOF

echo "Node.js client example saved to /tmp/pubsub_client.js"
echo "Install dependencies: npm install axios ws"
echo "Run with: node /tmp/pubsub_client.js"

print_header "12. METRICS & MONITORING"

echo "Prometheus metrics endpoint:"
run_curl "curl -s http://localhost:9092/metrics | grep pubsubgo | head -20"

print_header "13. PERFORMANCE TEST"

echo "Simple load test with curl and parallel..."
echo -e "${YELLOW}Publishing 100 messages in parallel...${NC}"

seq 1 100 | parallel -j 10 "curl -s -X POST $API_URL/api/v1/topics/iot-sensors/messages \
  -H 'Content-Type: application/json' \
  -d '{\"payload\":\"Message {}\"}' > /dev/null 2>&1 && echo -n '.'" 2>/dev/null || \
echo "(Install GNU parallel for parallel testing: brew install parallel)"

echo -e "\n"

print_header " EXAMPLES COMPLETE!"

echo "Summary of demonstrated features:"
echo "  • Health check and monitoring"
echo "  • Topic creation and management"
echo "  • Single and batch message publishing"
echo "  • HTTP long polling subscription"
echo "  • WebSocket real-time messaging"
echo "  • Server-Sent Events (SSE)"
echo "  • Python and Node.js client examples"
echo "  • Performance testing"
echo ""
echo "For interactive testing, open client.html in your browser:"
echo -e "${GREEN}open examples/http-websocket/client.html${NC}"
