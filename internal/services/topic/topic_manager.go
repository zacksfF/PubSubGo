package topic

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/zacksfF/PubSubGo/internal/config"
	"github.com/zacksfF/PubSubGo/internal/core/topic"
	"github.com/zacksfF/PubSubGo/internal/ports/repository"
)

// Service defines the topic management service interface
type Service interface {
	// CreateTopic creates a new topic
	CreateTopic(ctx context.Context, req *CreateTopicRequest) (*CreateTopicResponse, error)

	// GetTopic retrieves topic information
	GetTopic(ctx context.Context, name string) (*TopicInfo, error)

	// UpdateTopic updates topic configuration
	UpdateTopic(ctx context.Context, name string, req *UpdateTopicRequest) (*TopicInfo, error)

	// DeleteTopic deletes a topic
	DeleteTopic(ctx context.Context, name string) error

	// ListTopics lists all topics with pagination
	ListTopics(ctx context.Context, req *ListTopicsRequest) (*ListTopicsResponse, error)

	// GetTopicStats returns detailed statistics for a topic
	GetTopicStats(ctx context.Context, name string) (*TopicStats, error)

	// GetPartitionInfo returns partition information for a topic
	GetPartitionInfo(ctx context.Context, name string) (*PartitionInfo, error)

	// RebalancePartitions triggers partition rebalancing
	RebalancePartitions(ctx context.Context, name string) (*RebalanceResult, error)

	// GetTopicHealth returns health status of a topic
	GetTopicHealth(ctx context.Context, name string) (*TopicHealth, error)

	// StartTopicWorkers starts background workers for topic management
	StartTopicWorkers(ctx context.Context) error

	// StopTopicWorkers stops background workers
	StopTopicWorkers() error
}

// service implements the topic management service
type service struct {
	topicRepo   repository.TopicRepository
	messageRepo repository.MessageRepository
	config      *config.BrokerConfig
	logger      *logrus.Logger

	// Active topics cache
	activeTopics map[string]*activeTopic
	topicsMutex  sync.RWMutex

	// Partition management
	partitionManager *PartitionManager

	// Background workers
	workers      map[string]context.CancelFunc
	workersMutex sync.RWMutex

	// Statistics collection
	statsCollector *StatsCollector
}

// Request/Response types

type CreateTopicRequest struct {
	Name          string                 `json:"name"`
	Partitions    int32                  `json:"partitions,omitempty"`
	Replication   int32                  `json:"replication,omitempty"`
	RetentionTime time.Duration          `json:"retention_time,omitempty"`
	MaxSize       int64                  `json:"max_size,omitempty"`
	Config        map[string]interface{} `json:"config,omitempty"`
}

type CreateTopicResponse struct {
	Name          string        `json:"name"`
	Partitions    int32         `json:"partitions"`
	Replication   int32         `json:"replication"`
	RetentionTime time.Duration `json:"retention_time"`
	MaxSize       int64         `json:"max_size"`
	CreatedAt     time.Time     `json:"created_at"`
}

type UpdateTopicRequest struct {
	RetentionTime *time.Duration         `json:"retention_time,omitempty"`
	MaxSize       *int64                 `json:"max_size,omitempty"`
	Config        map[string]interface{} `json:"config,omitempty"`
}

type TopicInfo struct {
	Name          string                 `json:"name"`
	Partitions    int32                  `json:"partitions"`
	Replication   int32                  `json:"replication"`
	RetentionTime time.Duration          `json:"retention_time"`
	MaxSize       int64                  `json:"max_size"`
	Config        map[string]interface{} `json:"config"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	Status        string                 `json:"status"`
	Health        string                 `json:"health"`
	Stats         *TopicStats            `json:"stats,omitempty"`
}

type ListTopicsRequest struct {
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Filter string `json:"filter,omitempty"`
}

type ListTopicsResponse struct {
	Topics  []*TopicInfo `json:"topics"`
	Total   int          `json:"total"`
	Offset  int          `json:"offset"`
	Limit   int          `json:"limit"`
	HasMore bool         `json:"has_more"`
}

type TopicStats struct {
	Name              string     `json:"name"`
	MessageCount      int64      `json:"message_count"`
	BytesIn           int64      `json:"bytes_in"`
	BytesOut          int64      `json:"bytes_out"`
	MessagesPerSecond float64    `json:"messages_per_second"`
	BytesPerSecond    float64    `json:"bytes_per_second"`
	AvgMessageSize    float64    `json:"avg_message_size"`
	Partitions        int32      `json:"partitions"`
	ActiveConsumers   int        `json:"active_consumers"`
	PendingMessages   int64      `json:"pending_messages"`
	FailedMessages    int64      `json:"failed_messages"`
	LastActivity      *time.Time `json:"last_activity,omitempty"`
	CollectedAt       time.Time  `json:"collected_at"`
}

type PartitionInfo struct {
	TopicName       string            `json:"topic_name"`
	TotalPartitions int32             `json:"total_partitions"`
	Partitions      []*PartitionStats `json:"partitions"`
	LoadBalance     *LoadBalanceInfo  `json:"load_balance"`
}

type PartitionStats struct {
	ID              int32      `json:"id"`
	MessageCount    int64      `json:"message_count"`
	Size            int64      `json:"size"`
	StartOffset     int64      `json:"start_offset"`
	EndOffset       int64      `json:"end_offset"`
	ActiveConsumers int        `json:"active_consumers"`
	LastActivity    *time.Time `json:"last_activity,omitempty"`
}

type LoadBalanceInfo struct {
	IsBalanced         bool    `json:"is_balanced"`
	ImbalanceRatio     float64 `json:"imbalance_ratio"`
	RecommendRebalance bool    `json:"recommend_rebalance"`
	Reason             string  `json:"reason,omitempty"`
}

type RebalanceResult struct {
	TopicName       string                `json:"topic_name"`
	Success         bool                  `json:"success"`
	StartedAt       time.Time             `json:"started_at"`
	CompletedAt     *time.Time            `json:"completed_at,omitempty"`
	Status          string                `json:"status"`
	PartitionsMoved int                   `json:"partitions_moved"`
	Details         []*PartitionRebalance `json:"details,omitempty"`
	Error           string                `json:"error,omitempty"`
}

type PartitionRebalance struct {
	PartitionID   int32  `json:"partition_id"`
	FromConsumer  string `json:"from_consumer,omitempty"`
	ToConsumer    string `json:"to_consumer,omitempty"`
	MessagesMoved int64  `json:"messages_moved"`
	Status        string `json:"status"`
}

type TopicHealth struct {
	Name        string         `json:"name"`
	Status      string         `json:"status"` // healthy, degraded, unhealthy
	Health      string         `json:"health"` // green, yellow, red
	Issues      []string       `json:"issues,omitempty"`
	Checks      []*HealthCheck `json:"checks"`
	Score       float64        `json:"score"` // 0-100
	LastChecked time.Time      `json:"last_checked"`
}

type HealthCheck struct {
	Name    string                 `json:"name"`
	Status  string                 `json:"status"`
	Score   float64                `json:"score"`
	Message string                 `json:"message,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// Internal types

type activeTopic struct {
	topic       *topic.Topic
	lastAccess  time.Time
	subscribers int32
	publishers  int32
}

// NewService creates a new topic management service
func NewService(
	topicRepo repository.TopicRepository,
	messageRepo repository.MessageRepository,
	config *config.BrokerConfig,
	logger *logrus.Logger,
) Service {
	svc := &service{
		topicRepo:        topicRepo,
		messageRepo:      messageRepo,
		config:           config,
		logger:           logger,
		activeTopics:     make(map[string]*activeTopic),
		workers:          make(map[string]context.CancelFunc),
		partitionManager: NewPartitionManager(config, logger),
		statsCollector:   NewStatsCollector(config, logger),
	}

	return svc
}

// CreateTopic creates a new topic
func (s *service) CreateTopic(ctx context.Context, req *CreateTopicRequest) (*CreateTopicResponse, error) {
	// Validate request
	if err := s.validateCreateTopicRequest(req); err != nil {
		return nil, fmt.Errorf("invalid create topic request: %w", err)
	}

	// Check if topic already exists
	exists, err := s.topicRepo.Exists(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check topic existence: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("topic %s already exists", req.Name)
	}

	// Set defaults
	partitions := req.Partitions
	if partitions <= 0 {
		partitions = s.config.DefaultPartitions
	}

	replication := req.Replication
	if replication <= 0 {
		replication = s.config.DefaultReplication
	}

	retentionTime := req.RetentionTime
	if retentionTime == 0 {
		retentionTime = 24 * time.Hour // Default 24 hours
	}

	maxSize := req.MaxSize
	if maxSize <= 0 {
		maxSize = 1 << 30 // Default 1GB
	}

	// Create topic entity
	topicEntity := topic.NewTopic(req.Name, partitions)
	topicEntity.Replication = replication
	topicEntity.RetentionTime = retentionTime
	topicEntity.MaxSize = maxSize
	if req.Config != nil {
		topicEntity.Config = req.Config
	}

	// Store topic
	if err := s.topicRepo.Create(ctx, topicEntity); err != nil {
		return nil, fmt.Errorf("failed to create topic: %w", err)
	}

	// Initialize partitions
	if err := s.partitionManager.InitializePartitions(ctx, req.Name, partitions); err != nil {
		s.logger.WithError(err).Warn("Failed to initialize partitions")
	}

	// Add to active topics cache
	s.addToActiveTopics(topicEntity)

	s.logger.WithFields(logrus.Fields{
		"topic":       req.Name,
		"partitions":  partitions,
		"replication": replication,
	}).Info("Topic created successfully")

	return &CreateTopicResponse{
		Name:          topicEntity.Name,
		Partitions:    topicEntity.Partitions,
		Replication:   topicEntity.Replication,
		RetentionTime: topicEntity.RetentionTime,
		MaxSize:       topicEntity.MaxSize,
		CreatedAt:     topicEntity.CreatedAt,
	}, nil
}

// GetTopic retrieves topic information
func (s *service) GetTopic(ctx context.Context, name string) (*TopicInfo, error) {
	topicEntity, err := s.topicRepo.Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get topic: %w", err)
	}

	// Get topic health
	health, err := s.GetTopicHealth(ctx, name)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to get topic health")
		health = &TopicHealth{Health: "unknown", Status: "unknown"}
	}

	// Get topic statistics
	stats, err := s.GetTopicStats(ctx, name)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to get topic stats")
	}

	return &TopicInfo{
		Name:          topicEntity.Name,
		Partitions:    topicEntity.Partitions,
		Replication:   topicEntity.Replication,
		RetentionTime: topicEntity.RetentionTime,
		MaxSize:       topicEntity.MaxSize,
		Config:        topicEntity.Config,
		CreatedAt:     topicEntity.CreatedAt,
		UpdatedAt:     topicEntity.UpdatedAt,
		Status:        s.getTopicStatus(topicEntity),
		Health:        health.Health,
		Stats:         stats,
	}, nil
}

// UpdateTopic updates topic configuration
func (s *service) UpdateTopic(ctx context.Context, name string, req *UpdateTopicRequest) (*TopicInfo, error) {
	// Get existing topic
	topicEntity, err := s.topicRepo.Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get topic: %w", err)
	}

	// Update fields if provided
	updated := false

	if req.RetentionTime != nil {
		topicEntity.RetentionTime = *req.RetentionTime
		updated = true
	}

	if req.MaxSize != nil {
		topicEntity.MaxSize = *req.MaxSize
		updated = true
	}

	if req.Config != nil {
		if topicEntity.Config == nil {
			topicEntity.Config = make(map[string]interface{})
		}
		for k, v := range req.Config {
			topicEntity.Config[k] = v
		}
		updated = true
	}

	if !updated {
		return nil, fmt.Errorf("no updates provided")
	}

	topicEntity.UpdatedAt = time.Now()

	// Store updated topic
	if err := s.topicRepo.Update(ctx, topicEntity); err != nil {
		return nil, fmt.Errorf("failed to update topic: %w", err)
	}

	// Update active topics cache
	s.updateActiveTopics(topicEntity)

	s.logger.WithField("topic", name).Info("Topic updated successfully")

	// Return updated topic info
	return s.GetTopic(ctx, name)
}

// DeleteTopic deletes a topic
func (s *service) DeleteTopic(ctx context.Context, name string) error {
	// Check if topic exists
	exists, err := s.topicRepo.Exists(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to check topic existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("topic %s does not exist", name)
	}

	// TODO: Check if topic has active consumers/producers
	// For now, we'll allow deletion

	// Delete topic
	if err := s.topicRepo.Delete(ctx, name); err != nil {
		return fmt.Errorf("failed to delete topic: %w", err)
	}

	// Clean up partitions
	if err := s.partitionManager.CleanupPartitions(ctx, name); err != nil {
		s.logger.WithError(err).Warn("Failed to cleanup partitions")
	}

	// Remove from active topics cache
	s.removeFromActiveTopics(name)

	s.logger.WithField("topic", name).Info("Topic deleted successfully")

	return nil
}

// Helper methods

func (s *service) validateCreateTopicRequest(req *CreateTopicRequest) error {
	if req.Name == "" {
		return fmt.Errorf("topic name cannot be empty")
	}

	if req.Partitions < 0 {
		return fmt.Errorf("partitions cannot be negative")
	}

	if req.Replication < 0 {
		return fmt.Errorf("replication cannot be negative")
	}

	if req.RetentionTime < 0 {
		return fmt.Errorf("retention time cannot be negative")
	}

	if req.MaxSize < 0 {
		return fmt.Errorf("max size cannot be negative")
	}

	return nil
}

func (s *service) addToActiveTopics(t *topic.Topic) {
	s.topicsMutex.Lock()
	defer s.topicsMutex.Unlock()

	s.activeTopics[t.Name] = &activeTopic{
		topic:      t,
		lastAccess: time.Now(),
	}
}

func (s *service) updateActiveTopics(t *topic.Topic) {
	s.topicsMutex.Lock()
	defer s.topicsMutex.Unlock()

	if active, exists := s.activeTopics[t.Name]; exists {
		active.topic = t
		active.lastAccess = time.Now()
	}
}

func (s *service) removeFromActiveTopics(name string) {
	s.topicsMutex.Lock()
	defer s.topicsMutex.Unlock()

	delete(s.activeTopics, name)
}

func (s *service) getTopicStatus(t *topic.Topic) string {
	// Simple status determination
	// In production, this would check for active consumers, message flow, etc.
	return "active"
}

// ListTopics lists all topics with pagination
func (s *service) ListTopics(ctx context.Context, req *ListTopicsRequest) (*ListTopicsResponse, error) {
	// Set defaults
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 50 // Default limit
	} else if limit > 1000 {
		limit = 1000 // Max limit
	}

	// Get topics from repository
	topics, err := s.topicRepo.List(ctx, offset, limit+1) // Get one extra to check if there are more
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}

	// Apply filter if provided
	if req.Filter != "" {
		topics = s.statsCollector.FilterTopics(topics, req.Filter)
	}

	// Check if there are more topics
	hasMore := len(topics) > limit
	if hasMore {
		topics = topics[:limit] // Remove the extra topic
	}

	// Convert to TopicInfo
	topicInfos := make([]*TopicInfo, len(topics))
	for i, t := range topics {
		topicInfos[i] = &TopicInfo{
			Name:          t.Name,
			Partitions:    t.Partitions,
			Replication:   t.Replication,
			RetentionTime: t.RetentionTime,
			MaxSize:       t.MaxSize,
			Config:        t.Config,
			CreatedAt:     t.CreatedAt,
			UpdatedAt:     t.UpdatedAt,
			Status:        s.getTopicStatus(t),
			Health:        "unknown", // Health would be populated separately for performance
		}
	}

	return &ListTopicsResponse{
		Topics:  topicInfos,
		Total:   len(topicInfos), // In production, this would be the total count from a separate query
		Offset:  offset,
		Limit:   limit,
		HasMore: hasMore,
	}, nil
}

// GetTopicStats returns detailed statistics for a topic
func (s *service) GetTopicStats(ctx context.Context, name string) (*TopicStats, error) {
	// Check cache first
	if stats, cached := s.statsCollector.GetCachedStats(name); cached {
		return stats, nil
	}

	// Get topic entity
	topicEntity, err := s.topicRepo.Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get topic: %w", err)
	}

	// Collect fresh statistics
	stats, err := s.statsCollector.CollectTopicStats(ctx, topicEntity, s.messageRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to collect topic stats: %w", err)
	}

	return stats, nil
}

// GetPartitionInfo returns partition information for a topic
func (s *service) GetPartitionInfo(ctx context.Context, name string) (*PartitionInfo, error) {
	return s.partitionManager.GetPartitionInfo(ctx, name)
}

// RebalancePartitions triggers partition rebalancing
func (s *service) RebalancePartitions(ctx context.Context, name string) (*RebalanceResult, error) {
	return s.partitionManager.RebalancePartitions(ctx, name)
}

// GetTopicHealth returns health status of a topic
func (s *service) GetTopicHealth(ctx context.Context, name string) (*TopicHealth, error) {
	// Check cache first
	if health, cached := s.statsCollector.GetCachedHealth(name); cached {
		return health, nil
	}

	// Get topic entity
	topicEntity, err := s.topicRepo.Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get topic: %w", err)
	}

	// Get partition info for health checks
	partitionInfo, err := s.partitionManager.GetPartitionInfo(ctx, name)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to get partition info for health check")
		partitionInfo = nil
	}

	// Collect fresh health data
	health, err := s.statsCollector.CollectTopicHealth(ctx, topicEntity, partitionInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to collect topic health: %w", err)
	}

	return health, nil
}

// StartTopicWorkers starts background workers for topic management
func (s *service) StartTopicWorkers(ctx context.Context) error {
	s.workersMutex.Lock()
	defer s.workersMutex.Unlock()

	// Start stats collection worker
	if _, exists := s.workers["stats_collector"]; !exists {
		workerCtx, cancel := context.WithCancel(ctx)
		s.workers["stats_collector"] = cancel
		go s.statsCollectionWorker(workerCtx)
	}

	// Start health monitoring worker
	if _, exists := s.workers["health_monitor"]; !exists {
		workerCtx, cancel := context.WithCancel(ctx)
		s.workers["health_monitor"] = cancel
		go s.healthMonitoringWorker(workerCtx)
	}

	// Start cleanup worker
	if _, exists := s.workers["cleanup"]; !exists {
		workerCtx, cancel := context.WithCancel(ctx)
		s.workers["cleanup"] = cancel
		go s.cleanupWorker(workerCtx)
	}

	s.logger.Info("Topic management workers started")
	return nil
}

// StopTopicWorkers stops background workers
func (s *service) StopTopicWorkers() error {
	s.workersMutex.Lock()
	defer s.workersMutex.Unlock()

	// Stop all workers
	for name, cancel := range s.workers {
		cancel()
		delete(s.workers, name)
		s.logger.WithField("worker", name).Info("Topic worker stopped")
	}

	return nil
}

// Background worker methods

// statsCollectionWorker periodically collects statistics for active topics
func (s *service) statsCollectionWorker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Collect stats every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.collectStatsForActiveTopics(ctx)
		}
	}
}

// healthMonitoringWorker periodically checks health of active topics
func (s *service) healthMonitoringWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute) // Check health every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkHealthForActiveTopics(ctx)
		}
	}
}

// cleanupWorker performs periodic cleanup tasks
func (s *service) cleanupWorker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour) // Cleanup every hour
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.performCleanupTasks(ctx)
		}
	}
}

// collectStatsForActiveTopics collects statistics for all active topics
func (s *service) collectStatsForActiveTopics(ctx context.Context) {
	s.topicsMutex.RLock()
	activeTopics := make([]*topic.Topic, 0, len(s.activeTopics))
	for _, activeTopic := range s.activeTopics {
		activeTopics = append(activeTopics, activeTopic.topic)
	}
	s.topicsMutex.RUnlock()

	for _, t := range activeTopics {
		if _, err := s.statsCollector.CollectTopicStats(ctx, t, s.messageRepo); err != nil {
			s.logger.WithError(err).WithField("topic", t.Name).Warn("Failed to collect topic stats")
		}
	}
}

// checkHealthForActiveTopics checks health for all active topics
func (s *service) checkHealthForActiveTopics(ctx context.Context) {
	s.topicsMutex.RLock()
	activeTopics := make([]*topic.Topic, 0, len(s.activeTopics))
	for _, activeTopic := range s.activeTopics {
		activeTopics = append(activeTopics, activeTopic.topic)
	}
	s.topicsMutex.RUnlock()

	for _, t := range activeTopics {
		partitionInfo, _ := s.partitionManager.GetPartitionInfo(ctx, t.Name)
		if _, err := s.statsCollector.CollectTopicHealth(ctx, t, partitionInfo); err != nil {
			s.logger.WithError(err).WithField("topic", t.Name).Warn("Failed to collect topic health")
		}
	}
}

// performCleanupTasks performs various cleanup operations
func (s *service) performCleanupTasks(ctx context.Context) {
	now := time.Now()

	// Clean up inactive topics from cache
	s.topicsMutex.Lock()
	for name, activeTopic := range s.activeTopics {
		if now.Sub(activeTopic.lastAccess) > 24*time.Hour {
			delete(s.activeTopics, name)
			s.logger.WithField("topic", name).Debug("Removed inactive topic from cache")
		}
	}
	s.topicsMutex.Unlock()

	// Clear old statistics cache
	if now.Hour() == 0 { // Clear cache at midnight
		s.statsCollector.ClearCache()
		s.logger.Info("Cleared topic statistics cache")
	}
}
