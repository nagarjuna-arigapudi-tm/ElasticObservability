package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"ElasticObservability/pkg/config"
	"ElasticObservability/pkg/logger"
	"ElasticObservability/pkg/types"
	"ElasticObservability/pkg/utils"
)

// UpdateStatsByDay maintains daily statistics for indices
func UpdateStatsByDay(ctx context.Context, params map[string]interface{}) error {
	logger.JobInfo("updateStatsByDay", "Starting daily statistics update job")

	// Get exclude list
	excludeClusters := make([]string, 0)
	if exclude, ok := params["excludeClusters"].([]interface{}); ok {
		for _, item := range exclude {
			if str, ok := item.(string); ok && str != "" {
				excludeClusters = append(excludeClusters, str)
			}
		}
	}

	// Get backup file location
	backupFile := config.Global.BackupOfStatsInDays
	if backupFile == "" {
		backupFile = "./data/backup/statsInDays.json"
	}

	historyDays := config.Global.HistoryOfStatsInDays
	if historyDays == 0 {
		historyDays = 30
	}

	// Check if backup exists
	backupExists := fileExists(backupFile)

	if backupExists {
		logger.JobInfo("updateStatsByDay", "Backup file found at %s, restoring...", backupFile)
		if err := restoreFromBackup(backupFile); err != nil {
			logger.JobError("updateStatsByDay", "Failed to restore from backup: %v", err)
			return err
		}

		// Remove excluded clusters from restored data
		types.StatsByDayMu.Lock()
		for _, clusterName := range excludeClusters {
			if _, exists := types.AllStatsByDay[clusterName]; exists {
				delete(types.AllStatsByDay, clusterName)
				logger.JobInfo("updateStatsByDay", "Removed excluded cluster from stats: %s", clusterName)
			}
		}
		types.StatsByDayMu.Unlock()

		// Check if 24 hours have passed since last update
		if err := handleExistingStats(historyDays); err != nil {
			logger.JobError("updateStatsByDay", "Failed to handle existing stats: %v", err)
			return err
		}
	} else {
		logger.JobInfo("updateStatsByDay", "No backup file found, initializing new statistics")
		if err := initializeStats(excludeClusters, historyDays); err != nil {
			logger.JobError("updateStatsByDay", "Failed to initialize stats: %v", err)
			return err
		}
	}

	// Persist to backup file
	if err := saveToBackup(backupFile); err != nil {
		logger.JobError("updateStatsByDay", "Failed to save backup: %v", err)
		return err
	}

	logger.JobInfo("updateStatsByDay", "Daily statistics update completed successfully")
	return nil
}

// fileExists checks if a file exists
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// restoreFromBackup restores AllStatsByDay from backup file
func restoreFromBackup(backupFile string) error {
	data, err := os.ReadFile(backupFile)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	restored := make(map[string]*types.IndicesStatsByDay)
	if err := json.Unmarshal(data, &restored); err != nil {
		return fmt.Errorf("failed to unmarshal backup data: %w", err)
	}

	types.StatsByDayMu.Lock()
	types.AllStatsByDay = restored
	types.StatsByDayMu.Unlock()

	logger.JobInfo("updateStatsByDay", "Restored statistics for %d clusters from backup", len(restored))
	return nil
}

// saveToBackup saves AllStatsByDay to backup file
func saveToBackup(backupFile string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(backupFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	types.StatsByDayMu.RLock()
	data, err := json.MarshalIndent(types.AllStatsByDay, "", "  ")
	types.StatsByDayMu.RUnlock()

	if err != nil {
		return fmt.Errorf("failed to marshal stats data: %w", err)
	}

	if err := os.WriteFile(backupFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	logger.JobInfo("updateStatsByDay", "Saved statistics to backup file: %s", backupFile)
	return nil
}

// handleExistingStats handles existing statistics after restore
func handleExistingStats(historyDays uint8) error {
	currentTime := utils.TimeNowMillis()

	types.StatsByDayMu.RLock()
	if len(types.AllStatsByDay) == 0 {
		types.StatsByDayMu.RUnlock()
		return fmt.Errorf("no statistics found after restore")
	}

	// Get first cluster's last update time
	var lastUpdateTime int64
	for _, stats := range types.AllStatsByDay {
		lastUpdateTime = stats.LastUpdateTime
		break
	}
	types.StatsByDayMu.RUnlock()

	timeDiff := currentTime - lastUpdateTime
	hoursDiff := float64(timeDiff) / (60 * 60 * 1000) // convert ms to hours

	if hoursDiff < 24 {
		logger.JobInfo("updateStatsByDay", "Last update was %.1f hours ago, no update needed", hoursDiff)
		return nil
	}

	// Calculate how many days forward to move
	daysForward := int(math.Ceil(hoursDiff / 24))
	logger.JobInfo("updateStatsByDay", "Last update was %.1f hours ago (%d days), updating statistics", hoursDiff, daysForward)

	// Update statistics for all clusters
	return updateAllClustersStats(daysForward, historyDays)
}

// initializeStats initializes statistics from scratch
func initializeStats(excludeClusters []string, historyDays uint8) error {
	// Get list of clusters to process
	types.ClustersMu.RLock()
	allStatsClustersList := make([]string, 0)
	for _, clusterName := range types.AllClustersList {
		if !utils.Contains(excludeClusters, clusterName) {
			allStatsClustersList = append(allStatsClustersList, clusterName)
		}
	}
	types.ClustersMu.RUnlock()

	logger.JobInfo("updateStatsByDay", "Initializing statistics for %d clusters", len(allStatsClustersList))

	currentTime := utils.TimeNowMillis()

	// Initialize stats for each cluster
	for _, clusterName := range allStatsClustersList {
		types.HistoryMu.RLock()
		history, exists := types.AllHistory[clusterName]
		types.HistoryMu.RUnlock()

		if !exists {
			logger.JobWarn("updateStatsByDay", "No history found for cluster %s, skipping", clusterName)
			continue
		}

		// Get latest snapshot
		latestIdx := history.GetLatestIndex()
		if latestIdx < 0 {
			logger.JobWarn("updateStatsByDay", "No snapshots found for cluster %s, skipping", clusterName)
			continue
		}

		snapshot := history.Ptr[latestIdx]
		if snapshot == nil {
			logger.JobWarn("updateStatsByDay", "Latest snapshot is nil for cluster %s, skipping", clusterName)
			continue
		}

		// Create IndicesStatsByDay for this cluster
		clusterStats := &types.IndicesStatsByDay{
			LastUpdateTime: currentTime,
			StatHistory:    make(map[string]*types.IndexStatHistory),
		}

		// Populate stats for each index
		for indexName, indexInfo := range snapshot.MapIndices {
			statHistory := &types.IndexStatHistory{
				IndexName: indexName,
				SizeOfPtr: historyDays,
				StatsPtr:  make([]*types.IndexStat, historyDays+1),
			}

			// Store current stats in first position
			statHistory.StatsPtr[0] = &types.IndexStat{
				StatTime:  snapshot.SnapShotTime,
				TotalSize: indexInfo.TotalStorage,
				DocCount:  indexInfo.DocCount,
			}

			clusterStats.StatHistory[indexName] = statHistory
		}

		types.StatsByDayMu.Lock()
		types.AllStatsByDay[clusterName] = clusterStats
		types.StatsByDayMu.Unlock()

		logger.JobInfo("updateStatsByDay", "Initialized stats for cluster %s with %d indices", clusterName, len(clusterStats.StatHistory))
	}

	return nil
}

// updateAllClustersStats updates statistics for all clusters
func updateAllClustersStats(daysForward int, historyDays uint8) error {
	currentTime := utils.TimeNowMillis()

	types.StatsByDayMu.Lock()
	defer types.StatsByDayMu.Unlock()

	for clusterName, clusterStats := range types.AllStatsByDay {
		// Get latest history for this cluster
		types.HistoryMu.RLock()
		history, exists := types.AllHistory[clusterName]
		types.HistoryMu.RUnlock()

		if !exists {
			logger.JobWarn("updateStatsByDay", "No history found for cluster %s, skipping update", clusterName)
			continue
		}

		latestIdx := history.GetLatestIndex()
		if latestIdx < 0 {
			logger.JobWarn("updateStatsByDay", "No snapshots found for cluster %s, skipping update", clusterName)
			continue
		}

		snapshot := history.Ptr[latestIdx]
		if snapshot == nil {
			logger.JobWarn("updateStatsByDay", "Latest snapshot is nil for cluster %s, skipping update", clusterName)
			continue
		}

		// Get current indices from history
		indicesInHistory := make(map[string]bool)
		for indexName := range snapshot.MapIndices {
			indicesInHistory[indexName] = true
		}

		// Get indices from stats
		indicesInStats := make(map[string]bool)
		for indexName := range clusterStats.StatHistory {
			indicesInStats[indexName] = true
		}

		// Remove indices that are in stats but not in history (rolled over)
		for indexName := range indicesInStats {
			if !indicesInHistory[indexName] {
				delete(clusterStats.StatHistory, indexName)
				logger.JobInfo("updateStatsByDay", "Removed rolled-over index %s from cluster %s stats", indexName, clusterName)
			}
		}

		// Update existing indices and add new ones
		for indexName, indexInfo := range snapshot.MapIndices {
			statHistory, exists := clusterStats.StatHistory[indexName]

			if exists {
				// Roll forward pointers by daysForward
				rollStatsForward(statHistory, daysForward)
			} else {
				// Create new stat history for new index
				statHistory = &types.IndexStatHistory{
					IndexName: indexName,
					SizeOfPtr: historyDays,
					StatsPtr:  make([]*types.IndexStat, historyDays+1),
				}
				clusterStats.StatHistory[indexName] = statHistory
				logger.JobInfo("updateStatsByDay", "Added new index %s to cluster %s stats", indexName, clusterName)
			}

			// Store current stats in position 0
			statHistory.StatsPtr[0] = &types.IndexStat{
				StatTime:  snapshot.SnapShotTime,
				TotalSize: indexInfo.TotalStorage,
				DocCount:  indexInfo.DocCount,
			}
		}

		// Update last update time
		clusterStats.LastUpdateTime = currentTime
		logger.JobInfo("updateStatsByDay", "Updated stats for cluster %s with %d indices", clusterName, len(clusterStats.StatHistory))
	}

	return nil
}

// rollStatsForward rolls statistics pointers forward by specified number of days
func rollStatsForward(statHistory *types.IndexStatHistory, daysForward int) {
	if daysForward <= 0 {
		return
	}

	size := int(statHistory.SizeOfPtr)

	// If rolling forward more than size, all will be nil except position 0
	if daysForward > size {
		for i := 1; i <= size; i++ {
			statHistory.StatsPtr[i] = nil
		}
		return
	}

	// Roll pointers forward
	for i := size; i >= daysForward; i-- {
		statHistory.StatsPtr[i] = statHistory.StatsPtr[i-daysForward]
	}

	// Set positions 1 to daysForward-1 to nil
	for i := 1; i < daysForward; i++ {
		statHistory.StatsPtr[i] = nil
	}
}
