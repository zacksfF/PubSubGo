# CLI Usage Example

This example demonstrates using the PubSubGo CLI tool to build a real-time chat application.

## Features Demonstrated

- **Topic Management**: Create, list, describe, and delete topics
- **Message Publishing**: Single and batch message publishing
- **Message Subscription**: Real-time message consumption with consumer groups
- **Monitoring**: Health checks, metrics, and statistics
- **Advanced Features**: Filtering, replay, and consumer group management

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

## Running the Demo

```bash
cd examples/cli-usage
chmod +x demo.sh
./demo.sh
```

## What the Demo Does

1. **Health Check**: Verifies the PubSubGo server is running
2. **Topic Creation**: Creates chat channels (general, tech, announcements)
3. **Message Publishing**: Simulates chat messages from different users
4. **Subscription**: Starts a consumer to receive messages
5. **Batch Operations**: Publishes multiple messages at once
6. **Monitoring**: Shows metrics and statistics
7. **Consumer Groups**: Demonstrates load balancing across consumers

## CLI Commands Reference

### Topic Management

```bash
# Create topic
pubsubgo-cli topics create --name <topic> --partitions <n>

# List all topics
pubsubgo-cli topics list

# Get topic details
pubsubgo-cli topics describe <topic>

# Delete topic
pubsubgo-cli topics delete <topic>
```

### Publishing Messages

```bash
# Simple publish
pubsubgo-cli publish --topic <topic> --message "Hello World"

# With headers
pubsubgo-cli publish --topic <topic> \
    --message "Message" \
    --header key1=value1 \
    --header key2=value2

# From file
pubsubgo-cli publish --topic <topic> --file message.json

# Batch publish
pubsubgo-cli publish --topic <topic> --batch --file batch.json
```

### Subscribing to Messages

```bash
# Basic subscription
pubsubgo-cli subscribe --topic <topic> --consumer <id>

# With consumer group
pubsubgo-cli subscribe --topic <topic> \
    --consumer <id> \
    --group <group-name>

# With auto-acknowledge
pubsubgo-cli subscribe --topic <topic> \
    --consumer <id> \
    --auto-ack

# Save to file
pubsubgo-cli subscribe --topic <topic> \
    --consumer <id> \
    --output messages.jsonl

# Limited messages
pubsubgo-cli subscribe --topic <topic> \
    --consumer <id> \
    --max-messages 10
```

### Monitoring

```bash
# Health check
pubsubgo-cli health

# Get metrics
pubsubgo-cli metrics

# Topic statistics
pubsubgo-cli stats --topic <topic>

# Consumer groups
pubsubgo-cli consumer-groups list
pubsubgo-cli consumer-groups describe <group>
```

## Output Formats

The CLI supports multiple output formats:

```bash
# Table format (default)
pubsubgo-cli topics list --output table

# JSON format
pubsubgo-cli topics list --output json

# YAML format
pubsubgo-cli topics list --output yaml

# Plain text
pubsubgo-cli topics list --output plain
```

## Environment Variables

```bash
# Server endpoint
export PUBSUB_SERVER=http://localhost:8081

# Default timeout
export PUBSUB_TIMEOUT=30s

# Default consumer group
export PUBSUB_CONSUMER_GROUP=my-group
```

## Configuration File

Create `.pubsubgo.yaml` in your home directory:

```yaml
server: http://localhost:8081
timeout: 30s
output: table
consumer:
  group: default-group
  auto_ack: false
```

## Advanced Usage

### Message Filtering

```bash
# Subscribe with filter
pubsubgo-cli subscribe --topic orders \
    --filter 'amount > 100'
```

### Replay Messages

```bash
# From beginning
pubsubgo-cli subscribe --topic events \
    --from-beginning

# From specific offset
pubsubgo-cli subscribe --topic events \
    --from-offset 1000
```

### Performance Testing

```bash
# Benchmark publishing
pubsubgo-cli benchmark publish \
    --topic test \
    --messages 10000 \
    --size 1024 \
    --threads 10

# Benchmark consuming
pubsubgo-cli benchmark consume \
    --topic test \
    --consumers 5 \
    --duration 60s
```

## Troubleshooting

### Connection Issues
```bash
# Check server connectivity
pubsubgo-cli ping

# Verbose mode for debugging
pubsubgo-cli --verbose topics list
```

### Message Not Received
```bash
# Check consumer group lag
pubsubgo-cli consumer-groups describe <group>

# Check topic partitions
pubsubgo-cli topics describe <topic>
```