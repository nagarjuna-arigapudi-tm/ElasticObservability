package jobs

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"ElasticObservability/pkg/logger"
	"ElasticObservability/pkg/types"
	"ElasticObservability/pkg/utils"
)

// UpdateActiveEndpoint validates connectivity to clusters and updates active endpoints
func UpdateActiveEndpoint(ctx context.Context, params map[string]interface{}) error {
	logger.JobInfo("updateActiveEndpoint", "Starting endpoint validation job")

	// Get exclude list
	excludeClusters := make([]string, 0)
	if exclude, ok := params["excludeClusters"].([]interface{}); ok {
		for _, item := range exclude {
			if str, ok := item.(string); ok {
				excludeClusters = append(excludeClusters, str)
			}
		}
	}

	types.ClustersMu.RLock()
	clustersCopy := make(map[string]*types.ClusterData)
	for name, cluster := range types.AllClusters {
		clustersCopy[name] = cluster
	}
	types.ClustersMu.RUnlock()

	updatedCount := 0
	failedCount := 0

	for clusterName, cluster := range clustersCopy {
		// Skip excluded clusters
		if utils.Contains(excludeClusters, clusterName) {
			logger.JobInfo("updateActiveEndpoint", "Skipping excluded cluster: %s", clusterName)
			continue
		}

		endpoint := findActiveEndpoint(cluster)
		if endpoint != "" {
			cluster.ActiveEndPoint = endpoint
			updatedCount++
			logger.JobInfo("updateActiveEndpoint", "Cluster %s: Active endpoint set to %s", clusterName, endpoint)
		} else {
			cluster.ActiveEndPoint = ""
			failedCount++
			logger.JobWarn("updateActiveEndpoint", "Cluster %s: Failed to find active endpoint", clusterName)
		}
	}

	logger.JobInfo("updateActiveEndpoint", "Completed: %d endpoints updated, %d failed", updatedCount, failedCount)
	return nil
}

func findActiveEndpoint(cluster *types.ClusterData) string {
	// Try ClusterSAN endpoints first
	for _, endpoint := range cluster.ClusterSAN {
		if endpoint == "" {
			continue
		}
		if testConnection(endpoint, cluster) {
			return endpoint
		}
	}

	// Try master nodes
	for _, node := range cluster.Nodes {
		if utils.Contains(node.Type, "master") {
			endpoint := fmt.Sprintf("https://%s:%s", node.HostName, node.Port)
			if testConnection(endpoint, cluster) {
				return endpoint
			}
		}
	}

	// Try kibana nodes
	for _, node := range cluster.Nodes {
		if utils.Contains(node.Type, "kibana") {
			endpoint := fmt.Sprintf("https://%s:%s", node.HostName, node.KibanaPort)
			if testConnection(endpoint, cluster) {
				return endpoint
			}
		}
	}

	// Try all remaining nodes
	for _, node := range cluster.Nodes {
		endpoint := fmt.Sprintf("https://%s:%s", node.HostName, node.Port)
		if testConnection(endpoint, cluster) {
			return endpoint
		}
	}

	return ""
}

func testConnection(endpoint string, cluster *types.ClusterData) bool {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cluster.InsecureTLS,
			},
		},
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return false
	}

	// Try authentication methods based on preference
	cred := &cluster.AccessCred
	authenticated := false

	// Try preferred method first
	switch cred.Preferred {
	case 1: // API Key
		if cred.APIKey != "" {
			req.Header.Set("Authorization", "ApiKey "+cred.APIKey)
			authenticated = true
		}
	case 2: // Username/Password
		if cred.UserID != "" && cred.Password != "" {
			req.SetBasicAuth(cred.UserID, cred.Password)
			authenticated = true
		}
	case 3: // Certificate - would need more complex setup
		// Skip for now
	}

	// If preferred method not available, try others
	if !authenticated {
		if cred.APIKey != "" {
			req.Header.Set("Authorization", "ApiKey "+cred.APIKey)
		} else if cred.UserID != "" && cred.Password != "" {
			req.SetBasicAuth(cred.UserID, cred.Password)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Accept both successful connections and auth failures (endpoint is reachable)
	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized
}
