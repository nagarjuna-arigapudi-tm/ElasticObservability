package jobs

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"ElasticObservability/pkg/logger"
	"ElasticObservability/pkg/types"
	"ElasticObservability/pkg/utils"
)

// UpdateAccessCredentials updates access credentials for clusters from CSV file
func UpdateAccessCredentials(ctx context.Context, params map[string]interface{}) error {
	logger.JobInfo("updateAccessCredentials", "Starting credentials update job")

	// Get CSV file name from parameters
	csvFileName, ok := params["csv_fileName"].(string)
	if !ok || csvFileName == "" {
		return fmt.Errorf("csv_fileName parameter is required")
	}

	// Parse CSV file
	parser := utils.NewCSVParser(csvFileName)
	if err := parser.Parse(); err != nil {
		return fmt.Errorf("failed to parse CSV: %w", err)
	}

	rows := parser.GetRows()
	logger.JobInfo("updateAccessCredentials", "Parsed %d rows from CSV", len(rows))

	updatedCount := 0
	skippedCount := 0
	notFoundCount := 0

	for rowIdx, row := range rows {
		// Get cluster name
		clusterName := strings.TrimSpace(utils.GetValue(row, "ClusterName"))
		if clusterName == "" {
			logger.JobWarn("updateAccessCredentials", "Row %d: Empty cluster name, skipping", rowIdx+1)
			skippedCount++
			continue
		}

		// Check if cluster exists
		types.ClustersMu.Lock()
		cluster, exists := types.AllClusters[clusterName]
		if !exists {
			types.ClustersMu.Unlock()
			logger.JobWarn("updateAccessCredentials", "Row %d: Cluster %s not found, skipping", rowIdx+1, clusterName)
			notFoundCount++
			continue
		}

		// Parse and update AccessCred
		updateClusterCredentials(cluster, row)
		types.ClustersMu.Unlock()

		updatedCount++
		logger.JobInfo("updateAccessCredentials", "Row %d: Updated credentials for cluster: %s", rowIdx+1, clusterName)
	}

	logger.JobInfo("updateAccessCredentials", "Completed: %d clusters updated, %d not found, %d skipped",
		updatedCount, notFoundCount, skippedCount)

	return nil
}

func updateClusterCredentials(cluster *types.ClusterData, row map[string]string) {
	// Parse PrefferedAccess (intentionally matching the typo from requirements)
	preferredAccessStr := strings.TrimSpace(utils.GetValue(row, "PrefferedAccess"))
	if preferredAccessStr != "" {
		if preferredAccess, err := strconv.ParseUint(preferredAccessStr, 10, 8); err == nil {
			cluster.AccessCred.Preferred = uint8(preferredAccess)
		}
	}

	// Update APIKey
	apiKey := strings.TrimSpace(utils.GetValue(row, "APIKey"))
	if apiKey != "" {
		cluster.AccessCred.APIKey = apiKey
	}

	// Update UserID
	userID := strings.TrimSpace(utils.GetValue(row, "UserID"))
	if userID != "" {
		cluster.AccessCred.UserID = userID
	}

	// Update Password
	password := strings.TrimSpace(utils.GetValue(row, "Password"))
	if password != "" {
		cluster.AccessCred.Password = password
	}

	// Update ClientCert
	clientCert := strings.TrimSpace(utils.GetValue(row, "ClientCert"))
	if clientCert != "" {
		cluster.AccessCred.ClientCert = clientCert
	}

	// Update ClientKey
	clientKey := strings.TrimSpace(utils.GetValue(row, "ClientKey"))
	if clientKey != "" {
		cluster.AccessCred.ClientKey = clientKey
	}

	// Update CaCert
	caCert := strings.TrimSpace(utils.GetValue(row, "Cacert"))
	if caCert != "" {
		cluster.AccessCred.CaCert = caCert
	}
}
