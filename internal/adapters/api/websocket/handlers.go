package websocket

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/zacksfF/PubSubGo/internal/core/message"
	"github.com/zacksfF/PubSubGo/internal/core/subscription"
	"github.com/zacksfF/PubSubGo/internal/services/publisher"
	"github.com/zacksfF/PubSubGo/internal/services/subscriber"
)

// handleSubscribe handles subscription requests
func (c *Connection) handleSubscribe(msg *WSMessage) {
	var payload SubscribePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendError("Invalid subscribe payload", err)
		return
	}

	// Validate payload
	if payload.Topic == "" {
		c.sendError("Topic is required for subscription", fmt.Errorf("empty topic"))
		return
	}

	// Check if already subscribed to this topic
	c.subsMutex.RLock()
	_, exists := c.subscriptions[payload.Topic]
	c.subsMutex.RUnlock()
	
	if exists {
		c.sendError("Already subscribed to topic", fmt.Errorf("topic: %s", payload.Topic))
		return
	}

	// Create subscription request
	subReq := &subscriber.SubscribeRequest{
		Topic:         payload.Topic,
		ConsumerGroup: payload.ConsumerGroup,
		Type:          subscription.TypePush, // WebSocket is push-based
		Filter:        payload.Filter,
	}

	// Subscribe through subscriber service
	ctx := context.Background()
	subResp, err := c.server.subscriberSvc.Subscribe(ctx, subReq)
	if err != nil {
		c.sendError("Subscription failed", err)
		return
	}

	// Create subscription object
	sub := &Subscription{
		ID:            subResp.SubscriptionID,
		ConnectionID:  c.ID,
		Topic:         payload.Topic,
		ConsumerGroup: payload.ConsumerGroup,
		Filter:        payload.Filter,
		CreatedAt:     time.Now(),
		MessageCount:  0,
	}

	// Add to connection subscriptions
	c.subsMutex.Lock()
	c.subscriptions[payload.Topic] = sub
	c.subsMutex.Unlock()

	// Add to server subscriptions
	c.server.subsMutex.Lock()
	if c.server.subscriptions[payload.Topic] == nil {
		c.server.subscriptions[payload.Topic] = make(map[string]*Subscription)
	}
	c.server.subscriptions[payload.Topic][c.ID] = sub
	c.server.subsMutex.Unlock()

	// Send confirmation
	response := &WSMessage{
		Type:      MsgTypeSubscribed,
		ID:        msg.ID,
		Topic:     payload.Topic,
		Timestamp: time.Now(),
	}

	responsePayload := map[string]interface{}{
		"subscription_id": sub.ID,
		"topic":          payload.Topic,
		"consumer_group": payload.ConsumerGroup,
	}
	responseData, _ := json.Marshal(responsePayload)
	response.Payload = responseData

	c.sendMessage(response)

	c.server.logger.WithField("connection_id", c.ID).
		WithField("topic", payload.Topic).
		Info("WebSocket subscription created")
}

// handleUnsubscribe handles unsubscription requests
func (c *Connection) handleUnsubscribe(msg *WSMessage) {
	var payload struct {
		Topic string `json:"topic"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendError("Invalid unsubscribe payload", err)
		return
	}

	if payload.Topic == "" {
		c.sendError("Topic is required for unsubscription", fmt.Errorf("empty topic"))
		return
	}

	// Get subscription
	c.subsMutex.RLock()
	sub, exists := c.subscriptions[payload.Topic]
	c.subsMutex.RUnlock()

	if !exists {
		c.sendError("Not subscribed to topic", fmt.Errorf("topic: %s", payload.Topic))
		return
	}

	// Unsubscribe through subscriber service
	ctx := context.Background()
	err := c.server.subscriberSvc.Unsubscribe(ctx, sub.ID)
	if err != nil {
		c.server.logger.WithError(err).Warn("Failed to unsubscribe from service")
		// Continue with local cleanup
	}

	// Remove from connection subscriptions
	c.subsMutex.Lock()
	delete(c.subscriptions, payload.Topic)
	c.subsMutex.Unlock()

	// Remove from server subscriptions
	c.server.subsMutex.Lock()
	if topicSubs, exists := c.server.subscriptions[payload.Topic]; exists {
		delete(topicSubs, c.ID)
		if len(topicSubs) == 0 {
			delete(c.server.subscriptions, payload.Topic)
		}
	}
	c.server.subsMutex.Unlock()

	// Send confirmation
	response := &WSMessage{
		Type:      MsgTypeUnsubscribed,
		ID:        msg.ID,
		Topic:     payload.Topic,
		Timestamp: time.Now(),
	}

	responsePayload := map[string]interface{}{
		"topic": payload.Topic,
	}
	responseData, _ := json.Marshal(responsePayload)
	response.Payload = responseData

	c.sendMessage(response)

	c.server.logger.WithField("connection_id", c.ID).
		WithField("topic", payload.Topic).
		Info("WebSocket subscription removed")
}

// handlePublish handles message publishing
func (c *Connection) handlePublish(msg *WSMessage) {
	var payload PublishPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendError("Invalid publish payload", err)
		return
	}

	// Validate payload
	if msg.Topic == "" {
		c.sendError("Topic is required for publishing", fmt.Errorf("empty topic"))
		return
	}

	if payload.Payload == "" {
		c.sendError("Message payload is required", fmt.Errorf("empty payload"))
		return
	}

	// Decode base64 payload
	messageData, err := base64.StdEncoding.DecodeString(payload.Payload)
	if err != nil {
		c.sendError("Invalid base64 payload", err)
		return
	}

	// Create publish request
	pubReq := &publisher.PublishRequest{
		Topic:        msg.Topic,
		Key:          []byte(payload.Key),
		Payload:      messageData,
		Headers:      payload.Headers,
		Priority:     c.parsePriority(payload.Priority),
		DeliveryMode: message.DeliveryAtLeastOnce, // Default for WebSocket
	}

	// Publish through publisher service
	ctx := context.Background()
	pubResp, err := c.server.publisherSvc.Publish(ctx, pubReq)
	if err != nil {
		c.sendError("Publish failed", err)
		return
	}

	// Send confirmation
	response := &WSMessage{
		Type:      MsgTypePublished,
		ID:        msg.ID,
		Topic:     msg.Topic,
		Timestamp: time.Now(),
	}

	responsePayload := map[string]interface{}{
		"message_id": pubResp.MessageID,
		"topic":      pubResp.Topic,
		"partition":  pubResp.Partition,
		"offset":     pubResp.Offset,
		"timestamp":  pubResp.Timestamp,
	}
	responseData, _ := json.Marshal(responsePayload)
	response.Payload = responseData

	c.sendMessage(response)

	c.server.logger.WithField("connection_id", c.ID).
		WithField("topic", msg.Topic).
		WithField("message_id", pubResp.MessageID).
		Info("WebSocket message published")
}

// handleAck handles message acknowledgments
func (c *Connection) handleAck(msg *WSMessage) {
	var payload struct {
		MessageID     string `json:"message_id"`
		ConsumerGroup string `json:"consumer_group,omitempty"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendError("Invalid ack payload", err)
		return
	}

	if payload.MessageID == "" {
		c.sendError("Message ID is required for acknowledgment", fmt.Errorf("empty message_id"))
		return
	}

	// Create acknowledgment request
	ackReq := &subscriber.AcknowledgeRequest{
		MessageID:     payload.MessageID,
		ConsumerID:    c.ID, // Use connection ID as consumer ID
		ConsumerGroup: payload.ConsumerGroup,
	}

	// Acknowledge through subscriber service
	ctx := context.Background()
	err := c.server.subscriberSvc.Acknowledge(ctx, ackReq)
	if err != nil {
		c.sendError("Acknowledgment failed", err)
		return
	}

	c.server.logger.WithField("connection_id", c.ID).
		WithField("message_id", payload.MessageID).
		Debug("WebSocket message acknowledged")
}

// handlePing handles ping messages
func (c *Connection) handlePing(msg *WSMessage) {
	// Send pong response
	response := &WSMessage{
		Type:      MsgTypePong,
		ID:        msg.ID,
		Timestamp: time.Now(),
	}

	c.sendMessage(response)
}

// Helper methods

// parsePriority converts string priority to message.Priority
func (c *Connection) parsePriority(priority string) message.Priority {
	switch priority {
	case "low":
		return message.PriorityLow
	case "high":
		return message.PriorityHigh
	case "critical":
		return message.PriorityCritical
	default:
		return message.PriorityNormal
	}
}

// formatPriority converts message.Priority to string
func (c *Connection) formatPriority(priority message.Priority) string {
	switch priority {
	case message.PriorityLow:
		return "low"
	case message.PriorityHigh:
		return "high"
	case message.PriorityCritical:
		return "critical"
	default:
		return "normal"
	}
}

// formatDeliveryMode converts message.DeliveryMode to string
func (c *Connection) formatDeliveryMode(mode message.DeliveryMode) string {
	switch mode {
	case message.DeliveryAtMostOnce:
		return "at_most_once"
	case message.DeliveryExactlyOnce:
		return "exactly_once"
	default:
		return "at_least_once"
	}
}

// DeliverMessage delivers a message to this WebSocket connection
func (c *Connection) DeliverMessage(msg *message.Message) {
	// Update subscription stats
	c.subsMutex.Lock()
	if sub, exists := c.subscriptions[msg.Topic]; exists {
		sub.MessageCount++
		now := time.Now()
		sub.LastMessage = &now
	}
	c.subsMutex.Unlock()

	// Create message payload
	messagePayload := &MessagePayload{
		ID:            msg.ID,
		Key:           base64.StdEncoding.EncodeToString(msg.Key),
		Payload:       base64.StdEncoding.EncodeToString(msg.Payload),
		Headers:       msg.Headers,
		Priority:      c.formatPriority(msg.Priority),
		DeliveryMode:  c.formatDeliveryMode(msg.DeliveryMode),
		Topic:         msg.Topic,
		Partition:     msg.Partition,
		Offset:        msg.Offset,
		ConsumerGroup: msg.ConsumerGroup,
		CreatedAt:     msg.CreatedAt,
		ExpiresAt:     msg.ExpiresAt,
	}

	// Send message
	wsMsg := &WSMessage{
		Type:      MsgTypeMessage,
		ID:        uuid.New().String(),
		Topic:     msg.Topic,
		Timestamp: time.Now(),
	}

	payload, err := json.Marshal(messagePayload)
	if err != nil {
		c.server.logger.WithError(err).Error("Failed to marshal message payload")
		return
	}
	wsMsg.Payload = payload

	c.sendMessage(wsMsg)
}