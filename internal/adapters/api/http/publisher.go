package http

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zacksfF/PubSubGo/internal/core/message"
	"github.com/zacksfF/PubSubGo/internal/services/publisher"
)

// PublishRequest represents an HTTP publish request
type PublishRequest struct {
	Key          string            `json:"key,omitempty"`
	Payload      string            `json:"payload"`           // Base64 encoded payload
	Headers      map[string]string `json:"headers,omitempty"`
	Priority     string            `json:"priority,omitempty"`     // "low", "normal", "high", "critical"
	DeliveryMode string            `json:"delivery_mode,omitempty"` // "at_most_once", "at_least_once", "exactly_once"
	TTL          *int64            `json:"ttl,omitempty"`          // TTL in seconds
}

// BatchPublishRequest represents an HTTP batch publish request
type BatchPublishRequest struct {
	Messages []PublishRequest `json:"messages"`
}

// PublishResponse represents an HTTP publish response
type PublishResponse struct {
	MessageID string    `json:"message_id"`
	Topic     string    `json:"topic"`
	Partition int32     `json:"partition"`
	Offset    int64     `json:"offset"`
	Timestamp time.Time `json:"timestamp"`
}

// BatchPublishResponse represents an HTTP batch publish response
type BatchPublishResponse struct {
	Messages []PublishResponse `json:"messages"`
	Failed   []FailedMessage   `json:"failed,omitempty"`
}

type FailedMessage struct {
	Index   int    `json:"index"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

// setupPublisherRoutes configures publisher-related routes
func (s *Server) setupPublisherRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/publish/", s.handlePublisherRoutes)
}

// handlePublisherRoutes handles all publisher-related routes
func (s *Server) handlePublisherRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Extract topic from path (/v1/publish/{topic} or /v1/publish/{topic}/batch)
	pathParts := strings.Split(strings.TrimPrefix(path, "/v1/publish/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		s.writeErrorResponse(w, http.StatusBadRequest,
			fmt.Errorf("topic name is required"), "MISSING_TOPIC")
		return
	}
	
	topic := pathParts[0]

	// Route based on the remaining path
	if len(pathParts) > 1 && pathParts[1] == "batch" {
		s.handleBatchPublish(w, r, topic)
	} else if len(pathParts) == 1 {
		s.handlePublish(w, r, topic)
	} else {
		s.writeErrorResponse(w, http.StatusNotFound,
			fmt.Errorf("endpoint not found"), "NOT_FOUND")
	}
}

// handlePublish handles single message publishing
func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request, topic string) {
	if r.Method != "POST" {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
		return
	}

	// Parse request
	var req PublishRequest
	if err := s.parseJSONRequest(r, &req); err != nil {
		s.logRequestError(r, err, "parse_publish_request")
		s.writeErrorResponse(w, http.StatusBadRequest, err, "INVALID_REQUEST")
		return
	}

	// Validate request
	if err := s.validatePublishRequest(&req); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, err, "VALIDATION_ERROR")
		return
	}

	// Decode payload
	payload, err := base64.StdEncoding.DecodeString(req.Payload)
	if err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest,
			fmt.Errorf("invalid base64 payload: %w", err), "INVALID_PAYLOAD")
		return
	}

	// Convert to service request
	publishReq := &publisher.PublishRequest{
		Topic:        topic,
		Key:          []byte(req.Key),
		Payload:      payload,
		Headers:      req.Headers,
		Priority:     s.parsePriority(req.Priority),
		DeliveryMode: s.parseDeliveryMode(req.DeliveryMode),
	}

	// Set TTL if provided
	if req.TTL != nil {
		ttl := time.Duration(*req.TTL) * time.Second
		publishReq.TTL = &ttl
	}

	// Publish message
	ctx := r.Context()
	resp, err := s.publisherSvc.Publish(ctx, publishReq)
	if err != nil {
		s.logRequestError(r, err, "publish_message")
		s.writeErrorResponse(w, http.StatusInternalServerError, err, "PUBLISH_FAILED")
		return
	}

	// Convert response
	response := PublishResponse{
		MessageID: resp.MessageID,
		Topic:     resp.Topic,
		Partition: resp.Partition,
		Offset:    resp.Offset,
		Timestamp: resp.Timestamp,
	}

	s.writeSuccessResponse(w, response, "Message published successfully")
}

// handleBatchPublish handles batch message publishing
func (s *Server) handleBatchPublish(w http.ResponseWriter, r *http.Request, topic string) {
	if r.Method != "POST" {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
		return
	}

	// Parse request
	var req BatchPublishRequest
	if err := s.parseJSONRequest(r, &req); err != nil {
		s.logRequestError(r, err, "parse_batch_publish_request")
		s.writeErrorResponse(w, http.StatusBadRequest, err, "INVALID_REQUEST")
		return
	}

	// Validate request
	if len(req.Messages) == 0 {
		s.writeErrorResponse(w, http.StatusBadRequest,
			fmt.Errorf("at least one message is required"), "EMPTY_BATCH")
		return
	}

	if len(req.Messages) > 1000 { // Configurable limit
		s.writeErrorResponse(w, http.StatusBadRequest,
			fmt.Errorf("batch size exceeds maximum limit of 1000"), "BATCH_TOO_LARGE")
		return
	}

	// Convert messages
	messages := make([]*publisher.MessageBatch, 0, len(req.Messages))
	failed := make([]FailedMessage, 0)

	for i, msg := range req.Messages {
		// Validate message
		if err := s.validatePublishRequest(&msg); err != nil {
			failed = append(failed, FailedMessage{
				Index:   i,
				Error:   "VALIDATION_ERROR",
				Message: err.Error(),
			})
			continue
		}

		// Decode payload
		payload, err := base64.StdEncoding.DecodeString(msg.Payload)
		if err != nil {
			failed = append(failed, FailedMessage{
				Index:   i,
				Error:   "INVALID_PAYLOAD",
				Message: fmt.Sprintf("invalid base64 payload: %v", err),
			})
			continue
		}

		// Convert to service request
		batchMessage := &publisher.MessageBatch{
			Key:          []byte(msg.Key),
			Payload:      payload,
			Headers:      msg.Headers,
			Priority:     s.parsePriority(msg.Priority),
			DeliveryMode: s.parseDeliveryMode(msg.DeliveryMode),
		}

		// Set TTL if provided
		if msg.TTL != nil {
			ttl := time.Duration(*msg.TTL) * time.Second
			batchMessage.TTL = &ttl
		}

		messages = append(messages, batchMessage)
	}

	// If all messages failed validation, return error
	if len(messages) == 0 {
		response := BatchPublishResponse{
			Messages: make([]PublishResponse, 0),
			Failed:   failed,
		}
		s.writeJSONResponse(w, http.StatusBadRequest, response)
		return
	}

	// Publish batch
	batchReq := &publisher.BatchPublishRequest{
		Topic:    topic,
		Messages: messages,
	}

	ctx := r.Context()
	batchResp, err := s.publisherSvc.PublishBatch(ctx, batchReq)
	if err != nil {
		s.logRequestError(r, err, "publish_batch")
		s.writeErrorResponse(w, http.StatusInternalServerError, err, "BATCH_PUBLISH_FAILED")
		return
	}

	// Convert responses
	responses := make([]PublishResponse, 0)
	if batchResp.Responses != nil {
		responses = make([]PublishResponse, len(batchResp.Responses))
		for i, msg := range batchResp.Responses {
			responses[i] = PublishResponse{
				MessageID: msg.MessageID,
				Topic:     msg.Topic,
				Partition: msg.Partition,
				Offset:    msg.Offset,
				Timestamp: msg.Timestamp,
			}
		}
	}

	// Add batch-level failures to the failed list
	if len(batchResp.Errors) > 0 {
		for i, errorMsg := range batchResp.Errors {
			if errorMsg != "" {
				failed = append(failed, FailedMessage{
					Index:   i,
					Error:   "PUBLISH_FAILED",
					Message: errorMsg,
				})
			}
		}
	}

	response := BatchPublishResponse{
		Messages: responses,
		Failed:   failed,
	}

	statusCode := http.StatusOK
	if len(failed) > 0 {
		statusCode = http.StatusPartialContent
	}

	s.writeJSONResponse(w, statusCode, response)
}

// validatePublishRequest validates a publish request
func (s *Server) validatePublishRequest(req *PublishRequest) error {
	if req.Payload == "" {
		return fmt.Errorf("payload is required")
	}

	// Validate base64 encoding
	if _, err := base64.StdEncoding.DecodeString(req.Payload); err != nil {
		return fmt.Errorf("payload must be base64 encoded")
	}

	// Validate priority if specified
	if req.Priority != "" {
		validPriorities := map[string]bool{
			"low":      true,
			"normal":   true,
			"high":     true,
			"critical": true,
		}
		if !validPriorities[req.Priority] {
			return fmt.Errorf("invalid priority: %s", req.Priority)
		}
	}

	// Validate delivery mode if specified
	if req.DeliveryMode != "" {
		validModes := map[string]bool{
			"at_most_once":  true,
			"at_least_once": true,
			"exactly_once":  true,
		}
		if !validModes[req.DeliveryMode] {
			return fmt.Errorf("invalid delivery mode: %s", req.DeliveryMode)
		}
	}

	// Validate TTL if specified
	if req.TTL != nil && *req.TTL < 0 {
		return fmt.Errorf("TTL must be non-negative")
	}

	return nil
}

// parsePriority converts string priority to message.Priority
func (s *Server) parsePriority(priority string) message.Priority {
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

// parseDeliveryMode converts string delivery mode to message.DeliveryMode
func (s *Server) parseDeliveryMode(mode string) message.DeliveryMode {
	switch mode {
	case "at_most_once":
		return message.DeliveryAtMostOnce
	case "exactly_once":
		return message.DeliveryExactlyOnce
	default:
		return message.DeliveryAtLeastOnce
	}
}
