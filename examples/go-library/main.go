package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// PubSubClient wraps HTTP API calls to PubSubGo
type PubSubClient struct {
	baseURL string
	client  *http.Client
}

// NewClient creates a new PubSubGo HTTP client
func NewClient(baseURL string) *PubSubClient {
	return &PubSubClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// CreateTopic creates a new topic
func (c *PubSubClient) CreateTopic(name string, partitions int) error {
	payload := map[string]interface{}{
		"name":       name,
		"partitions": partitions,
		"retention":  "24h",
	}
	
	jsonData, _ := json.Marshal(payload)
	resp, err := c.client.Post(c.baseURL+"/v1/topics", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("failed to create topic, status: %d", resp.StatusCode)
	}
	return nil
}

// PublishMessage publishes a message to a topic
func (c *PubSubClient) PublishMessage(topic string, payload interface{}, headers map[string]string) error {
	message := map[string]interface{}{
		"payload": payload,
		"headers": headers,
	}
	
	jsonData, _ := json.Marshal(message)
	resp, err := c.client.Post(
		c.baseURL+"/v1/publish/"+topic,
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to publish message, status: %d", resp.StatusCode)
	}
	return nil
}

// OrderEvent represents an order in our e-commerce system
type OrderEvent struct {
	OrderID    string    `json:"order_id"`
	CustomerID string    `json:"customer_id"`
	Amount     float64   `json:"amount"`
	Status     string    `json:"status"`
	Timestamp  time.Time `json:"timestamp"`
}

func main() {
	log.Println(" Starting E-Commerce Order Processing System (HTTP Client)")
	log.Println("=" + string(make([]byte, 60)))

	// Initialize PubSubGo HTTP client
	client := NewClient("http://localhost:8081")

	// Create topics
	createTopics(client)

	// Simulate order flow
	simulateOrderFlow(client)

	log.Println("\n E-Commerce order processing demo completed!")
	log.Println("Check your PubSubGo monitoring dashboard for metrics.")
}

func createTopics(client *PubSubClient) {
	topics := []struct {
		name       string
		partitions int
	}{
		{"orders", 3},
		{"payments", 2},
		{"notifications", 1},
		{"inventory", 2},
	}

	log.Println("\n Creating Topics...")
	for _, topic := range topics {
		err := client.CreateTopic(topic.name, topic.partitions)
		if err != nil {
			log.Printf("Topic %s might already exist: %v", topic.name, err)
		} else {
			log.Printf(" Created topic: %s with %d partitions", topic.name, topic.partitions)
		}
	}
}

func simulateOrderFlow(client *PubSubClient) {
	log.Println("\n Starting Order Simulation...")
	
	// Simulate 10 orders
	for i := 1; i <= 10; i++ {
		order := OrderEvent{
			OrderID:    fmt.Sprintf("ORD-%s", uuid.New().String()[:8]),
			CustomerID: fmt.Sprintf("CUST-%03d", i%3+1),
			Amount:     float64(100 + i*50),
			Status:     "NEW",
			Timestamp:  time.Now(),
		}

		// Publish order event
		headers := map[string]string{
			"event_type": "order.created",
			"source":     "order-service",
			"priority":   getPriority(order.Amount),
		}

		err := client.PublishMessage("orders", order, headers)
		if err != nil {
			log.Printf(" Failed to publish order %s: %v", order.OrderID, err)
		} else {
			log.Printf(" Published order %s (Amount: $%.2f)", order.OrderID, order.Amount)
		}

		// Simulate payment processing
		if order.Amount > 200 {
			payment := map[string]interface{}{
				"order_id": order.OrderID,
				"amount":   order.Amount,
				"method":   "credit_card",
			}

			err = client.PublishMessage("payments", payment, map[string]string{
				"event_type": "payment.requested",
				"order_id":   order.OrderID,
			})

			if err != nil {
				log.Printf(" Failed to publish payment for %s: %v", order.OrderID, err)
			} else {
				log.Printf(" Published payment request for order %s", order.OrderID)
			}
		}

		// Simulate inventory check
		inventory := map[string]interface{}{
			"order_id": order.OrderID,
			"items":    []string{"item_1", "item_2"},
			"action":   "reserve",
		}

		err = client.PublishMessage("inventory", inventory, map[string]string{
			"event_type": "inventory.check",
			"order_id":   order.OrderID,
		})

		if err != nil {
			log.Printf(" Failed to publish inventory check for %s: %v", order.OrderID, err)
		}

		// Simulate notification
		notification := map[string]interface{}{
			"customer_id": order.CustomerID,
			"message":     fmt.Sprintf("Your order %s has been received", order.OrderID),
			"channel":     "email",
		}

		err = client.PublishMessage("notifications", notification, map[string]string{
			"event_type": "notification.send",
			"channel":    "email",
		})

		if err != nil {
			log.Printf(" Failed to send notification for %s: %v", order.OrderID, err)
		} else {
			log.Printf(" Sent notification for order %s", order.OrderID)
		}

		time.Sleep(1 * time.Second)
	}
}

func getPriority(amount float64) string {
	if amount > 1000 {
		return "high"
	} else if amount > 500 {
		return "medium"
	}
	return "low"
}
