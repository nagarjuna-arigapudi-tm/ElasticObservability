package jobs

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"ElasticObservability/pkg/logger"
	"ElasticObservability/pkg/types"
	"ElasticObservability/pkg/utils"
)

// GetTDataWriteBulk_sTasks collects bulk write task data from Elasticsearch clusters
// Processes clusters in parallel using goroutines with proper synchronization
func GetTDataWriteBulk_sTasks(ctx context.Context, params map[string]interface{}) error {
	logger.JobInfo("getTDataWriteBulk_sTasks", "Starting bulk write tasks monitoring job")

	// Get parameters
	excludeClusters := getStringSliceParam(params, "excludeClusters")
	includeClusters := getStringSliceParam(params, "includeClusters")
	historySize := getIntParam(params, "historySize", 60)
	insecureTLS := getBoolParam(params, "insecureTLS", false)
	maxConcurrent := getIntParam(params, "maxConcurrent", 9) // Default: process 5 clusters concurrently

	// Validate and adjust historySize
	if historySize < 10 {
		historySize = 10
		logger.JobWarn("getTDataWriteBulk_sTasks", "historySize too small, using minimum value: 10")
	} else if historySize > 180 {
		historySize = 180
		logger.JobWarn("getTDataWriteBulk_sTasks", "historySize too large, using maximum value: 180")
	}

	// Validate maxConcurrent
	if maxConcurrent < 1 {
		maxConcurrent = 1
	} else if maxConcurrent > 20 {
		maxConcurrent = 20
		logger.JobWarn("getTDataWriteBulk_sTasks", "maxConcurrent too large, using maximum value: 20")
	}

	logger.JobInfo("getTDataWriteBulk_sTasks", "Config: historySize=%d, insecureTLS=%v, maxConcurrent=%d",
		historySize, insecureTLS, maxConcurrent)

	// Build cluster list
	clusterList := buildClusterList(includeClusters, excludeClusters)
	logger.JobInfo("getTDataWriteBulk_sTasks", "Processing %d clusters in parallel", len(clusterList))

	// Process clusters in parallel with concurrency limit
	type result struct {
		clusterName string
		err         error
	}

	results := make(chan result, len(clusterList))
	semaphore := make(chan struct{}, maxConcurrent) // Limit concurrent goroutines

	// Launch goroutines for each cluster
	for _, clusterName := range clusterList {
		// Acquire semaphore slot
		semaphore <- struct{}{}

		go func(name string) {
			defer func() { <-semaphore }() // Release semaphore slot

			err := processClusterBulkTasks(ctx, name, uint(historySize), insecureTLS)
			results <- result{clusterName: name, err: err}
		}(clusterName)
	}

	// Collect results
	successCount := 0
	failCount := 0

	for i := 0; i < len(clusterList); i++ {
		res := <-results
		if res.err != nil {
			logger.JobError("getTDataWriteBulk_sTasks", "Failed to process cluster %s: %v", res.clusterName, res.err)
			failCount++
		} else {
			successCount++
		}
	}

	logger.JobInfo("getTDataWriteBulk_sTasks", "Completed: %d succeeded, %d failed", successCount, failCount)
	return nil
}

// buildClusterList creates the list of clusters to process
func buildClusterList(includeClusters, excludeClusters []string) []string {
	types.ClustersMu.RLock()
	defer types.ClustersMu.RUnlock()

	if len(includeClusters) > 0 {
		// Use included clusters, but validate they exist
		validClusters := make([]string, 0, len(includeClusters))
		for _, clusterName := range includeClusters {
			if utils.Contains(types.AllClustersList, clusterName) {
				validClusters = append(validClusters, clusterName)
			} else {
				logger.JobWarn("getTDataWriteBulk_sTasks", "Cluster %s in includeClusters not found in global cluster list", clusterName)
			}
		}
		return validClusters
	}

	// Use all clusters minus excluded ones
	clusterList := make([]string, 0, len(types.AllClustersList))
	for _, clusterName := range types.AllClustersList {
		if !utils.Contains(excludeClusters, clusterName) {
			clusterList = append(clusterList, clusterName)
		}
	}
	return clusterList
}

// processClusterBulkTasks processes bulk task data for a single cluster
func processClusterBulkTasks(ctx context.Context, clusterName string, historySize uint, insecureTLS bool) error {
	// Get master endpoint for cluster
	types.CurrentMasterEndPtsMu.RLock()
	masterEndpoint, exists := types.AllCurrentMasterEndPoints[clusterName]
	types.CurrentMasterEndPtsMu.RUnlock()

	if !exists || masterEndpoint == "" {
		return fmt.Errorf("no master endpoint found for cluster %s", clusterName)
	}

	// Ensure proper endpoint formatting
	endpoint := strings.TrimSuffix(masterEndpoint, "/") + "/_tasks?pretty&human&detailed=true"

	// Get cluster data for authentication
	types.ClustersMu.RLock()
	cluster, clusterExists := types.AllClusters[clusterName]
	types.ClustersMu.RUnlock()

	if !clusterExists {
		return fmt.Errorf("cluster %s not found in AllClusters", clusterName)
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecureTLS || cluster.InsecureTLS,
			},
		},
	}

	// Create and execute request
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	utils.AddAuthentication(req, &cluster.AccessCred)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var tasksResponse map[string]interface{}
	if err := json.Unmarshal(body, &tasksResponse); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Process tasks and create cluster data
	clusterData := parseTasksResponse(tasksResponse, clusterName, cluster)

	// Update global history
	updateClusterTasksHistory(clusterName, clusterData, historySize)

	logger.JobInfo("getTDataWriteBulk_sTasks", "Successfully processed cluster %s: %d nodes, %d indices",
		clusterName, len(clusterData.DataWriteBulk_sTasksByNode), len(clusterData.DataWriteBulk_sTasksByIndex))

	return nil
}

// parseTasksResponse parses the _tasks API response and creates ClusterDataWriteBulk_sTasks
func parseTasksResponse(response map[string]interface{}, clusterName string, cluster *types.ClusterData) *types.ClusterDataWriteBulk_sTasks {
	clusterData := &types.ClusterDataWriteBulk_sTasks{
		SnapShotTime:                time.Now().Unix(),
		DataWriteBulk_sTasksByNode:  make(map[string]*types.NodeDataWriteBulk_sTasks),
		DataWriteBulk_sTasksByIndex: make(map[string]*types.AggShardTaskDataWriteBulk_s),
	}

	// Parse nodes
	nodes, ok := response["nodes"].(map[string]interface{})
	if !ok {
		return clusterData
	}

	// Process each node
	for _, nodeData := range nodes {
		nodeMap, ok := nodeData.(map[string]interface{})
		if !ok {
			continue
		}

		// Get hostname
		hostName, ok := nodeMap["host"].(string)
		if !ok {
			continue
		}

		// Get tasks for this node
		tasks, ok := nodeMap["tasks"].(map[string]interface{})
		if !ok {
			continue
		}

		// Process tasks for this node
		nodeTaskData := processNodeTasks(tasks, hostName, clusterName, cluster)
		if nodeTaskData != nil {
			clusterData.DataWriteBulk_sTasksByNode[hostName] = nodeTaskData
		}
	}

	// Build cluster-level aggregations
	buildClusterAggregations(clusterData)

	return clusterData
}

// processNodeTasks processes all tasks for a single node
func processNodeTasks(tasks map[string]interface{}, hostName, clusterName string, cluster *types.ClusterData) *types.NodeDataWriteBulk_sTasks {
	nodeData := &types.NodeDataWriteBulk_sTasks{
		DataWriteBulk_sByShard: make(map[string]*types.AggShardTaskDataWriteBulk_s),
	}

	// Get zone information if available
	nodeData.Zone = getNodeZone(hostName, clusterName, cluster)

	// Regex to match bulk write tasks
	bulkActionRegex := regexp.MustCompile(`^indices:data/write/bulk\[s\]`)

	// Regex to parse description: "requests[236], index[index03][2]"
	descRegex := regexp.MustCompile(`requests\[(\d+)\].*index\[([^\]]+)\]\[(\d+)\]`)

	// Process each task
	for _, taskData := range tasks {
		taskMap, ok := taskData.(map[string]interface{})
		if !ok {
			continue
		}

		// Check if this is a bulk write task
		action, ok := taskMap["action"].(string)
		if !ok || !bulkActionRegex.MatchString(action) {
			continue
		}

		// Parse description
		description, ok := taskMap["description"].(string)
		if !ok {
			continue
		}

		matches := descRegex.FindStringSubmatch(description)
		if len(matches) < 4 {
			continue
		}

		requests, _ := strconv.ParseUint(matches[1], 10, 32)
		indexName := matches[2]
		shardNum := matches[3]
		indexShard := fmt.Sprintf("%s_%s", indexName, shardNum)

		// Get running time in nanoseconds
		runningTimeNanos, ok := taskMap["running_time_in_nanos"].(float64)
		if !ok {
			continue
		}

		timeTakenMs := uint64(math.Round(runningTimeNanos / 1000000))

		// Update or create shard data
		shardData, exists := nodeData.DataWriteBulk_sByShard[indexShard]
		if !exists {
			shardData = &types.AggShardTaskDataWriteBulk_s{
				NumberOfTasks:     1,
				TotalRequests:     uint(requests),
				TotalTimeTaken_ms: timeTakenMs,
			}
			nodeData.DataWriteBulk_sByShard[indexShard] = shardData
		} else {
			shardData.NumberOfTasks++
			shardData.TotalRequests += uint(requests)
			shardData.TotalTimeTaken_ms += timeTakenMs
		}
	}

	// If no bulk tasks found, return nil
	if len(nodeData.DataWriteBulk_sByShard) == 0 {
		return nil
	}

	// Calculate node-level totals
	calculateNodeTotals(nodeData)

	// Sort shards
	sortNodeShards(nodeData)

	return nodeData
}

// getNodeZone retrieves the zone for a node
func getNodeZone(hostName, clusterName string, cluster *types.ClusterData) string {
	if cluster == nil || cluster.Nodes == nil {
		return ""
	}

	for _, node := range cluster.Nodes {
		if node.HostName == hostName {
			return node.Zone
		}
	}
	return ""
}

// calculateNodeTotals calculates the total values for a node
func calculateNodeTotals(nodeData *types.NodeDataWriteBulk_sTasks) {
	for _, shardData := range nodeData.DataWriteBulk_sByShard {
		nodeData.TotalWiteBulk_sTasks += uint(shardData.NumberOfTasks)
		nodeData.TotalWriteBulk_sRequests += shardData.TotalRequests
		nodeData.TotalWrietBulk_sTimeTaken_ms += shardData.TotalTimeTaken_ms
	}
}

// sortNodeShards sorts the shards by different criteria
func sortNodeShards(nodeData *types.NodeDataWriteBulk_sTasks) {
	shards := make([]string, 0, len(nodeData.DataWriteBulk_sByShard))
	for shard := range nodeData.DataWriteBulk_sByShard {
		shards = append(shards, shard)
	}

	// Sort by tasks (descending)
	sortedOnTasks := make([]string, len(shards))
	copy(sortedOnTasks, shards)
	sort.Slice(sortedOnTasks, func(i, j int) bool {
		return nodeData.DataWriteBulk_sByShard[sortedOnTasks[i]].NumberOfTasks >
			nodeData.DataWriteBulk_sByShard[sortedOnTasks[j]].NumberOfTasks
	})
	nodeData.SortedShardsOnTasks = sortedOnTasks

	// Sort by time taken (descending)
	sortedOnTime := make([]string, len(shards))
	copy(sortedOnTime, shards)
	sort.Slice(sortedOnTime, func(i, j int) bool {
		return nodeData.DataWriteBulk_sByShard[sortedOnTime[i]].TotalTimeTaken_ms >
			nodeData.DataWriteBulk_sByShard[sortedOnTime[j]].TotalTimeTaken_ms
	})
	nodeData.SortedShardsOnTimetaken = sortedOnTime

	// Sort by requests (descending)
	sortedOnRequests := make([]string, len(shards))
	copy(sortedOnRequests, shards)
	sort.Slice(sortedOnRequests, func(i, j int) bool {
		return nodeData.DataWriteBulk_sByShard[sortedOnRequests[i]].TotalRequests >
			nodeData.DataWriteBulk_sByShard[sortedOnRequests[j]].TotalRequests
	})
	nodeData.SortedShardsOnRequest = sortedOnRequests
}

// buildClusterAggregations builds cluster-level aggregations
func buildClusterAggregations(clusterData *types.ClusterDataWriteBulk_sTasks) {
	// Sort hosts
	hosts := make([]string, 0, len(clusterData.DataWriteBulk_sTasksByNode))
	for host := range clusterData.DataWriteBulk_sTasksByNode {
		hosts = append(hosts, host)
	}

	// Sort by tasks
	sortedHosts := make([]string, len(hosts))
	copy(sortedHosts, hosts)
	sort.Slice(sortedHosts, func(i, j int) bool {
		return clusterData.DataWriteBulk_sTasksByNode[sortedHosts[i]].TotalWiteBulk_sTasks >
			clusterData.DataWriteBulk_sTasksByNode[sortedHosts[j]].TotalWiteBulk_sTasks
	})
	clusterData.SortedHostsOnTasks = sortedHosts

	// Sort by time taken
	sortedHostsTime := make([]string, len(hosts))
	copy(sortedHostsTime, hosts)
	sort.Slice(sortedHostsTime, func(i, j int) bool {
		return clusterData.DataWriteBulk_sTasksByNode[sortedHostsTime[i]].TotalWrietBulk_sTimeTaken_ms >
			clusterData.DataWriteBulk_sTasksByNode[sortedHostsTime[j]].TotalWrietBulk_sTimeTaken_ms
	})
	clusterData.SortedHostsOnTimetaken = sortedHostsTime

	// Sort by requests
	sortedHostsReq := make([]string, len(hosts))
	copy(sortedHostsReq, hosts)
	sort.Slice(sortedHostsReq, func(i, j int) bool {
		return clusterData.DataWriteBulk_sTasksByNode[sortedHostsReq[i]].TotalWriteBulk_sRequests >
			clusterData.DataWriteBulk_sTasksByNode[sortedHostsReq[j]].TotalWriteBulk_sRequests
	})
	clusterData.SortedHostsOnRequest = sortedHostsReq

	// Build index-level aggregations
	for _, nodeData := range clusterData.DataWriteBulk_sTasksByNode {
		for indexShard, shardData := range nodeData.DataWriteBulk_sByShard {
			// Extract index name by removing trailing "_<number>"
			indexName := extractIndexName(indexShard)

			// Update or create index data
			indexData, exists := clusterData.DataWriteBulk_sTasksByIndex[indexName]
			if !exists {
				indexData = &types.AggShardTaskDataWriteBulk_s{
					NumberOfTasks:     shardData.NumberOfTasks,
					TotalRequests:     shardData.TotalRequests,
					TotalTimeTaken_ms: shardData.TotalTimeTaken_ms,
				}
				clusterData.DataWriteBulk_sTasksByIndex[indexName] = indexData
			} else {
				indexData.NumberOfTasks += shardData.NumberOfTasks
				indexData.TotalRequests += shardData.TotalRequests
				indexData.TotalTimeTaken_ms += shardData.TotalTimeTaken_ms
			}
		}
	}

	// Sort indices
	indices := make([]string, 0, len(clusterData.DataWriteBulk_sTasksByIndex))
	for index := range clusterData.DataWriteBulk_sTasksByIndex {
		indices = append(indices, index)
	}

	// Sort by tasks
	sortedIndices := make([]string, len(indices))
	copy(sortedIndices, indices)
	sort.Slice(sortedIndices, func(i, j int) bool {
		return clusterData.DataWriteBulk_sTasksByIndex[sortedIndices[i]].NumberOfTasks >
			clusterData.DataWriteBulk_sTasksByIndex[sortedIndices[j]].NumberOfTasks
	})
	clusterData.IndicesSortedonTasks = sortedIndices

	// Sort by requests
	sortedIndicesReq := make([]string, len(indices))
	copy(sortedIndicesReq, indices)
	sort.Slice(sortedIndicesReq, func(i, j int) bool {
		return clusterData.DataWriteBulk_sTasksByIndex[sortedIndicesReq[i]].TotalRequests >
			clusterData.DataWriteBulk_sTasksByIndex[sortedIndicesReq[j]].TotalRequests
	})
	clusterData.IndicesSortedOnRequests = sortedIndicesReq

	// Sort by time taken
	sortedIndicesTime := make([]string, len(indices))
	copy(sortedIndicesTime, indices)
	sort.Slice(sortedIndicesTime, func(i, j int) bool {
		return clusterData.DataWriteBulk_sTasksByIndex[sortedIndicesTime[i]].TotalTimeTaken_ms >
			clusterData.DataWriteBulk_sTasksByIndex[sortedIndicesTime[j]].TotalTimeTaken_ms
	})
	clusterData.IndicesSortedOnTimetaken = sortedIndicesTime
}

// extractIndexName extracts the index name from index_shard format (removes trailing _<digits>)
func extractIndexName(indexShard string) string {
	// Find last underscore followed by digits
	re := regexp.MustCompile(`_\d+$`)
	return re.ReplaceAllString(indexShard, "")
}

// updateClusterTasksHistory updates the global history for a cluster (thread-safe)
func updateClusterTasksHistory(clusterName string, clusterData *types.ClusterDataWriteBulk_sTasks, historySize uint) {
	types.ClusterDataWriteBulkTasksHistoryMu.Lock()
	defer types.ClusterDataWriteBulkTasksHistoryMu.Unlock()

	history, exists := types.AllClusterDataWriteBulk_sTasksHistory[clusterName]

	if !exists {
		// Create new history
		history = &types.ClusterDataWriteBulk_sTasksHistory{
			LatestSnapShotTime:             clusterData.SnapShotTime,
			HistorySize:                    historySize,
			ClusterName:                    clusterName,
			PtrClusterDataWriteBulk_sTasks: make([]*types.ClusterDataWriteBulk_sTasks, historySize+1),
		}
		types.AllClusterDataWriteBulk_sTasksHistory[clusterName] = history
	}

	// Roll data: shift everything down by one position
	// (Already protected by ClusterDataWriteBulkTasksHistoryMu)
	for i := int(history.HistorySize); i > 0; i-- {
		history.PtrClusterDataWriteBulk_sTasks[i] = history.PtrClusterDataWriteBulk_sTasks[i-1]
	}

	// Insert new data at position 0
	history.PtrClusterDataWriteBulk_sTasks[0] = clusterData
	history.LatestSnapShotTime = clusterData.SnapShotTime
}
