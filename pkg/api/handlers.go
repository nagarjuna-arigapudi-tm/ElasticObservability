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
