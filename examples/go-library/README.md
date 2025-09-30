# Go Library Usage Example

This example demonstrates using PubSubGo as a Go library to build an e-commerce order processing system.

## Architecture

```
┌─────────────┐     ┌────────────┐     ┌─────────────────┐     ┌──────────────────┐
│   Orders    │────▶│  PubSubGo  │────▶│ Order Processor │────▶│     Payments     │
└─────────────┘     └────────────┘     └─────────────────┘     └──────────────────┘
                           │                     │                        │
                           │                     │                        ▼
                           │                     │              ┌──────────────────┐
                           │                     └─────────────▶│    Inventory     │
                           │                                    └──────────────────┘
                           │                                              
                           ▼                                              
                    ┌──────────────┐                                     
                    │ Notifications │◀────────────────────────────────────
                    └──────────────┘                                     
```

## Features Demonstrated

- **Topic Creation**: Creates multiple topics for different domains
- **Message Publishing**: Publishes order events with headers and metadata
- **Consumer Groups**: Multiple consumers processing messages in parallel
- **Event-Driven Architecture**: Orders trigger payments and notifications
- **Error Handling**: Graceful handling of failures
- **Message Acknowledgment**: Ensures reliable message processing

## Running the Example

### Prerequisites

1. Start Redis:
```bash
docker run -d -p 6379:6379 redis:7-alpine
```

2. Start PubSubGo server:
```bash
make build-server
./bin/pubsubgo-server
```

### Run the Example

```bash
cd examples/go-library
go run main.go
```

## Expected Output

```
🚀 Starting E-Commerce Order Processing System
==================================================
✅ Created topic: orders with 3 partitions
✅ Created topic: payments with 2 partitions
✅ Created topic: notifications with 1 partitions
✅ Created topic: inventory with 2 partitions

📦 Starting Order Simulation...
👷 Order Processor started...
💳 Payment Processor started...
📧 Notification Service started...

📤 Published order ORD-a1b2c3d4 (Amount: $150.00) - Message ID: msg_123
📥 Processing order ORD-a1b2c3d4 from customer CUST-001 ($150.00)
💰 Processing payment for order ORD-a1b2c3d4 ($150.00)
✅ Payment successful for order ORD-a1b2c3d4
📮 Sending notification to CUST-001: Order ORD-a1b2c3d4: PAID
✉️  Notification sent for order ORD-a1b2c3d4
...
```

## Code Structure

- **Main Function**: Initializes PubSubGo and coordinates the system
- **Order Simulation**: Generates sample orders
- **Order Processor**: Processes incoming orders and triggers payments
- **Payment Processor**: Handles payment processing with simulated success/failure
- **Notification Service**: Sends notifications based on order status

## Key Concepts

### Publisher Service
```go
pubService.Publish(ctx, &publisher.PublishRequest{
    Topic:   "orders",
    Payload: orderJSON,
    Headers: headers,
})
```

### Subscriber Service
```go
resp, err := subService.Subscribe(ctx, &subscriber.SubscribeRequest{
    Topic:         "orders",
    ConsumerID:    "processor-1",
    ConsumerGroup: "processors",
})
```

### Message Acknowledgment
```go
subService.Acknowledge(ctx, &subscriber.AckRequest{
    MessageID:  msg.ID,
    ConsumerID: consumerID,
})
```

## Configuration

The example uses the default `config.yaml` in the project root. You can customize:

- Redis connection settings
- Message retention
- Batch sizes
- Compression settings