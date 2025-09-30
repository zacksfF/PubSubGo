package http

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zacksfF/PubSubGo/internal/core/message"
	"github.com/zacksfF/PubSubGo/internal/core/subscription"
	"github.com/zacksfF/PubSubGo/internal/services/subscriber"
)

// SubscribeRequest represents an HTTP subscription request
type SubscribeRequest struct {
	Topic           string            `json:"topic"`
	ConsumerGroup   string            `json:"consumer_group,omitempty"`
	DeliveryMode    string            `json:"delivery_mode,omitempty"` // "push", "pull"
	MaxMessages     int               `json:"max_messages,omitempty"`
	AckDeadline     *int64            `json:"ack_deadline,omitempty"` // seconds
	RetryPolicy     *RetryPolicy      `json:"retry_policy,omitempty"`
	DeadLetterTopic string            `json:"dead_letter_topic,omitempty"`
	Config          map[string]string `json:"config,omitempty"`
}

type RetryPolicy struct {
	MaxRetries    int    `json:"max_retries"`
	BackoffFactor int    `json:"backoff_factor,omitempty"`
	MaxBackoff    *int64 `json:"max_backoff,omitempty"` // seconds
}

// PullRequest represents an HTTP pull request
type PullRequest struct {
	MaxMessages int    `json:"max_messages,omitempty"`
	AckDeadline *int64 `json:"ack_deadline,omitempty"` // seconds
	WaitTimeout *int64 `json:"wait_timeout,omitempty"` // seconds
}

// ConsumerGroupPullRequest represents a consumer group pull request
type ConsumerGroupPullRequest struct {
	ConsumerGroup string `json:"consumer_group"`
	ConsumerID    string `json:"consumer_id"`
	MaxMessages   int    `json:"max_messages,omitempty"`
	AckDeadline   *int64 `json:"ack_deadline,omitempty"` // seconds
	WaitTimeout   *int64 `json:"wait_timeout,omitempty"` // seconds
}

// JoinGroupRequest represents a consumer group join request
type JoinGroupRequest struct {
	ConsumerGroup string            `json:"consumer_group"`
	ConsumerID    string            `json:"consumer_id"`
	Config        map[string]string `json:"config,omitempty"`
}

// MessageResponse represents an HTTP message response
type MessageResponse struct {
	ID            string            `json:"id"`
	Topic         string            `json:"topic"`
	Partition     int32             `json:"partition,omitempty"`
	Offset        int64             `json:"offset,omitempty"`
	Key           string            `json:"key,omitempty"` // Base64 encoded
	Payload       string            `json:"payload"`       // Base64 encoded
	Headers       map[string]string `json:"headers,omitempty"`
	Priority      string            `json:"priority"`
	DeliveryMode  string            `json:"delivery_mode"`
	ConsumerGroup string            `json:"consumer_group,omitempty"`
	ConsumerID    string            `json:"consumer_id,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	DeliveredAt   *time.Time        `json:"delivered_at,omitempty"`
	ExpiresAt     *time.Time        `json:"expires_at,omitempty"`
}

// PullResponse represents an HTTP pull response
type PullResponse struct {
	Messages   []MessageResponse `json:"messages"`
	HasMore    bool              `json:"has_more"`
	NextOffset *int64            `json:"next_offset,omitempty"`
	WaitTime   time.Duration     `json:"wait_time"`
	ReceivedAt time.Time         `json:"received_at"`
}

// setupSubscriberRoutes configures subscriber-related routes
func (s *Server) setupSubscriberRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/subscriptions", s.handleSubscriptionListOrCreate)
	mux.HandleFunc("/v1/subscriptions/", s.handleSubscriptionOperations)
	mux.HandleFunc("/v1/consumer-groups/", s.handleConsumerGroupOperations)
}

// handleSubscriptionListOrCreate handles subscription list and creation
func (s *Server) handleSubscriptionListOrCreate(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		s.handleCreateSubscription(w, r)
	default:
		s.writeErrorResponse(w, http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
	}
}

// handleSubscriptionOperations handles individual subscription operations
func (s *Server) handleSubscriptionOperations(w http.ResponseWriter, r *http.Request) {
	subscriptionID := extractSubscriptionIDFromPath(r.URL.Path)
	if subscriptionID == "" {
		s.writeErrorResponse(w, http.StatusBadRequest,
			fmt.Errorf("subscription ID is required"), "MISSING_SUBSCRIPTION_ID")
		return
	}

	switch r.Method {
	case "GET":
		s.handleGetSubscription(w, r, subscriptionID)
	case "DELETE":
		s.handleDeleteSubscription(w, r, subscriptionID)
	default:
		s.writeErrorResponse(w, http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
	}
}

// handleTopicSubscriberOperations handles topic-specific subscriber operations
func (s *Server) handleTopicSubscriberOperations(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	topic := extractTopicFromPath(path)

	if topic == "" {
		// This might be handled by publisher routes, skip
		return
	}

	if strings.HasSuffix(path, "/pull") {
		s.handlePullMessages(w, r, topic)
	} else if strings.Contains(path, "/subscriptions") {
		s.handleListTopicSubscriptions(w, r, topic)
	}
}

// handleConsumerGroupOperations handles consumer group operations
func (s *Server) handleConsumerGroupOperations(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	group := extractConsumerGroupFromPath(path)

	if group == "" {
		s.writeErrorResponse(w, http.StatusBadRequest,
			fmt.Errorf("consumer group is required"), "MISSING_CONSUMER_GROUP")
		return
	}

	if strings.HasSuffix(path, "/pull") {
		s.handleConsumerGroupPull(w, r, group)
	} else if strings.HasSuffix(path, "/join") {
		s.handleJoinConsumerGroup(w, r, group)
	} else if strings.HasSuffix(path, "/leave") {
		s.handleLeaveConsumerGroup(w, r, group)
	} else {
		s.writeErrorResponse(w, http.StatusNotFound,
			fmt.Errorf("endpoint not found"), "NOT_FOUND")
	}
}

// handleCreateSubscription handles subscription creation
func (s *Server) handleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	var req SubscribeRequest
	if err := s.parseJSONRequest(r, &req); err != nil {
		s.logRequestError(r, err, "parse_subscribe_request")
		s.writeErrorResponse(w, http.StatusBadRequest, err, "INVALID_REQUEST")
		return
	}

	// Validate request
	if err := s.validateSubscribeRequest(&req); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, err, "VALIDATION_ERROR")
		return
	}

	// Convert to service request
	serviceReq := &subscriber.SubscribeRequest{
		Topic:           req.Topic,
		ConsumerGroup:   req.ConsumerGroup,
		Type:            s.parseSubscriptionType(req.DeliveryMode),
		DeadLetterTopic: req.DeadLetterTopic,
		Config:          req.Config,
	}

	// Set ack deadline
	if req.AckDeadline != nil {
		ackDeadline := time.Duration(*req.AckDeadline) * time.Second
		serviceReq.AckDeadline = ackDeadline
	}

	// Set retry policy
	if req.RetryPolicy != nil {
		serviceReq.MaxRetries = req.RetryPolicy.MaxRetries
	}

	// Create subscription
	ctx := r.Context()
	resp, err := s.subscriberSvc.Subscribe(ctx, serviceReq)
	if err != nil {
		s.logRequestError(r, err, "create_subscription")
		if strings.Contains(err.Error(), "topic not found") {
			s.writeErrorResponse(w, http.StatusNotFound, err, "TOPIC_NOT_FOUND")
		} else {
			s.writeErrorResponse(w, http.StatusInternalServerError, err, "SUBSCRIPTION_FAILED")
		}
		return
	}

	s.writeSuccessResponse(w, resp, "Subscription created successfully")
}

// handleGetSubscription handles subscription retrieval
func (s *Server) handleGetSubscription(w http.ResponseWriter, r *http.Request, subscriptionID string) {
	ctx := r.Context()
	subscription, err := s.subscriberSvc.GetSubscription(ctx, subscriptionID)
	if err != nil {
		s.logRequestError(r, err, "get_subscription")
		if strings.Contains(err.Error(), "not found") {
			s.writeErrorResponse(w, http.StatusNotFound, err, "SUBSCRIPTION_NOT_FOUND")
		} else {
			s.writeErrorResponse(w, http.StatusInternalServerError, err, "GET_FAILED")
		}
		return
	}

	s.writeSuccessResponse(w, subscription, "Subscription retrieved successfully")
}

// handleDeleteSubscription handles subscription deletion
func (s *Server) handleDeleteSubscription(w http.ResponseWriter, r *http.Request, subscriptionID string) {
	ctx := r.Context()
	err := s.subscriberSvc.Unsubscribe(ctx, subscriptionID)
	if err != nil {
		s.logRequestError(r, err, "delete_subscription")
		if strings.Contains(err.Error(), "not found") {
			s.writeErrorResponse(w, http.StatusNotFound, err, "SUBSCRIPTION_NOT_FOUND")
		} else {
			s.writeErrorResponse(w, http.StatusInternalServerError, err, "DELETE_FAILED")
		}
		return
	}

	s.writeSuccessResponse(w, nil, "Subscription deleted successfully")
}

// handlePullMessages handles message pulling from a topic
func (s *Server) handlePullMessages(w http.ResponseWriter, r *http.Request, topic string) {
	if r.Method != "POST" {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
		return
	}

	var req PullRequest
	if err := s.parseJSONRequest(r, &req); err != nil {
		s.logRequestError(r, err, "parse_pull_request")
		s.writeErrorResponse(w, http.StatusBadRequest, err, "INVALID_REQUEST")
		return
	}

	// Convert to service request
	serviceReq := &subscriber.PullRequest{
		Topic: topic,
		Limit: req.MaxMessages,
	}

	// Set ack deadline
	if req.AckDeadline != nil {
		ackDeadline := time.Duration(*req.AckDeadline) * time.Second
		serviceReq.AckDeadline = ackDeadline
	}

	// Set timeout
	if req.WaitTimeout != nil {
		timeout := time.Duration(*req.WaitTimeout) * time.Second
		serviceReq.Timeout = timeout
	}

	// Pull messages
	ctx := r.Context()
	resp, err := s.subscriberSvc.Pull(ctx, serviceReq)
	if err != nil {
		s.logRequestError(r, err, "pull_messages")
		if strings.Contains(err.Error(), "topic not found") {
			s.writeErrorResponse(w, http.StatusNotFound, err, "TOPIC_NOT_FOUND")
		} else {
			s.writeErrorResponse(w, http.StatusInternalServerError, err, "PULL_FAILED")
		}
		return
	}

	// Convert response
	pullResp := s.convertPullResponse(resp)
	s.writeSuccessResponse(w, pullResp, "Messages retrieved successfully")
}

// handleConsumerGroupPull handles consumer group message pulling
func (s *Server) handleConsumerGroupPull(w http.ResponseWriter, r *http.Request, group string) {
	if r.Method != "POST" {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
		return
	}

	var req ConsumerGroupPullRequest
	if err := s.parseJSONRequest(r, &req); err != nil {
		s.logRequestError(r, err, "parse_consumer_group_pull_request")
		s.writeErrorResponse(w, http.StatusBadRequest, err, "INVALID_REQUEST")
		return
	}

	// Validate request
	if req.ConsumerID == "" {
		s.writeErrorResponse(w, http.StatusBadRequest,
			fmt.Errorf("consumer ID is required"), "MISSING_CONSUMER_ID")
		return
	}

	// Convert to service request
	serviceReq := &subscriber.ConsumerGroupPullRequest{
		Topic:         req.ConsumerGroup, // This should be topic from URL path
		ConsumerGroup: group,
		ConsumerID:    req.ConsumerID,
		Limit:         req.MaxMessages,
	}

	// Set ack deadline
	if req.AckDeadline != nil {
		ackDeadline := time.Duration(*req.AckDeadline) * time.Second
		serviceReq.AckDeadline = ackDeadline
	}

	// Pull messages for consumer group
	ctx := r.Context()
	resp, err := s.subscriberSvc.PullWithConsumerGroup(ctx, serviceReq)
	if err != nil {
		s.logRequestError(r, err, "consumer_group_pull")
		if strings.Contains(err.Error(), "not found") {
			s.writeErrorResponse(w, http.StatusNotFound, err, "GROUP_NOT_FOUND")
		} else {
			s.writeErrorResponse(w, http.StatusInternalServerError, err, "PULL_FAILED")
		}
		return
	}

	// Convert response
	pullResp := s.convertPullResponse(resp)
	s.writeSuccessResponse(w, pullResp, "Messages retrieved successfully")
}

// handleJoinConsumerGroup handles joining a consumer group
func (s *Server) handleJoinConsumerGroup(w http.ResponseWriter, r *http.Request, group string) {
	if r.Method != "POST" {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
		return
	}

	var req JoinGroupRequest
	if err := s.parseJSONRequest(r, &req); err != nil {
		s.logRequestError(r, err, "parse_join_group_request")
		s.writeErrorResponse(w, http.StatusBadRequest, err, "INVALID_REQUEST")
		return
	}

	// Validate request
	if req.ConsumerID == "" {
		s.writeErrorResponse(w, http.StatusBadRequest,
			fmt.Errorf("consumer ID is required"), "MISSING_CONSUMER_ID")
		return
	}

	// Convert to service request
	serviceReq := &subscriber.JoinGroupRequest{
		ConsumerGroup: group,
		ConsumerID:    req.ConsumerID,
		Metadata:      req.Config,
	}

	// Join consumer group
	ctx := r.Context()
	resp, err := s.subscriberSvc.JoinConsumerGroup(ctx, serviceReq)
	if err != nil {
		s.logRequestError(r, err, "join_consumer_group")
		s.writeErrorResponse(w, http.StatusInternalServerError, err, "JOIN_FAILED")
		return
	}

	s.writeSuccessResponse(w, resp, "Joined consumer group successfully")
}

// handleLeaveConsumerGroup handles leaving a consumer group
func (s *Server) handleLeaveConsumerGroup(w http.ResponseWriter, r *http.Request, group string) {
	if r.Method != "POST" {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
		return
	}

	// Get consumer ID from query parameter or body
	consumerID := r.URL.Query().Get("consumer_id")
	if consumerID == "" {
		s.writeErrorResponse(w, http.StatusBadRequest,
			fmt.Errorf("consumer ID is required"), "MISSING_CONSUMER_ID")
		return
	}

	// Leave consumer group
	ctx := r.Context()
	err := s.subscriberSvc.LeaveConsumerGroup(ctx, group, consumerID)
	if err != nil {
		s.logRequestError(r, err, "leave_consumer_group")
		if strings.Contains(err.Error(), "not found") {
			s.writeErrorResponse(w, http.StatusNotFound, err, "CONSUMER_NOT_FOUND")
		} else {
			s.writeErrorResponse(w, http.StatusInternalServerError, err, "LEAVE_FAILED")
		}
		return
	}

	s.writeSuccessResponse(w, nil, "Left consumer group successfully")
}

// handleListTopicSubscriptions handles listing subscriptions for a topic
func (s *Server) handleListTopicSubscriptions(w http.ResponseWriter, r *http.Request, topic string) {
	if r.Method != "GET" {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
		return
	}

	ctx := r.Context()
	subscriptions, err := s.subscriberSvc.ListSubscriptions(ctx, topic)
	if err != nil {
		s.logRequestError(r, err, "list_topic_subscriptions")
		if strings.Contains(err.Error(), "topic not found") {
			s.writeErrorResponse(w, http.StatusNotFound, err, "TOPIC_NOT_FOUND")
		} else {
			s.writeErrorResponse(w, http.StatusInternalServerError, err, "LIST_FAILED")
		}
		return
	}

	s.writeSuccessResponse(w, subscriptions, "Subscriptions retrieved successfully")
}

// Helper functions

// validateSubscribeRequest validates a subscribe request
func (s *Server) validateSubscribeRequest(req *SubscribeRequest) error {
	if req.Topic == "" {
		return fmt.Errorf("topic is required")
	}

	// Validate delivery mode
	if req.DeliveryMode != "" {
		validModes := map[string]bool{
			"push": true,
			"pull": true,
		}
		if !validModes[req.DeliveryMode] {
			return fmt.Errorf("invalid delivery mode: %s", req.DeliveryMode)
		}
	}

	// Validate ack deadline
	if req.AckDeadline != nil && *req.AckDeadline < 0 {
		return fmt.Errorf("ack deadline must be non-negative")
	}

	// Validate retry policy
	if req.RetryPolicy != nil {
		if req.RetryPolicy.MaxRetries < 0 {
			return fmt.Errorf("max retries must be non-negative")
		}
		if req.RetryPolicy.MaxBackoff != nil && *req.RetryPolicy.MaxBackoff < 0 {
			return fmt.Errorf("max backoff must be non-negative")
		}
	}

	return nil
}

// parseSubscriptionType converts string delivery mode to subscription type
func (s *Server) parseSubscriptionType(mode string) subscription.Type {
	switch mode {
	case "push":
		return subscription.TypePush
	default:
		return subscription.TypePull
	}
}

// convertPullResponse converts service pull response to HTTP response
func (s *Server) convertPullResponse(resp *subscriber.PullResponse) *PullResponse {
	messages := make([]MessageResponse, len(resp.Messages))
	for i, msg := range resp.Messages {
		messages[i] = s.convertMessage(msg)
	}

	return &PullResponse{
		Messages:   messages,
		HasMore:    resp.HasMore,
		NextOffset: resp.NextOffset,
		WaitTime:   0, // Default
		ReceivedAt: time.Now(),
	}
}

// convertMessage converts a service message to HTTP message
func (s *Server) convertMessage(msg *message.Message) MessageResponse {
	return MessageResponse{
		ID:            msg.ID,
		Topic:         msg.Topic,
		Partition:     msg.Partition,
		Offset:        msg.Offset,
		Key:           base64.StdEncoding.EncodeToString(msg.Key),
		Payload:       base64.StdEncoding.EncodeToString(msg.Payload),
		Headers:       msg.Headers,
		Priority:      s.formatPriority(msg.Priority),
		DeliveryMode:  s.formatDeliveryMode(msg.DeliveryMode),
		ConsumerGroup: msg.ConsumerGroup,
		ConsumerID:    msg.ConsumerID,
		CreatedAt:     msg.CreatedAt,
		DeliveredAt:   msg.DeliveredAt,
		ExpiresAt:     msg.ExpiresAt,
	}
}

// formatPriority converts message.Priority to string
func (s *Server) formatPriority(priority message.Priority) string {
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
func (s *Server) formatDeliveryMode(mode message.DeliveryMode) string {
	switch mode {
	case message.DeliveryAtMostOnce:
		return "at_most_once"
	case message.DeliveryExactlyOnce:
		return "exactly_once"
	default:
		return "at_least_once"
	}
}
