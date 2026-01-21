package jobs

import (
	"context"

	"ElasticObservability/pkg/logger"
	"ElasticObservability/pkg/types"
	"ElasticObservability/pkg/utils"
)

// UpdateCurrentMasterEndPoints updates the global map of current master node endpoints for all clusters
func UpdateCurrentMasterEndPoints(ctx context.Context, params map[string]interface{}) error {
	logger.JobInfo("updateCurrentMasterEndPoints", "Starting master endpoints update job")

	// Get list of clusters
	types.ClustersMu.RLock()
	clusterList := make([]string, 0, len(types.AllClustersList))
	for _, clusterName := range types.AllClustersList {
		if cluster, exists := types.AllClusters[clusterName]; exists && cluster.ActiveEndpoint != "" {
			clusterList = append(clusterList, clusterName)
		}
	}
	types.ClustersMu.RUnlock()

	logger.JobInfo("updateCurrentMasterEndPoints", "Processing %d clusters with active endpoints", len(clusterList))

	successCount := 0
	failCount := 0

	// Process each cluster
	for _, clusterName := range clusterList {
		// Get master endpoint for this cluster
		masterEndpoint := utils.GetCurrentMasterEndpointForCluster(clusterName)

		if masterEndpoint == "" {
			logger.JobWarn("updateCurrentMasterEndPoints", "Could not determine master endpoint for cluster: %s", clusterName)
			failCount++
			continue
		}

		// Update global map (thread-safe)
		types.CurrentMasterEndPtsMu.Lock()
		types.AllCurrentMasterEndPoints[clusterName] = masterEndpoint
		types.CurrentMasterEndPtsMu.Unlock()

		logger.JobInfo("updateCurrentMasterEndPoints", "Updated master endpoint for cluster %s: %s", clusterName, masterEndpoint)
		successCount++
	}

	logger.JobInfo("updateCurrentMasterEndPoints", "Completed: %d succeeded, %d failed", successCount, failCount)
	return nil
}
