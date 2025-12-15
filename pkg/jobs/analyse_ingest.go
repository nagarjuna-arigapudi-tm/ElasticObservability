package jobs

import (
	"context"

	"ElasticObservability/pkg/logger"
	"ElasticObservability/pkg/types"
	"ElasticObservability/pkg/utils"
)

// AnalyseIngest analyzes indexing rates based on historical data
func AnalyseIngest(ctx context.Context, params map[string]interface{}) error {
	logger.JobInfo("analyseIngest", "Starting indexing rate analysis")

	// Get exclude list
	excludeClusters := make([]string, 0)
	if exclude, ok := params["excludeClusters"].([]interface{}); ok {
		for _, item := range exclude {
			if str, ok := item.(string); ok {
				excludeClusters = append(excludeClusters, str)
			}
		}
	}

	// Get a deep copy of all history (copying pointers)
	types.HistoryMu.RLock()
	historyCopy := make(map[string]*types.IndicesHistory)
	for clusterName, history := range types.AllHistory {
		if history != nil {
			historyCopy[clusterName] = history.GetCopy()
		}
	}
	types.HistoryMu.RUnlock()

	processedCount := 0
	skippedCount := 0

	// Process each cluster
	for clusterName, history := range historyCopy {
		// Skip excluded clusters
		if utils.Contains(excludeClusters, clusterName) {
			logger.JobInfo("analyseIngest", "Skipping excluded cluster: %s", clusterName)
			skippedCount++
			continue
		}

		// Calculate indexing rates for this cluster
		clusterRate, err := calculateClusterIndexingRate(clusterName, history)
		if err != nil {
			logger.JobWarn("analyseIngest", "Cluster %s: Failed to calculate rates: %v", clusterName, err)
			skippedCount++
			continue
		}

		// Store the indexing rate (thread-safe)
		types.IndexingRateMu.Lock()
		types.AllIndexingRate[clusterName] = clusterRate
		types.IndexingRateMu.Unlock()

		processedCount++
		logger.JobInfo("analyseIngest", "Cluster %s: Calculated rates for %d indices",
			clusterName, len(clusterRate.MapIndices))
	}

	logger.JobInfo("analyseIngest", "Completed: %d clusters processed, %d skipped", processedCount, skippedCount)
	return nil
}

func calculateClusterIndexingRate(clusterName string, history *types.IndicesHistory) (*types.ClusterIndexingRate, error) {
	// Find the latest snapshot index
	latestIdx := history.GetLatestIndex()
	if latestIdx < 0 {
		return nil, nil // No data yet
	}

	// Get snapshot pointers for different time windows
	p_0 := history.Ptr[latestIdx]
	if p_0 == nil {
		return nil, nil
	}

	// Find previous snapshots for time windows
	// Assuming 3 minute intervals: p_1 = 3min ago, p_5 = 15min ago, p_20 = 60min ago
	var p_1, p_5, p_20 *types.IndicesSnapShot

	if latestIdx >= 1 && history.Ptr[latestIdx-1] != nil {
		p_1 = history.Ptr[latestIdx-1]
	}

	if latestIdx >= 5 && history.Ptr[latestIdx-5] != nil {
		p_5 = history.Ptr[latestIdx-5]
	}

	if latestIdx >= 20 && history.Ptr[latestIdx-20] != nil {
		p_20 = history.Ptr[latestIdx-20]
	}

	// Get timestamps
	t_0 := p_0.SnapShotTime
	var t_1, t_5, t_20 int64

	if p_1 != nil {
		t_1 = p_1.SnapShotTime
	}
	if p_5 != nil {
		t_5 = p_5.SnapShotTime
	}
	if p_20 != nil {
		t_20 = p_20.SnapShotTime
	}

	// Create cluster indexing rate structure
	clusterRate := &types.ClusterIndexingRate{
		Timestamp:  t_0,
		MapIndices: make(map[string]*types.IndexingRate),
	}

	// Process each index in the latest snapshot
	for indexBase, currentIndex := range p_0.MapIndices {
		if currentIndex == nil {
			continue
		}

		indexRate := &types.IndexingRate{
			NumberOfShards: currentIndex.PrimaryShards,
			FromCreation:   -1,
			Last3Minutes:   -1,
			Last15Minutes:  -1,
			Last60Minutes:  -1,
		}

		numberOfShards := float64(currentIndex.PrimaryShards)
		if numberOfShards == 0 {
			numberOfShards = 1 // Avoid division by zero
		}

		// Calculate rate from creation
		if currentIndex.CreationTime > 0 && t_0 > currentIndex.CreationTime {
			timeDiff := float64(t_0 - currentIndex.CreationTime)
			if timeDiff > 0 {
				// bytes/ms per shard * 1000 to get bytes/s per shard
				indexRate.FromCreation = (float64(currentIndex.PrimaryStorage) * 1000) / (numberOfShards * timeDiff)
			}
		}

		// Calculate rate for last 3 minutes
		if p_1 != nil && t_1 > 0 {
			if prevIndex, exists := p_1.MapIndices[indexBase]; exists && prevIndex != nil {
				// Check if index rolled over
				if currentIndex.SeqNo == prevIndex.SeqNo {
					timeDiff := float64(t_0 - t_1)
					if timeDiff > 0 && currentIndex.PrimaryStorage >= prevIndex.PrimaryStorage {
						storageDiff := float64(currentIndex.PrimaryStorage - prevIndex.PrimaryStorage)
						indexRate.Last3Minutes = (storageDiff * 1000) / (numberOfShards * timeDiff)
					}
				}
			}
		}

		// Calculate rate for last 15 minutes
		if p_5 != nil && t_5 > 0 {
			if prevIndex, exists := p_5.MapIndices[indexBase]; exists && prevIndex != nil {
				if currentIndex.SeqNo == prevIndex.SeqNo {
					timeDiff := float64(t_0 - t_5)
					if timeDiff > 0 && currentIndex.PrimaryStorage >= prevIndex.PrimaryStorage {
						storageDiff := float64(currentIndex.PrimaryStorage - prevIndex.PrimaryStorage)
						indexRate.Last15Minutes = (storageDiff * 1000) / (numberOfShards * timeDiff)
					}
				}
			}
		}

		// Calculate rate for last 60 minutes
		if p_20 != nil && t_20 > 0 {
			if prevIndex, exists := p_20.MapIndices[indexBase]; exists && prevIndex != nil {
				if currentIndex.SeqNo == prevIndex.SeqNo {
					timeDiff := float64(t_0 - t_20)
					if timeDiff > 0 && currentIndex.PrimaryStorage >= prevIndex.PrimaryStorage {
						storageDiff := float64(currentIndex.PrimaryStorage - prevIndex.PrimaryStorage)
						indexRate.Last60Minutes = (storageDiff * 1000) / (numberOfShards * timeDiff)
					}
				}
			}
		}

		clusterRate.MapIndices[indexBase] = indexRate
	}

	return clusterRate, nil
}
