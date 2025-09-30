#!/bin/bash

# PubSubGo CLI Usage Demo
# This script demonstrates various CLI operations for a real-time chat application

set -e

echo "🚀 PubSubGo CLI Demo - Real-time Chat Application"
echo "=================================================="
echo ""

# Build CLI if not exists
if [ ! -f "../../bin/pubsubgo-cli" ]; then
    echo "Building CLI..."
    cd ../..
    make build-cli
    cd examples/cli-usage
fi

CLI="../../bin/pubsubgo-cli"

# Function to print section headers
print_section() {
    echo ""
    echo "═══════════════════════════════════════════════════"
    echo "  $1"
    echo "═══════════════════════════════════════════════════"
    echo ""
}

# Function to execute and show command
run_cmd() {
    echo "$ $1"
    eval $1
    echo ""
}

print_section "1. SYSTEM HEALTH CHECK"
run_cmd "$CLI health"

print_section "2. CREATE CHAT TOPICS"

# Create topics for different chat channels
echo "Creating general chat topic..."
run_cmd "$CLI topics create --name general-chat --partitions 3"

echo "Creating tech discussion topic..."
run_cmd "$CLI topics create --name tech-chat --partitions 2"

echo "Creating announcements topic..."
run_cmd "$CLI topics create --name announcements --partitions 1"

print_section "3. LIST ALL TOPICS"
run_cmd "$CLI topics list --output table"

print_section "4. PUBLISH MESSAGES"

echo "Publishing welcome message..."
run_cmd "$CLI publish --topic general-chat --message 'Welcome to PubSubGo Chat! 👋' --header sender=system --header priority=high"

echo "Publishing tech discussion..."
run_cmd "$CLI publish --topic tech-chat --message 'Has anyone tried the new Go 1.21 features?' --header sender=alice --header timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)"

echo "Publishing announcement..."
run_cmd "$CLI publish --topic announcements --message 'Server maintenance scheduled for tonight at 10 PM' --header sender=admin --header severity=important"

# Publish multiple messages to simulate chat
echo ""
echo "Simulating chat conversation..."
messages=(
    "Hey everyone!"
    "How's the new PubSubGo system working?"
    "It's amazing! The latency is so low"
    "I love the consumer group feature"
    "Yeah, perfect for scaling our chat app"
)

senders=("bob" "alice" "charlie" "diana" "eve")

for i in "${!messages[@]}"; do
    sender=${senders[$i]}
    message=${messages[$i]}
    echo "[$sender]: $message"
    $CLI publish --topic general-chat \
        --message "$message" \
        --header sender=$sender \
        --header timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
        2>/dev/null
    sleep 0.5
done

print_section "5. GET TOPIC STATISTICS"
run_cmd "$CLI topics describe general-chat"

print_section "6. SUBSCRIBE TO MESSAGES"

echo "Starting message consumer in background..."
echo "This will consume messages from general-chat topic:"
echo ""

# Start consumer in background
$CLI subscribe --topic general-chat \
    --consumer cli-demo-consumer \
    --group chat-readers \
    --auto-ack \
    --output ./chat-messages.jsonl &

CONSUMER_PID=$!
echo "Consumer started with PID: $CONSUMER_PID"
sleep 2

# Publish more messages while consumer is running
echo ""
echo "Publishing messages while consumer is active..."
for i in {1..5}; do
    $CLI publish --topic general-chat \
        --message "Real-time message $i" \
        --header sender=system \
        --header index=$i \
        2>/dev/null
    sleep 0.5
done

# Wait a bit and then stop consumer
sleep 3
kill $CONSUMER_PID 2>/dev/null || true
echo "Consumer stopped."

print_section "7. BATCH OPERATIONS"

# Create batch message file
cat > batch-messages.json << EOF
[
  {"payload": "Batch message 1", "headers": {"sender": "batch-processor", "batch_id": "001"}},
  {"payload": "Batch message 2", "headers": {"sender": "batch-processor", "batch_id": "002"}},
  {"payload": "Batch message 3", "headers": {"sender": "batch-processor", "batch_id": "003"}}
]
EOF

echo "Publishing batch messages from file..."
run_cmd "$CLI publish --topic tech-chat --batch --file batch-messages.json"

print_section "8. MONITORING & METRICS"

echo "Getting system metrics..."
run_cmd "$CLI metrics"

echo "Getting topic-specific stats..."
run_cmd "$CLI stats --topic general-chat"

print_section "9. CONSUMER GROUPS"

echo "Listing consumer groups..."
run_cmd "$CLI consumer-groups list"

echo "Getting consumer group details..."
run_cmd "$CLI consumer-groups describe chat-readers"

print_section "10. ADVANCED FEATURES"

echo "Testing message filtering (if supported)..."
$CLI subscribe --topic general-chat \
    --consumer filtered-consumer \
    --filter 'headers.sender == "alice"' \
    --max-messages 5 \
    --timeout 5s \
    2>/dev/null || echo "Filtering not supported in current version"

echo ""
echo "Testing message replay from offset..."
$CLI subscribe --topic general-chat \
    --consumer replay-consumer \
    --from-beginning \
    --max-messages 3 \
    --timeout 3s \
    2>/dev/null || echo "Replay not supported in current version"

print_section "11. CLEANUP (Optional)"

read -p "Do you want to delete the created topics? (y/n) " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Deleting topics..."
    run_cmd "$CLI topics delete general-chat --force"
    run_cmd "$CLI topics delete tech-chat --force"
    run_cmd "$CLI topics delete announcements --force"
    echo "Cleanup complete!"
else
    echo "Topics retained for further testing."
fi

print_section "✅ DEMO COMPLETE!"

echo "Key Features Demonstrated:"
echo "  • Topic management (create, list, describe, delete)"
echo "  • Message publishing (single and batch)"
echo "  • Message consumption (subscribe with consumer groups)"
echo "  • Real-time chat simulation"
echo "  • Monitoring and metrics"
echo "  • Consumer group management"
echo ""
echo "Check the generated files:"
echo "  • chat-messages.jsonl - Contains consumed messages"
echo "  • batch-messages.json - Batch message input file"
echo ""
echo "For more CLI options, run: $CLI --help"