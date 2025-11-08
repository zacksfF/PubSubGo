package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

type PublishRequest struct {
	Topic        string            `json:"topic"`
	Key          string            `json:"key,omitempty"`
	Payload      string            `json:"payload"`
	Headers      map[string]string `json:"headers,omitempty"`
	Priority     string            `json:"priority,omitempty"`
	DeliveryMode string            `json:"delivery_mode,omitempty"`
}

type PublishResponse struct {
	MessageID   string    `json:"message_id"`
	Topic       string    `json:"topic"`
	Partition   int32     `json:"partition"`
	Offset      int64     `json:"offset"`
	Timestamp   time.Time `json:"timestamp"`
	Compressed  bool      `json:"compressed,omitempty"`
}

func NewPublishCommand(serverURL *string, verbose *bool) *cobra.Command {
	var (
		topic        string
		message      string
		key          string
		priority     string
		deliveryMode string
		headers      []string
	)

	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish a message to a topic",
		Long: `Publish a message to a specified topic.

Examples:
  pubsub publish --topic events --message "Hello World"
  pubsub publish --topic notifications --message "Alert" --key user123 --priority high
  pubsub publish --topic logs --message "Error occurred" --headers "source=app1,level=error"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if topic == "" {
				return fmt.Errorf("topic is required")
			}
			if message == "" {
				return fmt.Errorf("message is required")
			}

			// Parse headers
			headerMap := make(map[string]string)
			for _, h := range headers {
				if err := parseHeader(h, headerMap); err != nil {
					return fmt.Errorf("invalid header format '%s': %v", h, err)
				}
			}

			// Create publish request
			req := PublishRequest{
				Topic:        topic,
				Key:          key,
				Payload:      message,
				Headers:      headerMap,
				Priority:     priority,
				DeliveryMode: deliveryMode,
			}

			return publishMessage(*serverURL, req, *verbose)
		},
	}

	cmd.Flags().StringVar(&topic, "topic", "", "Topic to publish to (required)")
	cmd.Flags().StringVar(&message, "message", "", "Message content to publish (required)")
	cmd.Flags().StringVar(&key, "key", "", "Message key for partitioning")
	cmd.Flags().StringVar(&priority, "priority", "normal", "Message priority (low, normal, high, critical)")
	cmd.Flags().StringVar(&deliveryMode, "delivery-mode", "at_least_once", "Delivery mode (at_most_once, at_least_once, exactly_once)")
	cmd.Flags().StringArrayVar(&headers, "headers", nil, "Message headers in key=value format (can be used multiple times)")

	cmd.MarkFlagRequired("topic")
	cmd.MarkFlagRequired("message")

	return cmd
}

func publishMessage(serverURL string, req PublishRequest, verbose bool) error {
	url := fmt.Sprintf("%s/api/v1/publish", serverURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}

	if verbose {
		fmt.Printf("Publishing to: %s\n", url)
		fmt.Printf("Request: %s\n", string(jsonData))
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	if verbose {
		fmt.Printf("Response Status: %s\n", resp.Status)
		fmt.Printf("Response Body: %s\n", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var publishResp PublishResponse
	if err := json.Unmarshal(body, &publishResp); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	// Display result
	fmt.Printf(" Message published successfully!\n")
	fmt.Printf("Message ID: %s\n", publishResp.MessageID)
	fmt.Printf("Topic: %s\n", publishResp.Topic)
	fmt.Printf("Partition: %d\n", publishResp.Partition)
	fmt.Printf("Offset: %d\n", publishResp.Offset)
	fmt.Printf("Timestamp: %s\n", publishResp.Timestamp.Format(time.RFC3339))
	if publishResp.Compressed {
		fmt.Printf("Compressed: Yes\n")
	}

	return nil
}

func parseHeader(header string, headerMap map[string]string) error {
	// Parse key=value format
	for i, char := range header {
		if char == '=' {
			if i == 0 || i == len(header)-1 {
				return fmt.Errorf("invalid format, expected key=value")
			}
			key := header[:i]
			value := header[i+1:]
			headerMap[key] = value
			return nil
		}
	}
	return fmt.Errorf("invalid format, expected key=value")
}
