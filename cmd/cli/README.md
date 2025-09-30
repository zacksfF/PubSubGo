# PubSubGo CLI

A command-line client for interacting with PubSubGo message broker.

## Installation

```bash
go build -o pubsub-cli ./cmd/cli
```

## Usage

### Basic Commands

```bash
# Show help
./pubsub-cli --help

# Check version
./pubsub-cli version

# Connect to different server
./pubsub-cli --server http://localhost:8080 <command>
```

### Publishing Messages

```bash
# Publish a simple message
./pubsub-cli publish --topic events --message "Hello World"

# Publish with key and priority
./pubsub-cli publish --topic notifications --message "Alert!" --key user123 --priority high

# Publish with headers
./pubsub-cli publish --topic logs --message "Error occurred" --headers "source=app1" --headers "level=error"
```

### Subscribing to Messages

```bash
# Subscribe to a topic
./pubsub-cli subscribe --topic events

# Subscribe with consumer group
./pubsub-cli subscribe --topic notifications --consumer-group mobile-app

# Subscribe without sending acknowledgments
./pubsub-cli subscribe --topic logs --no-ack
```

### Topic Management

```bash
# List all topics
./pubsub-cli topics list

# Create a topic
./pubsub-cli topics create --name events --partitions 4

# Get topic statistics
./pubsub-cli topics stats --name events

# Delete a topic
./pubsub-cli topics delete --name events
```

## Examples

### Quick Test

1. Start the PubSubGo server:
```bash
./pubsubgo-server -config config.yaml
```

2. In another terminal, create a topic:
```bash
./pubsub-cli topics create --name test-topic
```

3. Subscribe to messages (keep this running):
```bash
./pubsub-cli subscribe --topic test-topic
```

4. In another terminal, publish a message:
```bash
./pubsub-cli publish --topic test-topic --message "Hello from CLI!"
```

You should see the message appear in the subscriber terminal.

## Options

- `--server`: PubSubGo server URL (default: http://localhost:8080)
- `--verbose`: Enable verbose output for debugging
- `--help`: Show help for any command

## Message Format

Published messages support:
- **Topic**: Required destination topic
- **Message**: Required message content
- **Key**: Optional partitioning key
- **Priority**: low, normal, high, critical
- **Headers**: Key-value pairs in format `key=value`
- **Delivery Mode**: at_most_once, at_least_once, exactly_once