package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// SuccessResponse represents a successful API response
type SuccessResponse struct {
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// writeJSONResponse writes a JSON response
func (s *Server) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.WithError(err).Error("Failed to encode JSON response")
	}
}

// writeErrorResponse writes an error response
func (s *Server) writeErrorResponse(w http.ResponseWriter, statusCode int, err error, code string) {
	response := ErrorResponse{
		Error:   err.Error(),
		Code:    code,
		Message: fmt.Sprintf("Request failed: %s", err.Error()),
	}
	s.writeJSONResponse(w, statusCode, response)
}

// writeSuccessResponse writes a success response
func (s *Server) writeSuccessResponse(w http.ResponseWriter, data interface{}, message string) {
	response := SuccessResponse{
		Data:    data,
		Message: message,
	}
	s.writeJSONResponse(w, http.StatusOK, response)
}

// parseJSONRequest parses JSON request body into the provided struct
func (s *Server) parseJSONRequest(r *http.Request, v interface{}) error {
	if r.Header.Get("Content-Type") != "application/json" {
		return fmt.Errorf("content-type must be application/json")
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	return nil
}

// extractPathParam extracts a path parameter from URL
func extractPathParam(path, pattern string) string {
	// Simple path parameter extraction
	// For production, use a proper router like gorilla/mux or chi
	parts := strings.Split(path, "/")
	patternParts := strings.Split(pattern, "/")

	for i, part := range patternParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			if i < len(parts) {
				return parts[i]
			}
		}
	}
	return ""
}

// extractTopicFromPath extracts topic name from URL path
func extractTopicFromPath(path string) string {
	// Extract topic from paths like /v1/topics/{topic}/...
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) >= 3 && parts[0] == "v1" && parts[1] == "topics" {
		return parts[2]
	}
	return ""
}

// extractSubscriptionIDFromPath extracts subscription ID from URL path
func extractSubscriptionIDFromPath(path string) string {
	// Extract ID from paths like /v1/subscriptions/{id}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) >= 3 && parts[0] == "v1" && parts[1] == "subscriptions" {
		return parts[2]
	}
	return ""
}

// extractMessageIDFromPath extracts message ID from URL path
func extractMessageIDFromPath(path string) string {
	// Extract ID from paths like /v1/messages/{id}/...
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) >= 3 && parts[0] == "v1" && parts[1] == "messages" {
		return parts[2]
	}
	return ""
}

// extractConsumerGroupFromPath extracts consumer group from URL path
func extractConsumerGroupFromPath(path string) string {
	// Extract group from paths like /v1/consumer-groups/{group}/...
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) >= 3 && parts[0] == "v1" && parts[1] == "consumer-groups" {
		return parts[2]
	}
	return ""
}

// parseQueryInt parses integer query parameter
func parseQueryInt(r *http.Request, key string, defaultValue int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intValue
}

// parseQueryString parses string query parameter
func parseQueryString(r *http.Request, key, defaultValue string) string {
	value := r.URL.Query().Get(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// logRequestError logs request processing errors
func (s *Server) logRequestError(r *http.Request, err error, context string) {
	s.logger.WithFields(logrus.Fields{
		"method":  r.Method,
		"path":    r.URL.Path,
		"context": context,
		"error":   err.Error(),
	}).Error("Request processing failed")
}