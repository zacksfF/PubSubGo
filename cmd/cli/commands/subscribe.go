package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

type SubscribeMessage struct {
	Type      string          `json:"type"`
	ID        string          `json:"id,omitempty"`
	Topic     string          `json:"topic,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Error     string          `json:"error,omitempty"`
}

type SubscribePayload struct {
	Topic         string `json:"topic"`
	ConsumerGroup string `json:"consumer_group,omitempty"`
	Filter        string `json:"filter,omitempty"`
}

type MessagePayload struct {
	ID            string            `json:"id"`
	Key           string            `json:"key,omitempty"`
	Payload       string            `json:"payload"`
	Headers       map[string]string `json:"headers,omitempty"`
	Priority      string            `json:"priority"`
	DeliveryMode  string            `json:"delivery_mode"`
	Topic         string            `json:"topic"`
	Partition     int32             `json:"partition"`
	Offset        int64             `json:"offset"`
	ConsumerGroup string            `json:"consumer_group,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	ExpiresAt     *time.Time        `json:"expires_at,omitempty"`
}

func NewSubscribeCommand(serverURL *string, verbose *bool) *cobra.Command {
	var (
		topic         string
		consumerGroup string
		filter        string
		noAck         bool
	)

	cmd := &cobra.Command{
		Use:   "subscribe",
		Short: "Subscribe to messages from a topic",
		Long: `Subscribe to messages from a specified topic using WebSocket connection.

Examples:
  pubsub subscribe --topic events
  pubsub subscribe --topic notifications --consumer-group mobile-app
  pubsub subscribe --topic logs --filter "level=error"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if topic == "" {
				return fmt.Errorf("topic is required")
			}

			return subscribeToTopic(*serverURL, topic, consumerGroup, filter, noAck, *verbose)
		},
	}

	cmd.Flags().StringVar(&topic, "topic", "", "Topic to subscribe to (required)")
	cmd.Flags().StringVar(&consumerGroup, "consumer-group", "", "Consumer group name")
	cmd.Flags().StringVar(&filter, "filter", "", "Message filter expression")
	cmd.Flags().BoolVar(&noAck, "no-ack", false, "Don't send acknowledgments for received messages")

	cmd.MarkFlagRequired("topic")

	return cmd
}

func subscribeToTopic(serverURL, topic, consumerGroup, filter string, noAck, verbose bool) error {
	// Convert HTTP URL to WebSocket URL
	u, err := url.Parse(serverURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %v", err)
	}

	wsScheme := "ws"
	if u.Scheme == "https" {
		wsScheme = "wss"
	}

	wsURL := fmt.Sprintf("%s://%s/ws", wsScheme, u.Host)

	if verbose {
		fmt.Printf("Connecting to WebSocket: %s\n", wsURL)
	}

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	fmt.Printf("🔗 Connected to PubSubGo server\n")

	// Send subscription message
	subscribeMsg := SubscribeMessage{
		Type:  "subscribe",
		ID:    generateID(),
		Topic: topic,
		Payload: mustMarshal(SubscribePayload{
			Topic:         topic,
			ConsumerGroup: consumerGroup,
			Filter:        filter,
		}),
		Timestamp: time.Now(),
	}

	if err := conn.WriteJSON(subscribeMsg); err != nil {
		return fmt.Errorf("failed to send subscription: %v", err)
	}

	if verbose {
		fmt.Printf("Sent subscription: %+v\n", subscribeMsg)
	}

	fmt.Printf("📡 Subscribed to topic '%s'\n", topic)
	if consumerGroup != "" {
		fmt.Printf("👥 Consumer group: %s\n", consumerGroup)
	}
	if filter != "" {
		fmt.Printf("🔍 Filter: %s\n", filter)
	}
	fmt.Printf("⏳ Waiting for messages... (Press Ctrl+C to exit)\n\n")

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Printf("\n Shutting down...\n")
		cancel()
	}()

	// Message handling loop
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			var msg SubscribeMessage
			if err := conn.ReadJSON(&msg); err != nil {
				if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					fmt.Printf("📡 Connection closed\n")
					return nil
				}
				return fmt.Errorf("failed to read message: %v", err)
			}

			if verbose {
				fmt.Printf("Received: %+v\n", msg)
			}

			switch msg.Type {
			case "message":
				if err := handleMessage(conn, msg, noAck, verbose); err != nil {
					log.Printf("Error handling message: %v", err)
				}
			case "subscribed":
				fmt.Printf(" Subscription confirmed\n")
			case "error":
				fmt.Printf(" Error: %s\n", msg.Error)
			case "pong":
				if verbose {
					fmt.Printf(" Received pong\n")
				}
			default:
				if verbose {
					fmt.Printf("Unknown message type: %s\n", msg.Type)
				}
			}
		}
	}
}

func handleMessage(conn *websocket.Conn, msg SubscribeMessage, noAck, verbose bool) error {
	var payload MessagePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to parse message payload: %v", err)
	}

	// Display message
	fmt.Printf(" [%s] Message received:\n", time.Now().Format("15:04:05"))
	fmt.Printf("   ID: %s\n", payload.ID)
	fmt.Printf("   Topic: %s\n", payload.Topic)
	if payload.Key != "" {
		fmt.Printf("   Key: %s\n", payload.Key)
	}
	fmt.Printf("   Content: %s\n", payload.Payload)
	fmt.Printf("   Priority: %s\n", payload.Priority)
	fmt.Printf("   Partition: %d, Offset: %d\n", payload.Partition, payload.Offset)
	if len(payload.Headers) > 0 {
		fmt.Printf("   Headers: %v\n", payload.Headers)
	}
	fmt.Printf("   Created: %s\n", payload.CreatedAt.Format(time.RFC3339))
	fmt.Printf("\n")

	// Send acknowledgment
	if !noAck {
		ackMsg := SubscribeMessage{
			Type: "ack",
			ID:   generateID(),
			Payload: mustMarshal(map[string]string{
				"message_id":     payload.ID,
				"consumer_group": payload.ConsumerGroup,
			}),
			Timestamp: time.Now(),
		}

		if err := conn.WriteJSON(ackMsg); err != nil {
			return fmt.Errorf("failed to send acknowledgment: %v", err)
		}

		if verbose {
			fmt.Printf(" Sent acknowledgment for message %s\n", payload.ID)
		}
	}

	return nil
}

func generateID() string {
	return fmt.Sprintf("cli-%d", time.Now().UnixNano())
}

func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
