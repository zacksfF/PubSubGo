package http

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/zacksfF/PubSubGo/internal/services/subscriber"
)

// AcknowledgeRequest represents an HTTP acknowledgment request
type AcknowledgeRequest struct {
	MessageID     string `json:"message_id"`
	ConsumerID    string `json:"consumer_id"`
	ConsumerGroup string `json:"consumer_group,omitempty"`
}

// NackRequest represents an HTTP negative acknowledgment request
type NackRequest struct {
	MessageID     string `json:"message_id"`
	ConsumerID    string `json:"consumer_id"`
	ConsumerGroup string `json:"consumer_group,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Retry         bool   `json:"retry"`
}

// setupAcknowledgeRoutes configures acknowledgment-related routes
func (s *Server) setupAcknowledgeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/messages/", s.handleMessageOperations)
}

// handleMessageOperations handles message acknowledgment operations
func (s *Server) handleMessageOperations(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	messageID := extractMessageIDFromPath(path)
	
	if messageID == "" {
		s.writeErrorResponse(w, http.StatusBadRequest,
			fmt.Errorf("message ID is required"), "MISSING_MESSAGE_ID")
		return
	}

	// Route based on the operation
	if strings.HasSuffix(path, "/ack") {
		s.handleAcknowledgeMessage(w, r, messageID)
	} else if strings.HasSuffix(path, "/nack") {
		s.handleNegativeAcknowledgeMessage(w, r, messageID)
	} else {
		s.writeErrorResponse(w, http.StatusNotFound,
			fmt.Errorf("endpoint not found"), "NOT_FOUND")
	}
}

// handleAcknowledgeMessage handles message acknowledgment
func (s *Server) handleAcknowledgeMessage(w http.ResponseWriter, r *http.Request, messageID string) {
	if r.Method != "POST" {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
		return
	}

	var req AcknowledgeRequest
	if err := s.parseJSONRequest(r, &req); err != nil {
		s.logRequestError(r, err, "parse_acknowledge_request")
		s.writeErrorResponse(w, http.StatusBadRequest, err, "INVALID_REQUEST")
		return
	}

	// Validate request
	if err := s.validateAcknowledgeRequest(&req); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, err, "VALIDATION_ERROR")
		return
	}

	// Override message ID from URL
	req.MessageID = messageID

	// Convert to service request
	serviceReq := &subscriber.AcknowledgeRequest{
		MessageID:     req.MessageID,
		ConsumerID:    req.ConsumerID,
		ConsumerGroup: req.ConsumerGroup,
	}

	// Acknowledge message
	ctx := r.Context()
	err := s.subscriberSvc.Acknowledge(ctx, serviceReq)
	if err != nil {
		s.logRequestError(r, err, "acknowledge_message")
		if strings.Contains(err.Error(), "not found") {
			s.writeErrorResponse(w, http.StatusNotFound, err, "MESSAGE_NOT_FOUND")
		} else if strings.Contains(err.Error(), "already acknowledged") {
			s.writeErrorResponse(w, http.StatusConflict, err, "ALREADY_ACKNOWLEDGED")
		} else if strings.Contains(err.Error(), "deadline exceeded") {
			s.writeErrorResponse(w, http.StatusGone, err, "ACK_DEADLINE_EXCEEDED")
		} else {
			s.writeErrorResponse(w, http.StatusInternalServerError, err, "ACKNOWLEDGE_FAILED")
		}
		return
	}

	s.writeSuccessResponse(w, nil, "Message acknowledged successfully")
}

// handleNegativeAcknowledgeMessage handles negative message acknowledgment
func (s *Server) handleNegativeAcknowledgeMessage(w http.ResponseWriter, r *http.Request, messageID string) {
	if r.Method != "POST" {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
		return
	}

	var req NackRequest
	if err := s.parseJSONRequest(r, &req); err != nil {
		s.logRequestError(r, err, "parse_nack_request")
		s.writeErrorResponse(w, http.StatusBadRequest, err, "INVALID_REQUEST")
		return
	}

	// Validate request
	if err := s.validateNackRequest(&req); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, err, "VALIDATION_ERROR")
		return
	}

	// Override message ID from URL
	req.MessageID = messageID

	// Convert to service request
	serviceReq := &subscriber.NackRequest{
		MessageID:     req.MessageID,
		ConsumerID:    req.ConsumerID,
		ConsumerGroup: req.ConsumerGroup,
		Reason:        req.Reason,
		Retry:         req.Retry,
	}

	// Negative acknowledge message
	ctx := r.Context()
	err := s.subscriberSvc.NegativeAcknowledge(ctx, serviceReq)
	if err != nil {
		s.logRequestError(r, err, "nack_message")
		if strings.Contains(err.Error(), "not found") {
			s.writeErrorResponse(w, http.StatusNotFound, err, "MESSAGE_NOT_FOUND")
		} else if strings.Contains(err.Error(), "already acknowledged") {
			s.writeErrorResponse(w, http.StatusConflict, err, "ALREADY_ACKNOWLEDGED")
		} else if strings.Contains(err.Error(), "max retries exceeded") {
			s.writeErrorResponse(w, http.StatusBadRequest, err, "MAX_RETRIES_EXCEEDED")
		} else {
			s.writeErrorResponse(w, http.StatusInternalServerError, err, "NACK_FAILED")
		}
		return
	}

	// Return different messages based on retry flag
	message := "Message negative acknowledged successfully"
	if req.Retry {
		message = "Message negative acknowledged and scheduled for retry"
	} else {
		message = "Message negative acknowledged and moved to DLQ"
	}

	s.writeSuccessResponse(w, nil, message)
}

// Helper functions

// validateAcknowledgeRequest validates an acknowledge request
func (s *Server) validateAcknowledgeRequest(req *AcknowledgeRequest) error {
	if req.MessageID == "" {
		return fmt.Errorf("message ID is required")
	}

	if req.ConsumerID == "" {
		return fmt.Errorf("consumer ID is required")
	}

	return nil
}

// validateNackRequest validates a negative acknowledge request
func (s *Server) validateNackRequest(req *NackRequest) error {
	if req.MessageID == "" {
		return fmt.Errorf("message ID is required")
	}

	if req.ConsumerID == "" {
		return fmt.Errorf("consumer ID is required")
	}

	return nil
}