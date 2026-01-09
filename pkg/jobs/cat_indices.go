package jobs

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"ElasticObservability/pkg/config"
	"ElasticObservability/pkg/logger"
	"ElasticObservability/pkg/types"
	"ElasticObservability/pkg/utils"
)

// CatIndicesResponse represents the response from _cat/indices API
type CatIndicesResponse []map[string]interface{}

// RunCatIndices fetches indices information from all clusters
func RunCatIndices(ctx context.Context, params map[string]interface{}) error {
	logger.JobInfo("runCatIndices", "Starting indices fetch job")

	// Get exclude list
	excludeClusters := make([]string, 0)
	if exclude, ok := params["excludeClusters"].([]interface{}); ok {
		for _, item := range exclude {
			if str, ok := item.(string); ok {
				excludeClusters = append(excludeClusters, str)
			}
		}
	}

	// Get exclude indices patterns (optional)
	excludeIndices := make([]string, 0)
	if exclude, ok := params["excludeIndices"].([]interface{}); ok {
		for _, item := range exclude {
			if str, ok := item.(string); ok && str != "" {
				excludeIndices = append(excludeIndices, str)
			}
		}
	}

	// Get include only indices patterns (optional)
	includeOnlyIndices := make([]string, 0)
	if include, ok := params["includeOnlyIndices"].([]interface{}); ok {
		for _, item := range include {
			if str, ok := item.(string); ok && str != "" {
				includeOnlyIndices = append(includeOnlyIndices, str)
			}
		}
	}

	// Log filtering configuration
	if len(includeOnlyIndices) > 0 {
		logger.JobInfo("runCatIndices", "Index filter: includeOnlyIndices enabled with %d patterns (excludeIndices ignored)", len(includeOnlyIndices))
	} else if len(excludeIndices) > 0 {
		logger.JobInfo("runCatIndices", "Index filter: excludeIndices enabled with %d patterns", len(excludeIndices))
	}

	types.ClustersMu.RLock()
	clustersCopy := make(map[string]*types.ClusterData)
	for name, cluster := range types.AllClusters {
		clustersCopy[name] = cluster
	}
	types.ClustersMu.RUnlock()

	successCount := 0
	failedCount := 0
	currentTime := utils.TimeNowMillis()

	for clusterName, cluster := range clustersCopy {
		// Skip excluded clusters
		if utils.Contains(excludeClusters, clusterName) {
			logger.JobInfo("runCatIndices", "Skipping excluded cluster: %s", clusterName)
			continue
		}

		// Skip clusters without credentials
		if cluster.AccessCred.Preferred == 0 {
			logger.JobInfo("runCatIndices", "Skipping cluster %s: No credentials available (Preferred=0)", clusterName)
			failedCount++
			continue
		}

		// Skip if no active endpoint
		if cluster.ActiveEndpoint == "" {
			logger.JobWarn("runCatIndices", "Cluster %s: No active endpoint, skipping", clusterName)
			failedCount++
			continue
		}

		// Fetch indices
		indices, err := fetchIndices(cluster)
		if err != nil {
			logger.JobError("runCatIndices", "Cluster %s: Failed to fetch indices: %v", clusterName, err)
			failedCount++
			continue
		}

		// Process and store indices
		snapshot := &types.IndicesSnapShot{
			SnapShotTime: currentTime,
			MapIndices:   make(map[string]*types.IndexInfo),
		}

		totalFetched := len(indices)
		filteredCount := 0
		duplicateCount := 0
		indexBaseSeen := make(map[string]bool) // Track index bases to avoid duplicates

		for _, idx := range indices {
			indexInfo := parseIndexInfo(idx)
			if indexInfo != nil {
				// Apply index filtering
				if shouldIncludeIndex(indexInfo.Index, includeOnlyIndices, excludeIndices) {
					// Only add if this indexBase hasn't been seen before
					if !indexBaseSeen[indexInfo.IndexBase] {
						snapshot.MapIndices[indexInfo.Index] = indexInfo // Store by index name, not indexBase
						indexBaseSeen[indexInfo.IndexBase] = true
					} else {
						duplicateCount++ // Skip duplicate indexBase
					}
				} else {
					filteredCount++
				}
			}
		}

		// Store in history
		types.HistoryMu.Lock()
		history, exists := types.AllHistory[clusterName]
		if !exists {
			history = types.NewIndicesHistory(config.Global.HistoryForIndices)
			types.AllHistory[clusterName] = history
		}
		history.AddSnapshot(snapshot)
		types.HistoryMu.Unlock()

		successCount++
		if filteredCount > 0 || duplicateCount > 0 {
			logger.JobInfo("runCatIndices", "Cluster %s: Fetched %d indices, filtered %d, duplicates %d, stored %d",
				clusterName, totalFetched, filteredCount, duplicateCount, len(snapshot.MapIndices))
		} else {
			logger.JobInfo("runCatIndices", "Cluster %s: Fetched %d indices, stored %d",
				clusterName, totalFetched, len(snapshot.MapIndices))
		}
	}

	logger.JobInfo("runCatIndices", "Completed: %d clusters succeeded, %d failed", successCount, failedCount)
	return nil
}

// shouldIncludeIndex determines if an index should be included based on filter patterns
func shouldIncludeIndex(indexName string, includeOnlyPatterns []string, excludePatterns []string) bool {
	// If includeOnlyPatterns is specified, it takes precedence
	if len(includeOnlyPatterns) > 0 {
		for _, pattern := range includeOnlyPatterns {
			if matched, err := regexp.MatchString(pattern, indexName); err == nil && matched {
				return true
			}
		}
		return false // Not in include list
	}

	// If excludePatterns is specified, check if index matches any
	if len(excludePatterns) > 0 {
		for _, pattern := range excludePatterns {
			if matched, err := regexp.MatchString(pattern, indexName); err == nil && matched {
				return false // Matches exclude pattern
			}
		}
	}

	return true // Include by default
}

func fetchIndices(cluster *types.ClusterData) (CatIndicesResponse, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cluster.InsecureTLS,
			},
		},
	}

	url := fmt.Sprintf("%s/_cat/indices?format=json&pretty&h=health,status,docs.count,index,pri,creation.date,store.size,pri.store.size&s=creation.date:desc",
		cluster.ActiveEndpoint)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication
	cred := &cluster.AccessCred
	if cred.APIKey != "" {
		req.Header.Set("Authorization", "ApiKey "+cred.APIKey)
	} else if cred.UserID != "" && cred.Password != "" {
		req.SetBasicAuth(cred.UserID, cred.Password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var result CatIndicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

func parseIndexInfo(data map[string]interface{}) *types.IndexInfo {
	indexName, ok := data["index"].(string)
	if !ok || indexName == "" {
		return nil
	}

	indexBase, seqNo := utils.ParseIndexName(indexName)

	// Parse health
	health := uint8(0)
	if healthStr, ok := data["health"].(string); ok {
		health = utils.ParseHealth(healthStr)
	}

	// Parse status
	isOpen := false
	if statusStr, ok := data["status"].(string); ok {
		isOpen = utils.ParseStatus(statusStr)
	}

	// Parse doc count
	docCount := uint64(0)
	if docsStr, ok := data["docs.count"].(string); ok {
		if val, err := strconv.ParseUint(docsStr, 10, 64); err == nil {
			docCount = val
		}
	}

	// Parse primary shards
	primaryShards := uint8(0)
	if priStr, ok := data["pri"].(string); ok {
		if val, err := strconv.ParseUint(priStr, 10, 8); err == nil {
			primaryShards = uint8(val)
		}
	}

	// Parse creation time
	creationTime := int64(0)
	if creationStr, ok := data["creation.date"].(string); ok {
		if val, err := strconv.ParseInt(creationStr, 10, 64); err == nil {
			creationTime = val
		}
	}

	// Parse total storage
	totalStorage := uint64(0)
	if ssStr, ok := data["store.size"].(string); ok {
		if val, err := utils.ParseStorageSize(ssStr); err == nil {
			totalStorage = val
		}
	}

	// Parse primary storage
	primaryStorage := uint64(0)
	if priStoreStr, ok := data["pri.store.size"].(string); ok {
		if val, err := utils.ParseStorageSize(priStoreStr); err == nil {
			primaryStorage = val
		}
	}

	return &types.IndexInfo{
		Health:         health,
		IsOpen:         isOpen,
		DocCount:       docCount,
		Index:          indexName,
		IndexBase:      indexBase,
		SeqNo:          seqNo,
		PrimaryShards:  primaryShards,
		CreationTime:   creationTime,
		TotalStorage:   totalStorage,
		PrimaryStorage: primaryStorage,
	}
}
