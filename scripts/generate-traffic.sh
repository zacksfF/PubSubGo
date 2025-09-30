#!/bin/bash

# Generate traffic for PubSubGo monitoring demonstration

API_URL="http://localhost:8081"
echo "🚀 Generating traffic for PubSubGo monitoring..."

# Create topics
echo "Creating topics..."
curl -X POST $API_URL/topics -H "Content-Type: application/json" -d '{"name":"orders","partitions":3}' 2>/dev/null
curl -X POST $API_URL/topics -H "Content-Type: application/json" -d '{"name":"payments","partitions":2}' 2>/dev/null
curl -X POST $API_URL/topics -H "Content-Type: application/json" -d '{"name":"notifications","partitions":1}' 2>/dev/null

# Publish messages in a loop
echo "Publishing messages..."
for i in {1..100}; do
  # Orders topic
  curl -X POST $API_URL/topics/orders/publish -H "Content-Type: application/json" \
    -d "{\"payload\":\"Order #$i created\",\"headers\":{\"order_id\":\"$i\",\"customer\":\"customer_$((i % 10))\"}}" 2>/dev/null
  
  # Payments topic
  if [ $((i % 2)) -eq 0 ]; then
    curl -X POST $API_URL/topics/payments/publish -H "Content-Type: application/json" \
      -d "{\"payload\":\"Payment for order #$i\",\"headers\":{\"order_id\":\"$i\",\"amount\":\"$((100 + i * 10))\"}}" 2>/dev/null
  fi
  
  # Notifications topic
  if [ $((i % 3)) -eq 0 ]; then
    curl -X POST $API_URL/topics/notifications/publish -H "Content-Type: application/json" \
      -d "{\"payload\":\"Notification: Order #$i status update\",\"headers\":{\"type\":\"order_status\"}}" 2>/dev/null
  fi
  
  # Add some random delays
  sleep 0.$((RANDOM % 10))
  
  if [ $((i % 10)) -eq 0 ]; then
    echo "Published $i messages..."
  fi
done

echo "✅ Traffic generation complete!"
echo ""
echo "📊 Check metrics at:"
echo "  - Prometheus: http://localhost:9090"
echo "  - Grafana: http://localhost:3000"
echo "  - Jaeger: http://localhost:16686"