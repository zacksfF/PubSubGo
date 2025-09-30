package topic

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/zacksfF/PubSubGo/internal/config"
)

// PartitionManager handles partition management and load balancing
type PartitionManager struct {
	config  *config.BrokerConfig
	logger  *logrus.Logger
	
	// Partition assignment tracking
	assignments map[string]*PartitionAssignment
	mutex       sync.RWMutex
}

// PartitionAssignment tracks partition assignments for a topic
type PartitionAssignment struct {
	TopicName     string
	Partitions    []*Partition
	ConsumerMap   map[string][]int32 // consumer_id -> partition_ids
	LastRebalance time.Time
	Rebalancing   bool
}

// Partition represents a topic partition
type Partition struct {
	ID            int32
	TopicName     string
	MessageCount  int64
	Size          int64
	StartOffset   int64
	EndOffset     int64
	Consumers     []string
	LastActivity  time.Time
	CreatedAt     time.Time
}

// NewPartitionManager creates a new partition manager
func NewPartitionManager(config *config.BrokerConfig, logger *logrus.Logger) *PartitionManager {
	return &PartitionManager{
		config:      config,
		logger:      logger,
		assignments: make(map[string]*PartitionAssignment),
	}
}

// InitializePartitions creates initial partitions for a topic
func (pm *PartitionManager) InitializePartitions(ctx context.Context, topicName string, partitionCount int32) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	
	partitions := make([]*Partition, partitionCount)
	now := time.Now()
	
	for i := int32(0); i < partitionCount; i++ {
		partitions[i] = &Partition{
			ID:           i,
			TopicName:    topicName,
			MessageCount: 0,
			Size:         0,
			StartOffset:  0,
			EndOffset:    0,
			Consumers:    make([]string, 0),
			LastActivity: now,
			CreatedAt:    now,
		}
	}
	
	pm.assignments[topicName] = &PartitionAssignment{
		TopicName:     topicName,
		Partitions:    partitions,
		ConsumerMap:   make(map[string][]int32),
		LastRebalance: now,
		Rebalancing:   false,
	}
	
	pm.logger.WithFields(logrus.Fields{
		"topic":      topicName,
		"partitions": partitionCount,
	}).Info("Partitions initialized")
	
	return nil
}

// AssignPartition assigns a partition to a message based on key
func (pm *PartitionManager) AssignPartition(topicName string, key []byte, partitionCount int32) int32 {
	if len(key) == 0 {
		// Round-robin for messages without keys
		result := int32(time.Now().UnixNano()) % partitionCount
		if result < 0 {
			result += partitionCount
		}
		return result
	}
	
	// Hash-based assignment for keyed messages
	hash := fnv.New32a()
	hash.Write(key)
	result := int32(hash.Sum32()) % partitionCount
	if result < 0 {
		result += partitionCount
	}
	return result
}

// GetPartitionInfo returns partition information for a topic
func (pm *PartitionManager) GetPartitionInfo(ctx context.Context, topicName string) (*PartitionInfo, error) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	assignment, exists := pm.assignments[topicName]
	if !exists {
		return nil, fmt.Errorf("topic %s not found", topicName)
	}
	
	partitionStats := make([]*PartitionStats, len(assignment.Partitions))
	for i, partition := range assignment.Partitions {
		partitionStats[i] = &PartitionStats{
			ID:              partition.ID,
			MessageCount:    partition.MessageCount,
			Size:            partition.Size,
			StartOffset:     partition.StartOffset,
			EndOffset:       partition.EndOffset,
			ActiveConsumers: len(partition.Consumers),
			LastActivity:    &partition.LastActivity,
		}
	}
	
	// Calculate load balance info
	loadBalance := pm.calculateLoadBalance(assignment)
	
	return &PartitionInfo{
		TopicName:       topicName,
		TotalPartitions: int32(len(assignment.Partitions)),
		Partitions:      partitionStats,
		LoadBalance:     loadBalance,
	}, nil
}

// RebalancePartitions performs partition rebalancing for a topic
func (pm *PartitionManager) RebalancePartitions(ctx context.Context, topicName string) (*RebalanceResult, error) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	
	assignment, exists := pm.assignments[topicName]
	if !exists {
		return nil, fmt.Errorf("topic %s not found", topicName)
	}
	
	if assignment.Rebalancing {
		return nil, fmt.Errorf("rebalancing already in progress for topic %s", topicName)
	}
	
	startTime := time.Now()
	assignment.Rebalancing = true
	
	result := &RebalanceResult{
		TopicName: topicName,
		StartedAt: startTime,
		Status:    "in_progress",
		Details:   make([]*PartitionRebalance, 0),
	}
	
	defer func() {
		assignment.Rebalancing = false
		assignment.LastRebalance = time.Now()
		endTime := time.Now()
		result.CompletedAt = &endTime
	}()
	
	// Get current consumer assignments
	consumers := make([]string, 0, len(assignment.ConsumerMap))
	for consumerID := range assignment.ConsumerMap {
		consumers = append(consumers, consumerID)
	}
	
	if len(consumers) == 0 {
		result.Status = "completed"
		result.Success = true
		pm.logger.WithField("topic", topicName).Info("No consumers to rebalance")
		return result, nil
	}
	
	// Calculate optimal assignment
	newAssignment := pm.calculateOptimalAssignment(assignment.Partitions, consumers)
	
	// Apply new assignment
	partitionsMoved := 0
	for consumerID, partitionIDs := range newAssignment {
		oldPartitions := assignment.ConsumerMap[consumerID]
		
		// Find moved partitions
		for _, partitionID := range partitionIDs {
			wasAssigned := false
			for _, oldPartitionID := range oldPartitions {
				if oldPartitionID == partitionID {
					wasAssigned = true
					break
				}
			}
			
			if !wasAssigned {
				// This partition was moved to this consumer
				partitionsMoved++
				
				detail := &PartitionRebalance{
					PartitionID:   partitionID,
					ToConsumer:    consumerID,
					MessagesMoved: assignment.Partitions[partitionID].MessageCount,
					Status:        "completed",
				}
				
				// Find the previous consumer
				for oldConsumerID, oldPartitions := range assignment.ConsumerMap {
					for _, oldPartitionID := range oldPartitions {
						if oldPartitionID == partitionID && oldConsumerID != consumerID {
							detail.FromConsumer = oldConsumerID
							break
						}
					}
				}
				
				result.Details = append(result.Details, detail)
			}
		}
		
		// Update assignment
		assignment.ConsumerMap[consumerID] = partitionIDs
		
		// Update partition consumer references
		for _, partitionID := range partitionIDs {
			if int(partitionID) < len(assignment.Partitions) {
				assignment.Partitions[partitionID].Consumers = []string{consumerID}
			}
		}
	}
	
	result.PartitionsMoved = partitionsMoved
	result.Status = "completed"
	result.Success = true
	
	pm.logger.WithFields(logrus.Fields{
		"topic":            topicName,
		"partitions_moved": partitionsMoved,
		"consumers":        len(consumers),
	}).Info("Partition rebalancing completed")
	
	return result, nil
}

// AddConsumer adds a consumer to a topic and triggers rebalancing if needed
func (pm *PartitionManager) AddConsumer(ctx context.Context, topicName, consumerID string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	
	assignment, exists := pm.assignments[topicName]
	if !exists {
		return fmt.Errorf("topic %s not found", topicName)
	}
	
	// Check if consumer already exists
	if _, exists := assignment.ConsumerMap[consumerID]; exists {
		return nil // Consumer already added
	}
	
	// Add consumer with empty partition list
	assignment.ConsumerMap[consumerID] = make([]int32, 0)
	
	// Trigger rebalancing in background
	go func() {
		if _, err := pm.RebalancePartitions(context.Background(), topicName); err != nil {
			pm.logger.WithError(err).WithField("topic", topicName).Warn("Failed to rebalance after adding consumer")
		}
	}()
	
	pm.logger.WithFields(logrus.Fields{
		"topic":       topicName,
		"consumer_id": consumerID,
	}).Info("Consumer added to topic")
	
	return nil
}

// RemoveConsumer removes a consumer from a topic and triggers rebalancing
func (pm *PartitionManager) RemoveConsumer(ctx context.Context, topicName, consumerID string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	
	assignment, exists := pm.assignments[topicName]
	if !exists {
		return fmt.Errorf("topic %s not found", topicName)
	}
	
	// Remove consumer
	delete(assignment.ConsumerMap, consumerID)
	
	// Remove consumer from partition references
	for _, partition := range assignment.Partitions {
		newConsumers := make([]string, 0, len(partition.Consumers))
		for _, consumer := range partition.Consumers {
			if consumer != consumerID {
				newConsumers = append(newConsumers, consumer)
			}
		}
		partition.Consumers = newConsumers
	}
	
	// Trigger rebalancing in background if there are still consumers
	if len(assignment.ConsumerMap) > 0 {
		go func() {
			if _, err := pm.RebalancePartitions(context.Background(), topicName); err != nil {
				pm.logger.WithError(err).WithField("topic", topicName).Warn("Failed to rebalance after removing consumer")
			}
		}()
	}
	
	pm.logger.WithFields(logrus.Fields{
		"topic":       topicName,
		"consumer_id": consumerID,
	}).Info("Consumer removed from topic")
	
	return nil
}

// UpdatePartitionStats updates statistics for a partition
func (pm *PartitionManager) UpdatePartitionStats(topicName string, partitionID int32, messageCount, size int64) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	
	assignment, exists := pm.assignments[topicName]
	if !exists {
		return
	}
	
	if int(partitionID) >= len(assignment.Partitions) {
		return
	}
	
	partition := assignment.Partitions[partitionID]
	partition.MessageCount += messageCount
	partition.Size += size
	partition.LastActivity = time.Now()
	
	if messageCount > 0 {
		partition.EndOffset += messageCount
	}
}

// CleanupPartitions removes partition assignments for a deleted topic
func (pm *PartitionManager) CleanupPartitions(ctx context.Context, topicName string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	
	delete(pm.assignments, topicName)
	
	pm.logger.WithField("topic", topicName).Info("Partition assignments cleaned up")
	
	return nil
}

// Helper methods

// calculateLoadBalance calculates load balance metrics for partitions
func (pm *PartitionManager) calculateLoadBalance(assignment *PartitionAssignment) *LoadBalanceInfo {
	if len(assignment.Partitions) == 0 {
		return &LoadBalanceInfo{
			IsBalanced:         true,
			ImbalanceRatio:     0,
			RecommendRebalance: false,
		}
	}
	
	// Calculate partition load (message count + size)
	loads := make([]float64, len(assignment.Partitions))
	totalLoad := float64(0)
	
	for i, partition := range assignment.Partitions {
		load := float64(partition.MessageCount) + float64(partition.Size)/1024.0 // Convert bytes to KB
		loads[i] = load
		totalLoad += load
	}
	
	if totalLoad == 0 {
		return &LoadBalanceInfo{
			IsBalanced:         true,
			ImbalanceRatio:     0,
			RecommendRebalance: false,
		}
	}
	
	// Calculate imbalance ratio (standard deviation / mean)
	avgLoad := totalLoad / float64(len(loads))
	variance := float64(0)
	
	for _, load := range loads {
		variance += (load - avgLoad) * (load - avgLoad)
	}
	
	stdDev := math.Sqrt(variance / float64(len(loads)))
	imbalanceRatio := stdDev / avgLoad
	
	// Thresholds for balance recommendation
	const (
		balancedThreshold           = 0.1 // 10% imbalance is acceptable
		rebalanceRecommendThreshold = 0.3 // 30% imbalance recommends rebalancing
	)
	
	isBalanced := imbalanceRatio <= balancedThreshold
	recommendRebalance := imbalanceRatio >= rebalanceRecommendThreshold
	
	reason := ""
	if !isBalanced {
		if recommendRebalance {
			reason = fmt.Sprintf("High load imbalance detected (%.1f%%). Rebalancing recommended.", imbalanceRatio*100)
		} else {
			reason = fmt.Sprintf("Moderate load imbalance detected (%.1f%%).", imbalanceRatio*100)
		}
	}
	
	return &LoadBalanceInfo{
		IsBalanced:         isBalanced,
		ImbalanceRatio:     imbalanceRatio,
		RecommendRebalance: recommendRebalance,
		Reason:             reason,
	}
}

// calculateOptimalAssignment calculates optimal partition assignment for consumers
func (pm *PartitionManager) calculateOptimalAssignment(partitions []*Partition, consumers []string) map[string][]int32 {
	assignment := make(map[string][]int32)
	
	if len(consumers) == 0 {
		return assignment
	}
	
	// Initialize assignment maps
	for _, consumerID := range consumers {
		assignment[consumerID] = make([]int32, 0)
	}
	
	// Simple round-robin assignment
	// In production, this would consider partition load and consumer capacity
	for i, partition := range partitions {
		consumerIndex := i % len(consumers)
		consumerID := consumers[consumerIndex]
		assignment[consumerID] = append(assignment[consumerID], partition.ID)
	}
	
	return assignment
}

// GetConsumerPartitions returns the partitions assigned to a consumer
func (pm *PartitionManager) GetConsumerPartitions(topicName, consumerID string) ([]int32, error) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	
	assignment, exists := pm.assignments[topicName]
	if !exists {
		return nil, fmt.Errorf("topic %s not found", topicName)
	}
	
	partitions, exists := assignment.ConsumerMap[consumerID]
	if !exists {
		return make([]int32, 0), nil
	}
	
	// Return a copy to avoid race conditions
	result := make([]int32, len(partitions))
	copy(result, partitions)
	
	return result, nil
}