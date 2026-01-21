package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"ElasticObservability/pkg/logger"
	"ElasticObservability/pkg/scheduler"
	"ElasticObservability/pkg/types"
	"ElasticObservability/pkg/utils"

	"github.com/gorilla/mux"
)

// Server represents the API server
type Server struct {
	router    *mux.Router
	scheduler *scheduler.Scheduler
}

// NewServer creates a new API server
func NewServer(sched *scheduler.Scheduler) *Server {
	s := &Server{
		router:    mux.NewRouter(),
		scheduler: sched,
	}
	s.setupRoutes()
	return s
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Cluster endpoints
	s.router.HandleFunc("/api/clusters", s.handleGetClusters).Methods("GET")
	s.router.HandleFunc("/api/clusters/{clusterName}/nodes", s.handleGetNodes).Methods("GET")

	// Indexing rate endpoints
	s.router.HandleFunc("/api/indexingRate/{clusterName}", s.handleGetIndexingRate).Methods("GET")

	// Stale indices endpoint
	s.router.HandleFunc("/api/staleIndices/{clusterName}/{days}", s.handleGetStaleIndices).Methods("GET")

	// Thread Pool Write Queue endpoints
	s.router.HandleFunc("/api/tpwqueue/{clusterName}", s.handleGetTPWQueueCluster).Methods("GET")
	s.router.HandleFunc("/api/tpwqueue/{clusterName}/{hostName}", s.handleGetTPWQueueHost).Methods("GET")

	// Bulk Write Tasks endpoints
	s.router.HandleFunc("/api/bulkTasks/clusters", s.handleGetBulkTasksClusters).Methods("GET")
	s.router.HandleFunc("/api/bulkTasks/{clusterName}", s.handleGetBulkTasksHistory).Methods("GET")
	s.router.HandleFunc("/api/bulkTasks/{clusterName}/latest", s.handleGetBulkTasksLatest).Methods("GET")

	// Status endpoints
	s.router.HandleFunc("/api/status", s.handleGetStatus).Methods("GET")
	s.router.HandleFunc("/api/jobs", s.handleGetJobs).Methods("GET")

	// Job control
	s.router.HandleFunc("/api/jobs/{jobName}/trigger", s.handleTriggerJob).Methods("POST")
}

// ServeHTTP implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// handleGetClusters returns list of all clusters
func (s *Server) handleGetClusters(w http.ResponseWriter, r *http.Request) {
	types.ClustersMu.RLock()
	clusters := make([]string, len(types.AllClustersList))
	copy(clusters, types.AllClustersList)
	types.ClustersMu.RUnlock()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"clusters": clusters,
		"count":    len(clusters),
	})
}

// handleGetNodes returns nodes for a specific cluster
func (s *Server) handleGetNodes(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["clusterName"]

	// Validate cluster name
	if !utils.ValidateClusterName(clusterName) {
		respondError(w, http.StatusBadRequest, "Invalid cluster name")
		return
	}

	types.ClustersMu.RLock()
	cluster, exists := types.AllClusters[clusterName]
	types.ClustersMu.RUnlock()

	if !exists {
		respondError(w, http.StatusNotFound, "Cluster not found")
		return
	}

	// Build response with node information
	nodes := make([]map[string]interface{}, 0, len(cluster.Nodes))
	for _, node := range cluster.Nodes {
		nodes = append(nodes, map[string]interface{}{
			"hostName":   node.HostName,
			"ipAddress":  node.IPAddress,
			"port":       node.Port,
			"type":       node.Type,
			"zone":       node.Zone,
			"nodeTier":   node.NodeTier,
			"dataCenter": node.DataCenter,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"cluster": clusterName,
		"nodes":   nodes,
		"count":   len(nodes),
	})
}

// handleGetIndexingRate returns indexing rate for a cluster
func (s *Server) handleGetIndexingRate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["clusterName"]

	// Validate cluster name
	if !utils.ValidateClusterName(clusterName) {
		respondError(w, http.StatusBadRequest, "Invalid cluster name")
		return
	}

	// Check if cluster exists
	types.ClustersMu.RLock()
	_, exists := types.AllClusters[clusterName]
	types.ClustersMu.RUnlock()

	if !exists {
		respondError(w, http.StatusNotFound, "Cluster not found")
		return
	}

	// Get indexing rate
	types.IndexingRateMu.RLock()
	clusterRate, hasRate := types.AllIndexingRate[clusterName]
	types.IndexingRateMu.RUnlock()

	if !hasRate || clusterRate == nil {
		respondError(w, http.StatusNotFound, "Indexing rate data not available yet")
		return
	}

	// Build response
	indices := make(map[string]interface{})
	for indexBase, rate := range clusterRate.MapIndices {
		if rate != nil {
			indices[indexBase] = map[string]interface{}{
				"fromCreation":   rate.FromCreation,
				"last3Minutes":   rate.Last3Minutes,
				"last15Minutes":  rate.Last15Minutes,
				"last60Minutes":  rate.Last60Minutes,
				"numberOfShards": rate.NumberOfShards,
			}
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"cluster":   clusterName,
		"timestamp": clusterRate.Timestamp,
		"indices":   indices,
	})
}

// handleGetStatus returns application status
func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	types.ClustersMu.RLock()
	clusterCount := len(types.AllClusters)
	types.ClustersMu.RUnlock()

	types.IndexingRateMu.RLock()
	rateCount := len(types.AllIndexingRate)
	types.IndexingRateMu.RUnlock()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "running",
		"clusters":     clusterCount,
		"ratesTracked": rateCount,
		"timestamp":    utils.TimeNowMillis(),
	})
}

// handleGetJobs returns job status
func (s *Server) handleGetJobs(w http.ResponseWriter, r *http.Request) {
	jobStatus := s.scheduler.GetJobStatus()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"jobs": jobStatus,
	})
}

// handleGetStaleIndices returns indices that have not been modified in n days
func (s *Server) handleGetStaleIndices(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["clusterName"]
	daysStr := vars["days"]

	// Validate cluster name
	if !utils.ValidateClusterName(clusterName) {
		respondError(w, http.StatusBadRequest, "Invalid cluster name")
		return
	}

	// Parse days parameter
	var days int
	if _, err := fmt.Sscanf(daysStr, "%d", &days); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid days parameter, must be a positive integer")
		return
	}

	if days < 1 {
		respondError(w, http.StatusBadRequest, "Days parameter must be at least 1")
		return
	}

	// Check if cluster exists
	types.ClustersMu.RLock()
	_, exists := types.AllClusters[clusterName]
	types.ClustersMu.RUnlock()

	if !exists {
		respondError(w, http.StatusNotFound, "Cluster not found")
		return
	}

	// Get stats for the cluster and make a copy (thread-safe)
	types.StatsByDayMu.RLock()
	clusterStats, hasStats := types.AllStatsByDay[clusterName]

	if !hasStats || clusterStats == nil {
		types.StatsByDayMu.RUnlock()
		respondError(w, http.StatusNotFound, "Daily statistics not available for this cluster yet")
		return
	}

	// Check if we have enough history
	if clusterStats.StatHistory == nil || len(clusterStats.StatHistory) == 0 {
		types.StatsByDayMu.RUnlock()
		respondError(w, http.StatusNotFound, "No index statistics available")
		return
	}

	// Make a copy of data we need for processing (shallow copy of map entries)
	statHistoryCopy := make(map[string]*types.IndexStatHistory, len(clusterStats.StatHistory))
	for indexName, statHistory := range clusterStats.StatHistory {
		statHistoryCopy[indexName] = statHistory
	}
	lastUpdateTime := clusterStats.LastUpdateTime

	// Release lock immediately after copying
	types.StatsByDayMu.RUnlock()

	// Now work on the copy without holding the lock
	staleIndices := make([]map[string]interface{}, 0)
	totalIndices := 0
	insufficientData := 0

	for indexName, statHistory := range statHistoryCopy {
		totalIndices++

		// Validate statHistory is not nil
		if statHistory == nil {
			logger.AppWarn("StatHistory is nil for index %s in cluster %s", indexName, clusterName)
			insufficientData++
			continue
		}

		// Validate StatsPtr array is not nil and has enough size
		if statHistory.StatsPtr == nil || len(statHistory.StatsPtr) <= days {
			insufficientData++
			continue
		}

		// Get current stats (StatsPtr[0])
		currentStats := statHistory.StatsPtr[0]
		if currentStats == nil {
			insufficientData++
			continue
		}

		// Get stats from n days ago (StatsPtr[days])
		oldStats := statHistory.StatsPtr[days]
		if oldStats == nil {
			// Not enough historical data yet
			insufficientData++
			continue
		}

		// Compare DocCount - if same, index hasn't been modified
		if currentStats.DocCount == oldStats.DocCount {
			staleIndices = append(staleIndices, map[string]interface{}{
				"indexName":        indexName,
				"docCount":         currentStats.DocCount,
				"currentSize":      currentStats.TotalSize,
				"currentTimestamp": currentStats.StatTime,
				"oldSize":          oldStats.TotalSize,
				"oldTimestamp":     oldStats.StatTime,
				"daysStale":        days,
				"sizeChange":       int64(currentStats.TotalSize) - int64(oldStats.TotalSize),
			})
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"cluster":               clusterName,
		"daysChecked":           days,
		"totalIndices":          totalIndices,
		"staleIndices":          staleIndices,
		"staleCount":            len(staleIndices),
		"insufficientDataCount": insufficientData,
		"lastUpdateTime":        lastUpdateTime,
	})
}

// handleGetTPWQueueCluster returns thread pool write queue data for all hosts in a cluster
func (s *Server) handleGetTPWQueueCluster(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["clusterName"]

	// Validate cluster name
	if !utils.ValidateClusterName(clusterName) {
		respondError(w, http.StatusBadRequest, "Invalid cluster name")
		return
	}

	// Check if cluster exists
	types.ClustersMu.RLock()
	_, exists := types.AllClusters[clusterName]
	types.ClustersMu.RUnlock()

	if !exists {
		respondError(w, http.StatusNotFound, "Cluster not found")
		return
	}

	// Get TPWQueue data for cluster (thread-safe)
	types.TPWQueueMu.RLock()
	clusterData, hasData := types.AllThreadPoolWriteQueues[clusterName]

	if !hasData || clusterData == nil {
		types.TPWQueueMu.RUnlock()
		respondError(w, http.StatusNotFound, "Thread pool write queue data not available for this cluster yet")
		return
	}

	// Make a copy of the data
	hostnames := make([]string, len(clusterData.HostnameList))
	copy(hostnames, clusterData.HostnameList)

	hostsData := make(map[string]map[string]interface{})
	for hostName, tpwq := range clusterData.HostTPWQueue {
		if tpwq == nil {
			continue
		}

		// Build data point arrays with only existing data
		dataPoints := make([]map[string]interface{}, 0, tpwq.NumberOfDataPoints)
		for i := 0; i < tpwq.NumberOfDataPoints; i++ {
			if tpwq.DataExists[i] {
				dataPoints = append(dataPoints, map[string]interface{}{
					"timestamp": tpwq.TimeStamps[i],
					"queue":     tpwq.ThreadPoolWriteQueues[i],
					"index":     i,
				})
			}
		}

		hostsData[hostName] = map[string]interface{}{
			"numberOfDataPoints": tpwq.NumberOfDataPoints,
			"dataPoints":         dataPoints,
			"dataPointCount":     len(dataPoints),
		}
	}
	types.TPWQueueMu.RUnlock()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"cluster":   clusterName,
		"hostnames": hostnames,
		"hostCount": len(hostnames),
		"hosts":     hostsData,
	})
}

// handleGetTPWQueueHost returns thread pool write queue data for a specific host
func (s *Server) handleGetTPWQueueHost(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["clusterName"]
	hostName := vars["hostName"]

	// Validate cluster name
	if !utils.ValidateClusterName(clusterName) {
		respondError(w, http.StatusBadRequest, "Invalid cluster name")
		return
	}

	if hostName == "" {
		respondError(w, http.StatusBadRequest, "Host name is required")
		return
	}

	// Check if cluster exists
	types.ClustersMu.RLock()
	_, exists := types.AllClusters[clusterName]
	types.ClustersMu.RUnlock()

	if !exists {
		respondError(w, http.StatusNotFound, "Cluster not found")
		return
	}

	// Get TPWQueue data for host (thread-safe)
	types.TPWQueueMu.RLock()
	clusterData, hasData := types.AllThreadPoolWriteQueues[clusterName]

	if !hasData || clusterData == nil {
		types.TPWQueueMu.RUnlock()
		respondError(w, http.StatusNotFound, "Thread pool write queue data not available for this cluster yet")
		return
	}

	tpwq, hostExists := clusterData.HostTPWQueue[hostName]
	if !hostExists || tpwq == nil {
		types.TPWQueueMu.RUnlock()
		respondError(w, http.StatusNotFound, fmt.Sprintf("Host %s not found in cluster %s", hostName, clusterName))
		return
	}

	// Build response with all data points
	dataPoints := make([]map[string]interface{}, 0, tpwq.NumberOfDataPoints)
	existingCount := 0
	missingCount := 0

	for i := 0; i < tpwq.NumberOfDataPoints; i++ {
		point := map[string]interface{}{
			"index":      i,
			"dataExists": tpwq.DataExists[i],
		}

		if tpwq.DataExists[i] {
			point["timestamp"] = tpwq.TimeStamps[i]
			point["queue"] = tpwq.ThreadPoolWriteQueues[i]
			existingCount++
		} else {
			point["timestamp"] = nil
			point["queue"] = nil
			missingCount++
		}

		dataPoints = append(dataPoints, point)
	}
	types.TPWQueueMu.RUnlock()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"cluster":            clusterName,
		"hostName":           hostName,
		"numberOfDataPoints": tpwq.NumberOfDataPoints,
		"existingCount":      existingCount,
		"missingCount":       missingCount,
		"dataPoints":         dataPoints,
	})
}

// handleTriggerJob manually triggers a job
func (s *Server) handleTriggerJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobName := vars["jobName"]

	err := s.scheduler.TriggerJob(jobName)
	if err != nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("Failed to trigger job: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("Job %s triggered successfully", jobName),
	})
}

// handleGetBulkTasksClusters returns list of clusters with bulk tasks history
func (s *Server) handleGetBulkTasksClusters(w http.ResponseWriter, r *http.Request) {
	types.ClusterDataWriteBulkTasksHistoryMu.RLock()
	clusters := make([]map[string]interface{}, 0, len(types.AllClusterDataWriteBulk_sTasksHistory))

	for clusterName, history := range types.AllClusterDataWriteBulk_sTasksHistory {
		if history != nil {
			clusters = append(clusters, map[string]interface{}{
				"clusterName":        clusterName,
				"historySize":        history.HistorySize,
				"latestSnapshotTime": history.LatestSnapShotTime,
			})
		}
	}
	types.ClusterDataWriteBulkTasksHistoryMu.RUnlock()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"clusters": clusters,
		"count":    len(clusters),
	})
}

// handleGetBulkTasksHistory returns complete bulk tasks history for a cluster
func (s *Server) handleGetBulkTasksHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["clusterName"]

	// Validate cluster name
	if !utils.ValidateClusterName(clusterName) {
		respondError(w, http.StatusBadRequest, "Invalid cluster name")
		return
	}

	// Get history data (thread-safe)
	types.ClusterDataWriteBulkTasksHistoryMu.RLock()
	history, exists := types.AllClusterDataWriteBulk_sTasksHistory[clusterName]

	if !exists || history == nil {
		types.ClusterDataWriteBulkTasksHistoryMu.RUnlock()
		respondError(w, http.StatusNotFound, "Bulk tasks history not available for this cluster yet")
		return
	}

	// Build response with all snapshots
	snapshots := make([]interface{}, 0, history.HistorySize)
	for i := uint(0); i < history.HistorySize; i++ {
		snapshot := history.PtrClusterDataWriteBulk_sTasks[i]
		if snapshot != nil {
			snapshots = append(snapshots, snapshot)
		}
	}

	response := map[string]interface{}{
		"clusterName":        history.ClusterName,
		"historySize":        history.HistorySize,
		"latestSnapshotTime": history.LatestSnapShotTime,
		"snapshots":          snapshots,
		"snapshotCount":      len(snapshots),
	}
	types.ClusterDataWriteBulkTasksHistoryMu.RUnlock()

	respondJSON(w, http.StatusOK, response)
}

// handleGetBulkTasksLatest returns latest bulk tasks snapshot for a cluster
func (s *Server) handleGetBulkTasksLatest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["clusterName"]

	// Validate cluster name
	if !utils.ValidateClusterName(clusterName) {
		respondError(w, http.StatusBadRequest, "Invalid cluster name")
		return
	}

	// Get history data (thread-safe)
	types.ClusterDataWriteBulkTasksHistoryMu.RLock()
	history, exists := types.AllClusterDataWriteBulk_sTasksHistory[clusterName]

	if !exists || history == nil {
		types.ClusterDataWriteBulkTasksHistoryMu.RUnlock()
		respondError(w, http.StatusNotFound, "Bulk tasks history not available for this cluster yet")
		return
	}

	// Get latest snapshot (at index 0)
	latestSnapshot := history.PtrClusterDataWriteBulk_sTasks[0]
	if latestSnapshot == nil {
		types.ClusterDataWriteBulkTasksHistoryMu.RUnlock()
		respondError(w, http.StatusNotFound, "No bulk tasks data available yet")
		return
	}

	response := map[string]interface{}{
		"clusterName":        clusterName,
		"snapshot":           latestSnapshot,
		"latestSnapshotTime": history.LatestSnapShotTime,
	}
	types.ClusterDataWriteBulkTasksHistoryMu.RUnlock()

	respondJSON(w, http.StatusOK, response)
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logger.AppError("Failed to encode JSON response: %v", err)
	}
}

// respondError sends an error response
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]interface{}{
		"error": message,
	})
}
