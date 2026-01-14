package jobs

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"ElasticObservability/pkg/config"
	"ElasticObservability/pkg/logger"
	"ElasticObservability/pkg/types"
	"ElasticObservability/pkg/utils"
)

const (
	defaultQuery = `{
	"aggs": {
		"hostname": {
			"terms": {
				"field": "source_node.host",
				"order": {
					"2[node_stats.thread_pool.write.queue]": "desc"
				},
				"size": 250
			},
			"aggs": {
				"2": {
					"top_metrics": {
						"metrics": {
							"field": "node_stats.thread_pool.write.queue"
						},
						"size": 1,
						"sort": {
							"timestamp": "desc"
						}
					}
				},
				"date_bucket": {
					"date_histogram": {
						"field": "source_node.timestamp",
						"fixed_interval": "__INTERVAL__",
						"time_zone": "US/Eastern"
					},
					"aggs": {
						"2": {
							"top_metrics": {
								"metrics": {
									"field": "node_stats.thread_pool.write.queue"
								},
								"size": 1,
								"sort": {
									"timestamp": "desc"
								}
							}
						}
					}
				}
			}
		}
	},
	"size": 0,
	"fields": [
		{
			"field": "enrich_executing_policy_stats.task.start_time_in_millis",
			"format": "date_time"
		},
		{
			"field": "job_stats.data_counts.earliest_record_timestamp",
			"format": "date_time"
		},
		{
			"field": "source_node.timestamp",
			"format": "date_time"
		},
		{
			"field": "timestamp",
			"format": "date_time"
		}
	],
	"script_fields": {},
	"stored_fields": ["*"],
	"runtime_mappings": {},
	"_source": {
		"excludes": []
	},
	"query": {
		"bool": {
			"must": [],
			"filter": [
				{
					"match_phrase": {
						"cluster_uuid": "__UUID__"
					}
				},
				{
					"match_phrase": {
						"type": "node_stats"
					}
				},
				{
					"range": {
						"source_node.timestamp": {
							"format": "strict_date_optional_time",
							"gte": "now-__TIME_SPAN__",
							"lte": "now"
						}
					}
				}
			],
			"should": [],
			"must_not": [
				{
					"match_phrase": {
						"node_stats.indices.docs.count": 0
					}
				}
			]
		}
	}
}`
	defaultHostNamePath        = "aggregations.hostname.buckets.key"
	defaultMetricsPath         = "aggregations.hostname.buckets.date_bucket.buckets.2.top_metrics.metrics.node_stats.thread_pool.write.queue"
	defaultMetricTimestampPath = "aggregations.hostname.buckets.date_bucket.buckets.key"
)

type clusterJobResult struct {
	ClusterName string
	Data        map[string]*types.TPWQueue
	Hostnames   []string
	Error       error
}

// GetThreadPoolWriteQueue collects thread pool write queue metrics from monitoring cluster
func GetThreadPoolWriteQueue(ctx context.Context, params map[string]interface{}) error {
	logger.JobInfo("getThreadPoolWriteQueue", "Starting thread pool write queue monitoring job")

	// Get parameters
	excludeClusters := getStringSliceParam(params, "excludeClusters")
	spanInterval := getStringParam(params, "spanInterval", "30s")
	timeSpan := getStringParam(params, "timeSpan", "10m")
	parallelRoutines := getIntParam(params, "parallelRoutines", 5)
	insecureTLS := getBoolParam(params, "insecureTLS", false)
	apiKey := getStringParam(params, "APIKEY", "")
	apiEndpoints := getStringSliceParam(params, "APIEndPoints")
	queryTemplate := getStringParam(params, "query", defaultQuery)

	// Get JSON paths
	resultsJsonPaths := getMapParam(params, "resultsJsonPaths")
	hostNamePath := getStringFromMap(resultsJsonPaths, "hostName", defaultHostNamePath)
	metricsPath := getStringFromMap(resultsJsonPaths, "metrics", defaultMetricsPath)
	metricTimestampPath := getStringFromMap(resultsJsonPaths, "metricTimestamp", defaultMetricTimestampPath)

	if len(apiEndpoints) == 0 {
		return fmt.Errorf("APIEndPoints parameter is required")
	}
	if apiKey == "" {
		return fmt.Errorf("APIKEY parameter is required")
	}

	// Calculate data points
	dataSets := config.Global.ThreadPoolWriteQueueDataSets
	dataPointsInDataSet := parseTimeToDataPoints(timeSpan, spanInterval)
	numberOfDataPoints := int(dataSets) * dataPointsInDataSet
	intervalMs := parseDurationToMillis(spanInterval)

	logger.JobInfo("getThreadPoolWriteQueue", "Config: dataSets=%d, pointsPerSet=%d, total=%d, intervalMs=%d",
		dataSets, dataPointsInDataSet, numberOfDataPoints, intervalMs)

	// Build cluster list and UUID map
	types.ClustersMu.RLock()
	clusterListForTPWQueue := make([]string, 0)
	mapClusterUUID := make(map[string]string)

	for _, clusterName := range types.AllClustersList {
		if utils.Contains(excludeClusters, clusterName) {
			continue
		}

		cluster, exists := types.AllClusters[clusterName]
		if !exists || cluster.ClusterUUID == "" {
			logger.JobWarn("getThreadPoolWriteQueue", "Cluster %s has no UUID, skipping", clusterName)
			continue
		}

		clusterListForTPWQueue = append(clusterListForTPWQueue, clusterName)
		mapClusterUUID[clusterName] = cluster.ClusterUUID
	}
	types.ClustersMu.RUnlock()

	logger.JobInfo("getThreadPoolWriteQueue", "Processing %d clusters", len(clusterListForTPWQueue))

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureTLS},
		},
	}

	// Process clusters in parallel
	resultsChan := make(chan clusterJobResult, len(clusterListForTPWQueue))
	semaphore := make(chan struct{}, parallelRoutines)
	var wg sync.WaitGroup

	for _, clusterName := range clusterListForTPWQueue {
		wg.Add(1)
		go func(cName string) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			result := processCluster(ctx, cName, mapClusterUUID[cName], apiEndpoints, apiKey,
				queryTemplate, spanInterval, timeSpan, httpClient,
				hostNamePath, metricsPath, metricTimestampPath,
				numberOfDataPoints, intervalMs, dataPointsInDataSet)
			resultsChan <- result
		}(clusterName)
	}

	// Wait for all goroutines and close channel
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	successCount := 0
	failCount := 0
	for result := range resultsChan {
		if result.Error != nil {
			logger.JobError("getThreadPoolWriteQueue", "Cluster %s failed: %v", result.ClusterName, result.Error)
			failCount++
			continue
		}

		// Update global structure (thread-safe)
		updateGlobalTPWQueue(result.ClusterName, result.Data, result.Hostnames, numberOfDataPoints)
		successCount++
		logger.JobInfo("getThreadPoolWriteQueue", "Cluster %s processed successfully with %d hosts",
			result.ClusterName, len(result.Hostnames))
	}

	logger.JobInfo("getThreadPoolWriteQueue", "Completed: %d succeeded, %d failed", successCount, failCount)
	return nil
}

func processCluster(ctx context.Context, clusterName, clusterUUID string, apiEndpoints []string,
	apiKey, queryTemplate, spanInterval, timeSpan string, httpClient *http.Client,
	hostNamePath, metricsPath, metricTimestampPath string,
	numberOfDataPoints int, intervalMs int64, dataPointsInDataSet int) clusterJobResult {

	// Substitute macros in query
	query := strings.ReplaceAll(queryTemplate, "__UUID__", clusterUUID)
	query = strings.ReplaceAll(query, "__INTERVAL__", spanInterval)
	query = strings.ReplaceAll(query, "__TIME_SPAN__", timeSpan)

	// Try each endpoint
	var responseData map[string]interface{}
	var lastErr error

	for _, endpoint := range apiEndpoints {
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBufferString(query))
		if err != nil {
			lastErr = err
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "ApiKey "+apiKey)

		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
			continue
		}

		if err := json.Unmarshal(body, &responseData); err != nil {
			lastErr = err
			continue
		}

		// Success
		lastErr = nil
		break
	}

	if lastErr != nil {
		return clusterJobResult{ClusterName: clusterName, Error: lastErr}
	}

	// Parse response
	hostData, hostnames, err := parseTPWQueueResponse(responseData, hostNamePath, metricsPath,
		metricTimestampPath, numberOfDataPoints, intervalMs, dataPointsInDataSet)
	if err != nil {
		return clusterJobResult{ClusterName: clusterName, Error: err}
	}

	return clusterJobResult{
		ClusterName: clusterName,
		Data:        hostData,
		Hostnames:   hostnames,
		Error:       nil,
	}
}

func parseTPWQueueResponse(data map[string]interface{}, hostNamePath, metricsPath,
	metricTimestampPath string, numberOfDataPoints int, intervalMs int64,
	dataPointsInDataSet int) (map[string]*types.TPWQueue, []string, error) {

	// Navigate to hostname buckets
	aggregations, ok := data["aggregations"].(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("aggregations not found")
	}

	hostname, ok := aggregations["hostname"].(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("hostname aggregation not found")
	}

	buckets, ok := hostname["buckets"].([]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("buckets not found")
	}

	hostData := make(map[string]*types.TPWQueue)
	hostnames := make([]string, 0, len(buckets))

	for _, bucket := range buckets {
		bucketMap, ok := bucket.(map[string]interface{})
		if !ok {
			continue
		}

		// Get hostname
		hostName, ok := bucketMap["key"].(string)
		if !ok {
			continue
		}

		// Get date_bucket
		dateBucket, ok := bucketMap["date_bucket"].(map[string]interface{})
		if !ok {
			continue
		}

		dateBuckets, ok := dateBucket["buckets"].([]interface{})
		if !ok {
			continue
		}

		// Initialize TPWQueue for this host
		tpwq := &types.TPWQueue{
			NumberOfDataPoints:    numberOfDataPoints,
			TimeStamps:            make([]int64, numberOfDataPoints),
			ThreadPoolWriteQueues: make([]uint32, numberOfDataPoints),
			DataExists:            make([]bool, numberOfDataPoints),
		}

		// Extract metrics and timestamps
		dataPoints := make([]struct {
			timestamp int64
			metric    uint32
		}, 0, len(dateBuckets))

		for _, db := range dateBuckets {
			dbMap, ok := db.(map[string]interface{})
			if !ok {
				continue
			}

			// Get timestamp
			tsVal, ok := dbMap["key"].(float64)
			if !ok {
				continue
			}
			timestamp := int64(tsVal)

			// Get metric
			topMetrics, ok := dbMap["2"].(map[string]interface{})
			if !ok {
				continue
			}

			topArray, ok := topMetrics["top"].([]interface{})
			if !ok || len(topArray) == 0 {
				continue
			}

			topItem, ok := topArray[0].(map[string]interface{})
			if !ok {
				continue
			}

			metrics, ok := topItem["metrics"].(map[string]interface{})
			if !ok {
				continue
			}

			metricVal, ok := metrics["node_stats.thread_pool.write.queue"].(float64)
			if !ok {
				continue
			}

			dataPoints = append(dataPoints, struct {
				timestamp int64
				metric    uint32
			}{timestamp, uint32(metricVal)})
		}

		// Sort by timestamp descending (latest first)
		sort.Slice(dataPoints, func(i, j int) bool {
			return dataPoints[i].timestamp > dataPoints[j].timestamp
		})

		// Fill in the data (latest at index 0)
		if len(dataPoints) > 0 {
			latestTime := dataPoints[0].timestamp

			for i, dp := range dataPoints {
				if i >= dataPointsInDataSet {
					break
				}

				expectedIndex := int((latestTime - dp.timestamp) / intervalMs)
				if expectedIndex < 0 || expectedIndex >= dataPointsInDataSet {
					continue
				}

				tpwq.TimeStamps[expectedIndex] = dp.timestamp
				tpwq.ThreadPoolWriteQueues[expectedIndex] = dp.metric
				tpwq.DataExists[expectedIndex] = true
			}
		}

		hostData[hostName] = tpwq
		hostnames = append(hostnames, hostName)
	}

	return hostData, hostnames, nil
}

func updateGlobalTPWQueue(clusterName string, newData map[string]*types.TPWQueue,
	hostnames []string, numberOfDataPoints int) {

	types.TPWQueueMu.Lock()
	defer types.TPWQueueMu.Unlock()

	existing, exists := types.AllThreadPoolWriteQueues[clusterName]

	if !exists {
		// First time - just store the data
		types.AllThreadPoolWriteQueues[clusterName] = &types.ClustersTPWQueue{
			HostnameList: hostnames,
			HostTPWQueue: newData,
		}
		return
	}

	// Roll existing data and merge with new data
	dataPointsInDataSet := numberOfDataPoints / int(config.Global.ThreadPoolWriteQueueDataSets)

	for hostName, newTPWQ := range newData {
		existingTPWQ, hostExists := existing.HostTPWQueue[hostName]

		if !hostExists {
			// New host - just add it
			existing.HostTPWQueue[hostName] = newTPWQ
			existing.HostnameList = append(existing.HostnameList, hostName)
			continue
		}

		// Roll the data: move existing data down by dataPointsInDataSet positions
		rollTPWQueueData(existingTPWQ, newTPWQ, dataPointsInDataSet)
	}

	// Remove hosts that are no longer present
	newHostSet := make(map[string]bool)
	for _, h := range hostnames {
		newHostSet[h] = true
	}

	updatedHostList := make([]string, 0, len(hostnames))
	for _, h := range existing.HostnameList {
		if newHostSet[h] {
			updatedHostList = append(updatedHostList, h)
		} else {
			delete(existing.HostTPWQueue, h)
		}
	}
	existing.HostnameList = updatedHostList
}

func rollTPWQueueData(existing, new *types.TPWQueue, dataPointsInDataSet int) {
	totalSize := existing.NumberOfDataPoints

	// Roll down: move data from position i to position i+dataPointsInDataSet
	for i := totalSize - 1; i >= dataPointsInDataSet; i-- {
		sourceIdx := i - dataPointsInDataSet
		existing.TimeStamps[i] = existing.TimeStamps[sourceIdx]
		existing.ThreadPoolWriteQueues[i] = existing.ThreadPoolWriteQueues[sourceIdx]
		existing.DataExists[i] = existing.DataExists[sourceIdx]
	}

	// Copy new data into positions 0 to dataPointsInDataSet-1
	for i := 0; i < dataPointsInDataSet && i < len(new.TimeStamps); i++ {
		existing.TimeStamps[i] = new.TimeStamps[i]
		existing.ThreadPoolWriteQueues[i] = new.ThreadPoolWriteQueues[i]
		existing.DataExists[i] = new.DataExists[i]
	}
}

// Helper functions
func getStringSliceParam(params map[string]interface{}, key string) []string {
	if val, ok := params[key].([]interface{}); ok {
		result := make([]string, 0, len(val))
		for _, item := range val {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return []string{}
}

func getStringParam(params map[string]interface{}, key, defaultVal string) string {
	if val, ok := params[key].(string); ok && val != "" {
		return val
	}
	return defaultVal
}

func getIntParam(params map[string]interface{}, key string, defaultVal int) int {
	if val, ok := params[key].(int); ok {
		return val
	}
	if val, ok := params[key].(float64); ok {
		return int(val)
	}
	return defaultVal
}

func getBoolParam(params map[string]interface{}, key string, defaultVal bool) bool {
	if val, ok := params[key].(bool); ok {
		return val
	}
	return defaultVal
}

func getMapParam(params map[string]interface{}, key string) map[string]interface{} {
	if val, ok := params[key].(map[string]interface{}); ok {
		return val
	}
	return make(map[string]interface{})
}

func getStringFromMap(m map[string]interface{}, key, defaultVal string) string {
	if val, ok := m[key].(string); ok && val != "" {
		return val
	}
	return defaultVal
}

func parseDurationToMillis(duration string) int64 {
	duration = strings.ToLower(strings.TrimSpace(duration))

	var value int64
	var unit string

	fmt.Sscanf(duration, "%d%s", &value, &unit)

	switch unit {
	case "s", "sec", "second", "seconds":
		return value * 1000
	case "m", "min", "minute", "minutes":
		return value * 60 * 1000
	case "h", "hour", "hours":
		return value * 60 * 60 * 1000
	default:
		return value * 1000 // Default to seconds
	}
}

func parseTimeToDataPoints(timeSpan, interval string) int {
	spanMs := parseDurationToMillis(timeSpan)
	intervalMs := parseDurationToMillis(interval)

	if intervalMs == 0 {
		return 20 // Default
	}

	return int(spanMs / intervalMs)
}
