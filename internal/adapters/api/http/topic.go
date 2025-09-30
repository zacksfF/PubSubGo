package http

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zacksfF/PubSubGo/internal/services/topic"
)

// CreateTopicRequest represents an HTTP topic creation request
type CreateTopicRequest struct {
	Name          string                 `json:"name"`
	Partitions    int32                  `json:"partitions,omitempty"`
	Replication   int32                  `json:"replication,omitempty"`
	RetentionTime *int64                 `json:"retention_time,omitempty"` // seconds
	MaxSize       *int64                 `json:"max_size,omitempty"`       // bytes
	Config        map[string]interface{} `json:"config,omitempty"`
}

// UpdateTopicRequest represents an HTTP topic update request
type UpdateTopicRequest struct {
	RetentionTime *int64                 `json:"retention_time,omitempty"` // seconds
	MaxSize       *int64                 `json:"max_size,omitempty"`       // bytes
	Config        map[string]interface{} `json:"config,omitempty"`
}

// setupTopicRoutes configures topic-related routes
func (s *Server) setupTopicRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/topics", s.handleTopicListOrCreate)
	mux.HandleFunc("/v1/topics/", s.handleTopicOperations)
}

// handleTopicListOrCreate handles topic list and creation
func (s *Server) handleTopicListOrCreate(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.handleListTopics(w, r)
	case "POST":
		s.handleCreateTopic(w, r)
	default:
		s.writeErrorResponse(w, http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
	}
}

// handleTopicOperations handles individual topic operations
func (s *Server) handleTopicOperations(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	topic := extractTopicFromPath(path)

	if topic == "" {
		s.writeErrorResponse(w, http.StatusBadRequest,
			fmt.Errorf("topic name is required"), "MISSING_TOPIC")
		return
	}

	// Route based on the remaining path and method
	if strings.HasSuffix(path, "/stats") {
		s.handleGetTopicStats(w, r, topic)
	} else if strings.HasSuffix(path, "/health") {
		s.handleGetTopicHealth(w, r, topic)
	} else if strings.HasSuffix(path, "/partitions") {
		s.handleGetPartitionInfo(w, r, topic)
	} else if strings.HasSuffix(path, "/rebalance") {
		s.handleRebalancePartitions(w, r, topic)
	} else if r.URL.Path == "/v1/topics/"+topic {
		// Handle CRUD operations on the topic itself
		switch r.Method {
		case "GET":
			s.handleGetTopic(w, r, topic)
		case "PUT":
			s.handleUpdateTopic(w, r, topic)
		case "DELETE":
			s.handleDeleteTopic(w, r, topic)
		default:
			s.writeErrorResponse(w, http.StatusMethodNotAllowed,
				fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
		}
	} else {
		s.writeErrorResponse(w, http.StatusNotFound,
			fmt.Errorf("endpoint not found"), "NOT_FOUND")
	}
}

// handleCreateTopic handles topic creation
func (s *Server) handleCreateTopic(w http.ResponseWriter, r *http.Request) {
	var req CreateTopicRequest
	if err := s.parseJSONRequest(r, &req); err != nil {
		s.logRequestError(r, err, "parse_create_topic_request")
		s.writeErrorResponse(w, http.StatusBadRequest, err, "INVALID_REQUEST")
		return
	}

	// Validate request
	if err := s.validateCreateTopicRequest(&req); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, err, "VALIDATION_ERROR")
		return
	}

	// Convert to service request
	serviceReq := &topic.CreateTopicRequest{
		Name:        req.Name,
		Partitions:  req.Partitions,
		Replication: req.Replication,
		Config:      req.Config,
	}

	// Convert retention time
	if req.RetentionTime != nil {
		retentionTime := time.Duration(*req.RetentionTime) * time.Second
		serviceReq.RetentionTime = retentionTime
	}

	// Set max size
	if req.MaxSize != nil {
		serviceReq.MaxSize = *req.MaxSize
	}

	// Create topic
	ctx := r.Context()
	resp, err := s.topicSvc.CreateTopic(ctx, serviceReq)
	if err != nil {
		s.logRequestError(r, err, "create_topic")
		if strings.Contains(err.Error(), "already exists") {
			s.writeErrorResponse(w, http.StatusConflict, err, "TOPIC_EXISTS")
		} else {
			s.writeErrorResponse(w, http.StatusInternalServerError, err, "CREATE_FAILED")
		}
		return
	}

	s.writeSuccessResponse(w, resp, "Topic created successfully")
}

// handleListTopics handles topic listing
func (s *Server) handleListTopics(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	offset := parseQueryInt(r, "offset", 0)
	limit := parseQueryInt(r, "limit", 50)
	filter := parseQueryString(r, "filter", "")

	// Create service request
	req := &topic.ListTopicsRequest{
		Offset: offset,
		Limit:  limit,
		Filter: filter,
	}

	// List topics
	ctx := r.Context()
	resp, err := s.topicSvc.ListTopics(ctx, req)
	if err != nil {
		s.logRequestError(r, err, "list_topics")
		s.writeErrorResponse(w, http.StatusInternalServerError, err, "LIST_FAILED")
		return
	}

	s.writeSuccessResponse(w, resp, "Topics retrieved successfully")
}

// handleGetTopic handles single topic retrieval
func (s *Server) handleGetTopic(w http.ResponseWriter, r *http.Request, topicName string) {
	ctx := r.Context()
	topicInfo, err := s.topicSvc.GetTopic(ctx, topicName)
	if err != nil {
		s.logRequestError(r, err, "get_topic")
		if strings.Contains(err.Error(), "not found") {
			s.writeErrorResponse(w, http.StatusNotFound, err, "TOPIC_NOT_FOUND")
		} else {
			s.writeErrorResponse(w, http.StatusInternalServerError, err, "GET_FAILED")
		}
		return
	}

	s.writeSuccessResponse(w, topicInfo, "Topic retrieved successfully")
}

// handleUpdateTopic handles topic updates
func (s *Server) handleUpdateTopic(w http.ResponseWriter, r *http.Request, topicName string) {
	var req UpdateTopicRequest
	if err := s.parseJSONRequest(r, &req); err != nil {
		s.logRequestError(r, err, "parse_update_topic_request")
		s.writeErrorResponse(w, http.StatusBadRequest, err, "INVALID_REQUEST")
		return
	}

	// Convert to service request
	serviceReq := &topic.UpdateTopicRequest{
		Config: req.Config,
	}

	// Convert retention time
	if req.RetentionTime != nil {
		retentionTime := time.Duration(*req.RetentionTime) * time.Second
		serviceReq.RetentionTime = &retentionTime
	}

	// Set max size
	if req.MaxSize != nil {
		serviceReq.MaxSize = req.MaxSize
	}

	// Update topic
	ctx := r.Context()
	topicInfo, err := s.topicSvc.UpdateTopic(ctx, topicName, serviceReq)
	if err != nil {
		s.logRequestError(r, err, "update_topic")
		if strings.Contains(err.Error(), "not found") {
			s.writeErrorResponse(w, http.StatusNotFound, err, "TOPIC_NOT_FOUND")
		} else if strings.Contains(err.Error(), "no updates") {
			s.writeErrorResponse(w, http.StatusBadRequest, err, "NO_UPDATES")
		} else {
			s.writeErrorResponse(w, http.StatusInternalServerError, err, "UPDATE_FAILED")
		}
		return
	}

	s.writeSuccessResponse(w, topicInfo, "Topic updated successfully")
}

// handleDeleteTopic handles topic deletion
func (s *Server) handleDeleteTopic(w http.ResponseWriter, r *http.Request, topicName string) {
	ctx := r.Context()
	err := s.topicSvc.DeleteTopic(ctx, topicName)
	if err != nil {
		s.logRequestError(r, err, "delete_topic")
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "does not exist") {
			s.writeErrorResponse(w, http.StatusNotFound, err, "TOPIC_NOT_FOUND")
		} else {
			s.writeErrorResponse(w, http.StatusInternalServerError, err, "DELETE_FAILED")
		}
		return
	}

	s.writeSuccessResponse(w, nil, "Topic deleted successfully")
}

// handleGetTopicStats handles topic statistics retrieval
func (s *Server) handleGetTopicStats(w http.ResponseWriter, r *http.Request, topicName string) {
	if r.Method != "GET" {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
		return
	}

	ctx := r.Context()
	stats, err := s.topicSvc.GetTopicStats(ctx, topicName)
	if err != nil {
		s.logRequestError(r, err, "get_topic_stats")
		if strings.Contains(err.Error(), "not found") {
			s.writeErrorResponse(w, http.StatusNotFound, err, "TOPIC_NOT_FOUND")
		} else {
			s.writeErrorResponse(w, http.StatusInternalServerError, err, "STATS_FAILED")
		}
		return
	}

	s.writeSuccessResponse(w, stats, "Topic statistics retrieved successfully")
}

// handleGetTopicHealth handles topic health retrieval
func (s *Server) handleGetTopicHealth(w http.ResponseWriter, r *http.Request, topicName string) {
	if r.Method != "GET" {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
		return
	}

	ctx := r.Context()
	health, err := s.topicSvc.GetTopicHealth(ctx, topicName)
	if err != nil {
		s.logRequestError(r, err, "get_topic_health")
		if strings.Contains(err.Error(), "not found") {
			s.writeErrorResponse(w, http.StatusNotFound, err, "TOPIC_NOT_FOUND")
		} else {
			s.writeErrorResponse(w, http.StatusInternalServerError, err, "HEALTH_FAILED")
		}
		return
	}

	s.writeSuccessResponse(w, health, "Topic health retrieved successfully")
}

// handleGetPartitionInfo handles partition information retrieval
func (s *Server) handleGetPartitionInfo(w http.ResponseWriter, r *http.Request, topicName string) {
	if r.Method != "GET" {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
		return
	}

	ctx := r.Context()
	partitionInfo, err := s.topicSvc.GetPartitionInfo(ctx, topicName)
	if err != nil {
		s.logRequestError(r, err, "get_partition_info")
		if strings.Contains(err.Error(), "not found") {
			s.writeErrorResponse(w, http.StatusNotFound, err, "TOPIC_NOT_FOUND")
		} else {
			s.writeErrorResponse(w, http.StatusInternalServerError, err, "PARTITION_INFO_FAILED")
		}
		return
	}

	s.writeSuccessResponse(w, partitionInfo, "Partition information retrieved successfully")
}

// handleRebalancePartitions handles partition rebalancing
func (s *Server) handleRebalancePartitions(w http.ResponseWriter, r *http.Request, topicName string) {
	if r.Method != "POST" {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"), "METHOD_NOT_ALLOWED")
		return
	}

	ctx := r.Context()
	result, err := s.topicSvc.RebalancePartitions(ctx, topicName)
	if err != nil {
		s.logRequestError(r, err, "rebalance_partitions")
		if strings.Contains(err.Error(), "not found") {
			s.writeErrorResponse(w, http.StatusNotFound, err, "TOPIC_NOT_FOUND")
		} else if strings.Contains(err.Error(), "already in progress") {
			s.writeErrorResponse(w, http.StatusConflict, err, "REBALANCE_IN_PROGRESS")
		} else {
			s.writeErrorResponse(w, http.StatusInternalServerError, err, "REBALANCE_FAILED")
		}
		return
	}

	s.writeSuccessResponse(w, result, "Partition rebalancing initiated successfully")
}

// validateCreateTopicRequest validates a create topic request
func (s *Server) validateCreateTopicRequest(req *CreateTopicRequest) error {
	if req.Name == "" {
		return fmt.Errorf("topic name is required")
	}

	// Validate topic name format
	if len(req.Name) > 255 {
		return fmt.Errorf("topic name must be 255 characters or less")
	}

	// Basic name validation (alphanumeric, dash, underscore)
	for _, r := range req.Name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
			return fmt.Errorf("topic name contains invalid characters")
		}
	}

	// Validate partitions
	if req.Partitions < 0 {
		return fmt.Errorf("partitions must be non-negative")
	}

	// Validate replication
	if req.Replication < 0 {
		return fmt.Errorf("replication must be non-negative")
	}

	// Validate retention time
	if req.RetentionTime != nil && *req.RetentionTime < 0 {
		return fmt.Errorf("retention time must be non-negative")
	}

	// Validate max size
	if req.MaxSize != nil && *req.MaxSize < 0 {
		return fmt.Errorf("max size must be non-negative")
	}

	return nil
}
