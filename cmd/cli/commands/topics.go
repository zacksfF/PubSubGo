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

type TopicInfo struct {
	Name          string                 `json:"name"`
	Partitions    int32                  `json:"partitions"`
	MessageCount  int64                  `json:"message_count"`
	RetentionTime time.Duration          `json:"retention_time"`
	CreatedAt     time.Time              `json:"created_at"`
	Config        map[string]interface{} `json:"config,omitempty"`
}

type CreateTopicRequest struct {
	Name          string                 `json:"name"`
	Partitions    int32                  `json:"partitions,omitempty"`
	Replication   int32                  `json:"replication,omitempty"`
	RetentionTime string                 `json:"retention_time,omitempty"`
	MaxSize       int64                  `json:"max_size,omitempty"`
	Config        map[string]interface{} `json:"config,omitempty"`
}

type TopicStats struct {
	Topic        string    `json:"topic"`
	Partitions   int32     `json:"partitions"`
	Messages     int64     `json:"messages"`
	Consumers    int64     `json:"consumers"`
	Rate         string    `json:"rate"`
	Size         string    `json:"size"`
	BytesIn      int64     `json:"bytes_in"`
	BytesOut     int64     `json:"bytes_out"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func NewTopicsCommand(serverURL *string, verbose *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "topics",
		Short: "Manage topics",
		Long:  "Create, list, and manage topics in PubSubGo",
	}

	// Add subcommands
	cmd.AddCommand(newListTopicsCommand(serverURL, verbose))
	cmd.AddCommand(newCreateTopicCommand(serverURL, verbose))
	cmd.AddCommand(newDeleteTopicCommand(serverURL, verbose))
	cmd.AddCommand(newTopicStatsCommand(serverURL, verbose))

	return cmd
}

func newListTopicsCommand(serverURL *string, verbose *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all topics",
		Long:  "List all available topics in the PubSubGo server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listTopics(*serverURL, *verbose)
		},
	}
}

func newCreateTopicCommand(serverURL *string, verbose *bool) *cobra.Command {
	var (
		topicName     string
		partitions    int32
		replication   int32
		retentionTime string
		maxSize       int64
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new topic",
		Long: `Create a new topic with specified configuration.

Examples:
  pubsub topics create --name events --partitions 4
  pubsub topics create --name logs --partitions 1 --retention 168h`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if topicName == "" {
				return fmt.Errorf("topic name is required")
			}

			req := CreateTopicRequest{
				Name:          topicName,
				Partitions:    partitions,
				Replication:   replication,
				RetentionTime: retentionTime,
				MaxSize:       maxSize,
			}

			return createTopic(*serverURL, req, *verbose)
		},
	}

	cmd.Flags().StringVar(&topicName, "name", "", "Topic name (required)")
	cmd.Flags().Int32Var(&partitions, "partitions", 1, "Number of partitions")
	cmd.Flags().Int32Var(&replication, "replication", 1, "Replication factor")
	cmd.Flags().StringVar(&retentionTime, "retention", "24h", "Message retention time (e.g., 24h, 7d)")
	cmd.Flags().Int64Var(&maxSize, "max-size", 0, "Maximum topic size in bytes")

	cmd.MarkFlagRequired("name")

	return cmd
}

func newDeleteTopicCommand(serverURL *string, verbose *bool) *cobra.Command {
	var topicName string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a topic",
		Long:  "Delete a topic and all its messages",
		RunE: func(cmd *cobra.Command, args []string) error {
			if topicName == "" {
				return fmt.Errorf("topic name is required")
			}

			return deleteTopic(*serverURL, topicName, *verbose)
		},
	}

	cmd.Flags().StringVar(&topicName, "name", "", "Topic name to delete (required)")
	cmd.MarkFlagRequired("name")

	return cmd
}

func newTopicStatsCommand(serverURL *string, verbose *bool) *cobra.Command {
	var topicName string

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Get topic statistics",
		Long:  "Get detailed statistics for a specific topic",
		RunE: func(cmd *cobra.Command, args []string) error {
			if topicName == "" {
				return fmt.Errorf("topic name is required")
			}

			return getTopicStats(*serverURL, topicName, *verbose)
		},
	}

	cmd.Flags().StringVar(&topicName, "name", "", "Topic name (required)")
	cmd.MarkFlagRequired("name")

	return cmd
}

func listTopics(serverURL string, verbose bool) error {
	url := fmt.Sprintf("%s/api/v1/topics", serverURL)

	if verbose {
		fmt.Printf("Fetching topics from: %s\n", url)
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch topics: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	if verbose {
		fmt.Printf("Response: %s\n", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
	}

	var topics []TopicInfo
	if err := json.Unmarshal(body, &topics); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	if len(topics) == 0 {
		fmt.Printf(" No topics found\n")
		return nil
	}

	fmt.Printf(" Found %d topic(s):\n\n", len(topics))
	for _, topic := range topics {
		fmt.Printf(" %s\n", topic.Name)
		fmt.Printf("   Partitions: %d\n", topic.Partitions)
		fmt.Printf("   Messages: %d\n", topic.MessageCount)
		fmt.Printf("   Retention: %s\n", topic.RetentionTime)
		fmt.Printf("   Created: %s\n", topic.CreatedAt.Format(time.RFC3339))
		fmt.Printf("\n")
	}

	return nil
}

func createTopic(serverURL string, req CreateTopicRequest, verbose bool) error {
	url := fmt.Sprintf("%s/api/v1/topics", serverURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}

	if verbose {
		fmt.Printf("Creating topic at: %s\n", url)
		fmt.Printf("Request: %s\n", string(jsonData))
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create topic: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	if verbose {
		fmt.Printf("Response: %s\n", string(body))
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
	}

	fmt.Printf(" Topic '%s' created successfully!\n", req.Name)
	fmt.Printf("   Partitions: %d\n", req.Partitions)
	if req.RetentionTime != "" {
		fmt.Printf("   Retention: %s\n", req.RetentionTime)
	}

	return nil
}

func deleteTopic(serverURL, topicName string, verbose bool) error {
	url := fmt.Sprintf("%s/api/v1/topics/%s", serverURL, topicName)

	if verbose {
		fmt.Printf("Deleting topic: %s\n", url)
	}

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete topic: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	if verbose {
		fmt.Printf("Response: %s\n", string(body))
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
	}

	fmt.Printf(" Topic '%s' deleted successfully!\n", topicName)

	return nil
}

func getTopicStats(serverURL, topicName string, verbose bool) error {
	url := fmt.Sprintf("%s/api/v1/topics/%s/stats", serverURL, topicName)

	if verbose {
		fmt.Printf("Fetching stats from: %s\n", url)
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch topic stats: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	if verbose {
		fmt.Printf("Response: %s\n", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
	}

	var stats TopicStats
	if err := json.Unmarshal(body, &stats); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	fmt.Printf(" Statistics for topic '%s':\n\n", topicName)
	fmt.Printf("   Partitions: %d\n", stats.Partitions)
	fmt.Printf("   Messages: %d\n", stats.Messages)
	fmt.Printf("   Consumers: %d\n", stats.Consumers)
	fmt.Printf("   Message Rate: %s\n", stats.Rate)
	fmt.Printf("   Size: %s\n", stats.Size)
	fmt.Printf("   Bytes In: %d\n", stats.BytesIn)
	fmt.Printf("   Bytes Out: %d\n", stats.BytesOut)
	fmt.Printf("   Created: %s\n", stats.CreatedAt.Format(time.RFC3339))
	fmt.Printf("   Updated: %s\n", stats.UpdatedAt.Format(time.RFC3339))

	return nil
}
