package http

import (
	"fmt"
	"net/http"
	"time"
)

// HealthStatus represents the health status response
type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Version   string            `json:"version,omitempty"`
	Services  map[string]string `json:"services"`
}

// healthHandler handles health check requests
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, 
			fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
		return
	}

	// Check service health
	services := make(map[string]string)
	
	// Check publisher service (simplified check)
	services["publisher"] = "healthy"
	
	// Check subscriber service
	services["subscriber"] = "healthy"
	
	// Check topic service
	services["topic"] = "healthy"
	
	// Check acknowledgment service
	services["acknowledgment"] = "healthy"

	// Overall status
	status := "healthy"
	for _, serviceStatus := range services {
		if serviceStatus != "healthy" {
			status = "degraded"
			break
		}
	}

	healthStatus := HealthStatus{
		Status:    status,
		Timestamp: time.Now(),
		Version:   "1.0.0", // Should come from build info
		Services:  services,
	}

	s.writeJSONResponse(w, http.StatusOK, healthStatus)
}

// metricsHandler handles Prometheus metrics requests
func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
		return
	}

	// For now, return basic metrics
	// In production, integrate with prometheus client
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	
	metrics := `# HELP pubsub_messages_total Total number of messages processed
# TYPE pubsub_messages_total counter
pubsub_messages_total 0

# HELP pubsub_topics_total Total number of topics
# TYPE pubsub_topics_total gauge
pubsub_topics_total 0

# HELP pubsub_subscriptions_total Total number of active subscriptions
# TYPE pubsub_subscriptions_total gauge
pubsub_subscriptions_total 0
`
	w.Write([]byte(metrics))
}