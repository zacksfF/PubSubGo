package topic

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/zacksfF/PubSubGo/internal/config"
	"github.com/zacksfF/PubSubGo/internal/core/topic"
)

// StatsCollector handles topic statistics collection and health monitoring
type StatsCollector struct {
	config *config.BrokerConfig
	logger *logrus.Logger

	// Statistics cache
	statsCache map[string]*TopicStats
	statsMutex sync.RWMutex

	// Health cache
	healthCache map[string]*TopicHealth
	healthMutex sync.RWMutex

	// Collection intervals
	lastCollection time.Time
}

// NewStatsCollector creates a new statistics collector
func NewStatsCollector(config *config.BrokerConfig, logger *logrus.Logger) *StatsCollector {
	return &StatsCollector{
		config:      config,
		logger:      logger,
		statsCache:  make(map[string]*TopicStats),
		healthCache: make(map[string]*TopicHealth),
	}
}

// CollectTopicStats collects statistics for a topic
func (sc *StatsCollector) CollectTopicStats(ctx context.Context, topicEntity *topic.Topic, messageRepo interface{}) (*TopicStats, error) {
	now := time.Now()

	// Get basic stats from topic entity
	messageCount, bytesIn, bytesOut := topicEntity.GetStats()

	// Calculate rates (simplified - in production would use time windows)
	sc.statsMutex.RLock()
	prevStats, hasPrev := sc.statsCache[topicEntity.Name]
	sc.statsMutex.RUnlock()

	var messagesPerSecond, bytesPerSecond float64
	if hasPrev {
		timeDiff := now.Sub(prevStats.CollectedAt).Seconds()
		if timeDiff > 0 {
			messagesPerSecond = float64(messageCount-prevStats.MessageCount) / timeDiff
			bytesPerSecond = float64((bytesIn+bytesOut)-(prevStats.BytesIn+prevStats.BytesOut)) / timeDiff
		}
	}

	// Calculate average message size
	var avgMessageSize float64
	if messageCount > 0 {
		avgMessageSize = float64(bytesIn) / float64(messageCount)
	}

	stats := &TopicStats{
		Name:              topicEntity.Name,
		MessageCount:      messageCount,
		BytesIn:           bytesIn,
		BytesOut:          bytesOut,
		MessagesPerSecond: messagesPerSecond,
		BytesPerSecond:    bytesPerSecond,
		AvgMessageSize:    avgMessageSize,
		Partitions:        topicEntity.Partitions,
		ActiveConsumers:   0, // Would be populated from consumer tracking
		PendingMessages:   0, // Would be calculated from message repository
		FailedMessages:    0, // Would be calculated from message repository
		LastActivity:      &now,
		CollectedAt:       now,
	}

	// Cache the stats
	sc.statsMutex.Lock()
	sc.statsCache[topicEntity.Name] = stats
	sc.statsMutex.Unlock()

	return stats, nil
}

// GetCachedStats returns cached statistics for a topic
func (sc *StatsCollector) GetCachedStats(topicName string) (*TopicStats, bool) {
	sc.statsMutex.RLock()
	defer sc.statsMutex.RUnlock()

	stats, exists := sc.statsCache[topicName]
	if !exists {
		return nil, false
	}

	// Check if stats are too old (older than 1 minute)
	if time.Since(stats.CollectedAt) > time.Minute {
		return nil, false
	}

	return stats, true
}

// CollectTopicHealth performs health checks for a topic
func (sc *StatsCollector) CollectTopicHealth(ctx context.Context, topicEntity *topic.Topic, partitionInfo *PartitionInfo) (*TopicHealth, error) {
	now := time.Now()

	checks := make([]*HealthCheck, 0)
	issues := make([]string, 0)
	totalScore := float64(0)

	// Health Check 1: Topic Configuration
	configCheck := sc.checkTopicConfiguration(topicEntity)
	checks = append(checks, configCheck)
	totalScore += configCheck.Score
	if configCheck.Status == "unhealthy" {
		issues = append(issues, configCheck.Message)
	}

	// Health Check 2: Partition Balance
	if partitionInfo != nil {
		balanceCheck := sc.checkPartitionBalance(partitionInfo)
		checks = append(checks, balanceCheck)
		totalScore += balanceCheck.Score
		if balanceCheck.Status == "unhealthy" {
			issues = append(issues, balanceCheck.Message)
		}
	}

	// Health Check 3: Activity Level
	activityCheck := sc.checkTopicActivity(topicEntity)
	checks = append(checks, activityCheck)
	totalScore += activityCheck.Score
	if activityCheck.Status == "unhealthy" {
		issues = append(issues, activityCheck.Message)
	}

	// Health Check 4: Resource Usage
	resourceCheck := sc.checkResourceUsage(topicEntity)
	checks = append(checks, resourceCheck)
	totalScore += resourceCheck.Score
	if resourceCheck.Status == "unhealthy" {
		issues = append(issues, resourceCheck.Message)
	}

	// Calculate overall health
	avgScore := totalScore / float64(len(checks))
	health, status := sc.calculateOverallHealth(avgScore, len(issues))

	topicHealth := &TopicHealth{
		Name:        topicEntity.Name,
		Status:      status,
		Health:      health,
		Issues:      issues,
		Checks:      checks,
		Score:       avgScore,
		LastChecked: now,
	}

	// Cache the health status
	sc.healthMutex.Lock()
	sc.healthCache[topicEntity.Name] = topicHealth
	sc.healthMutex.Unlock()

	return topicHealth, nil
}

// GetCachedHealth returns cached health status for a topic
func (sc *StatsCollector) GetCachedHealth(topicName string) (*TopicHealth, bool) {
	sc.healthMutex.RLock()
	defer sc.healthMutex.RUnlock()

	health, exists := sc.healthCache[topicName]
	if !exists {
		return nil, false
	}

	// Check if health data is too old (older than 5 minutes)
	if time.Since(health.LastChecked) > 5*time.Minute {
		return nil, false
	}

	return health, true
}

// Individual health check methods

func (sc *StatsCollector) checkTopicConfiguration(topicEntity *topic.Topic) *HealthCheck {
	score := float64(100)
	status := "healthy"
	message := "Topic configuration is optimal"
	details := make(map[string]interface{})

	// Check partition count
	if topicEntity.Partitions <= 0 {
		score -= 50
		status = "unhealthy"
		message = "Invalid partition count"
	} else if topicEntity.Partitions > 100 {
		score -= 20
		status = "degraded"
		message = "High partition count may impact performance"
	}
	details["partitions"] = topicEntity.Partitions

	// Check retention time
	if topicEntity.RetentionTime <= 0 {
		score -= 30
		status = "unhealthy"
		message = "Invalid retention time"
	} else if topicEntity.RetentionTime < time.Hour {
		score -= 10
		if status == "healthy" {
			status = "degraded"
			message = "Short retention time"
		}
	}
	details["retention_time"] = topicEntity.RetentionTime.String()

	// Check max size
	if topicEntity.MaxSize <= 0 {
		score -= 20
		status = "unhealthy"
		message = "Invalid max size"
	}
	details["max_size"] = topicEntity.MaxSize

	if score < 0 {
		score = 0
	}

	return &HealthCheck{
		Name:    "Configuration",
		Status:  status,
		Score:   score,
		Message: message,
		Details: details,
	}
}

func (sc *StatsCollector) checkPartitionBalance(partitionInfo *PartitionInfo) *HealthCheck {
	score := float64(100)
	status := "healthy"
	message := "Partitions are well balanced"
	details := make(map[string]interface{})

	if partitionInfo.LoadBalance != nil {
		details["is_balanced"] = partitionInfo.LoadBalance.IsBalanced
		details["imbalance_ratio"] = partitionInfo.LoadBalance.ImbalanceRatio

		if !partitionInfo.LoadBalance.IsBalanced {
			if partitionInfo.LoadBalance.RecommendRebalance {
				score -= 40
				status = "unhealthy"
				message = "Severe partition imbalance detected"
			} else {
				score -= 20
				status = "degraded"
				message = "Moderate partition imbalance"
			}
		}

		if partitionInfo.LoadBalance.Reason != "" {
			message = partitionInfo.LoadBalance.Reason
		}
	}

	details["total_partitions"] = partitionInfo.TotalPartitions

	return &HealthCheck{
		Name:    "Partition Balance",
		Status:  status,
		Score:   score,
		Message: message,
		Details: details,
	}
}

func (sc *StatsCollector) checkTopicActivity(topicEntity *topic.Topic) *HealthCheck {
	score := float64(100)
	status := "healthy"
	message := "Topic activity is normal"
	details := make(map[string]interface{})

	now := time.Now()
	timeSinceUpdate := now.Sub(topicEntity.UpdatedAt)

	details["last_updated"] = topicEntity.UpdatedAt
	details["time_since_update"] = timeSinceUpdate.String()
	details["message_count"] = topicEntity.MessageCount

	// Check for recent activity
	if timeSinceUpdate > 24*time.Hour {
		score -= 30
		status = "degraded"
		message = "No recent activity detected"
	} else if timeSinceUpdate > 7*24*time.Hour {
		score -= 50
		status = "unhealthy"
		message = "Topic appears inactive"
	}

	// Check message count
	if topicEntity.MessageCount == 0 {
		score -= 10
		if status == "healthy" {
			status = "degraded"
			message = "No messages in topic"
		}
	}

	return &HealthCheck{
		Name:    "Activity Level",
		Status:  status,
		Score:   score,
		Message: message,
		Details: details,
	}
}

func (sc *StatsCollector) checkResourceUsage(topicEntity *topic.Topic) *HealthCheck {
	score := float64(100)
	status := "healthy"
	message := "Resource usage is within limits"
	details := make(map[string]interface{})

	// Calculate current size
	currentSize := topicEntity.BytesIn - topicEntity.BytesOut
	if currentSize < 0 {
		currentSize = topicEntity.BytesIn // Fallback
	}

	details["current_size"] = currentSize
	details["max_size"] = topicEntity.MaxSize
	details["bytes_in"] = topicEntity.BytesIn
	details["bytes_out"] = topicEntity.BytesOut

	// Check size usage
	if topicEntity.MaxSize > 0 {
		usageRatio := float64(currentSize) / float64(topicEntity.MaxSize)
		details["usage_ratio"] = usageRatio

		if usageRatio > 0.9 {
			score -= 40
			status = "unhealthy"
			message = "Topic size approaching limit"
		} else if usageRatio > 0.7 {
			score -= 20
			status = "degraded"
			message = "Topic size usage is high"
		}
	}

	// Check message count relative to partitions
	if topicEntity.Partitions > 0 {
		messagesPerPartition := topicEntity.MessageCount / int64(topicEntity.Partitions)
		details["messages_per_partition"] = messagesPerPartition

		if messagesPerPartition > 1000000 { // 1M messages per partition
			score -= 15
			if status == "healthy" {
				status = "degraded"
				message = "High message density per partition"
			}
		}
	}

	return &HealthCheck{
		Name:    "Resource Usage",
		Status:  status,
		Score:   score,
		Message: message,
		Details: details,
	}
}

func (sc *StatsCollector) calculateOverallHealth(avgScore float64, issueCount int) (health, status string) {
	// Determine health color
	if avgScore >= 80 && issueCount == 0 {
		health = "green"
		status = "healthy"
	} else if avgScore >= 60 {
		health = "yellow"
		status = "degraded"
	} else {
		health = "red"
		status = "unhealthy"
	}

	return health, status
}

// ClearCache clears all cached statistics and health data
func (sc *StatsCollector) ClearCache() {
	sc.statsMutex.Lock()
	sc.healthMutex.Lock()
	defer sc.statsMutex.Unlock()
	defer sc.healthMutex.Unlock()

	sc.statsCache = make(map[string]*TopicStats)
	sc.healthCache = make(map[string]*TopicHealth)
}

// GetAllCachedStats returns all cached statistics
func (sc *StatsCollector) GetAllCachedStats() map[string]*TopicStats {
	sc.statsMutex.RLock()
	defer sc.statsMutex.RUnlock()

	result := make(map[string]*TopicStats)
	for name, stats := range sc.statsCache {
		result[name] = stats
	}

	return result
}

// FilterTopics filters topics based on filter criteria
func (sc *StatsCollector) FilterTopics(topics []*topic.Topic, filter string) []*topic.Topic {
	if filter == "" {
		return topics
	}

	filter = strings.ToLower(filter)
	filtered := make([]*topic.Topic, 0, len(topics))

	for _, t := range topics {
		if strings.Contains(strings.ToLower(t.Name), filter) {
			filtered = append(filtered, t)
		}
	}

	return filtered
}
