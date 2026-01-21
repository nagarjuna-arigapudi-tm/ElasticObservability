package utils

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ElasticObservability/pkg/types"
)

// ToLower converts a string to lowercase
func ToLower(s string) string {
	return strings.ToLower(s)
}

// ToUpper converts a string to uppercase
func ToUpper(s string) string {
	return strings.ToUpper(s)
}

// RemoveNonAlphaNumeric replaces non-alphanumeric characters with '_'
func RemoveNonAlphaNumeric(s string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	return reg.ReplaceAllString(s, "_")
}

// BoolStringCompare performs case-insensitive comparison with array of strings
func BoolStringCompare(value string, compareList []string) bool {
	lowerValue := strings.ToLower(strings.TrimSpace(value))
	for _, comp := range compareList {
		if lowerValue == strings.ToLower(strings.TrimSpace(comp)) {
			return true
		}
	}
	return false
}

// StrStringCompare compares a field against nested string arrays and returns corresponding retVal
func StrStringCompare(value string, compareList [][]string, retVals []string) string {
	lowerValue := strings.ToLower(strings.TrimSpace(value))

	for i, strList := range compareList {
		for _, comp := range strList {
			if lowerValue == strings.ToLower(strings.TrimSpace(comp)) {
				if i < len(retVals) {
					return retVals[i]
				}
				return value
			}
		}
	}
	return value
}

// SplitString splits a string by delimiter
func SplitString(s string, delimiter string) []string {
	if delimiter == "" {
		delimiter = ","
	}
	parts := strings.Split(s, delimiter)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// ParseIndexName extracts index_base and seq_no from index name
// Examples:
// .kibana_task_manager_7.17.2_001 => seq_no = 1, index_base = '.kibana_task_manager_7.17.2'
// .transform-internal-007 => seq_no = 7, index_base = '.transform-internal'
// .ds-citi scorecard billing_test2-2025.09.17-000012 => seq_no = 12, index_base = '.ds-citi scorecard billing_test2'
// 169736-elk-transforms => seq_no = 0, index_base = '169736-elk-transforms'
func ParseIndexName(indexName string) (indexBase string, seqNo uint64) {
	// Regular expression to match trailing digits
	digitPattern := regexp.MustCompile(`(\d+)$`)

	// Check if index name ends with digits
	matches := digitPattern.FindStringSubmatch(indexName)
	if len(matches) > 0 {
		// Extract seq_no
		seqNo, _ = strconv.ParseUint(matches[1], 10, 64)
		// Remove the digits from the end
		indexName = strings.TrimSuffix(indexName, matches[1])
	} else {
		seqNo = 0
	}

	// Remove trailing non-alphanumeric character (like - or _)
	indexName = strings.TrimRight(indexName, "-_")

	// Check for timestamp pattern at the end (YYYY.MM.DD)
	timestampPattern := regexp.MustCompile(`-?\d{4}\.\d{2}\.\d{2}$`)
	if timestampPattern.MatchString(indexName) {
		// Remove timestamp
		indexName = timestampPattern.ReplaceAllString(indexName, "")
		// Remove trailing non-alphanumeric character again
		indexName = strings.TrimRight(indexName, "-_")
	}

	indexBase = indexName
	return
}

// ParseStorageSize converts storage size string to bytes
// Supports formats like: 1kb, 1mb, 1gb, 1tb, 1b
func ParseStorageSize(sizeStr string) (uint64, error) {
	sizeStr = strings.ToLower(strings.TrimSpace(sizeStr))
	if sizeStr == "" {
		return 0, nil
	}

	// Extract number and unit
	re := regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([kmgt]?b?)$`)
	matches := re.FindStringSubmatch(sizeStr)

	if len(matches) < 2 {
		// Try to parse as plain number (bytes)
		val, err := strconv.ParseFloat(sizeStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid storage size format: %s", sizeStr)
		}
		return uint64(val), nil
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid storage size value: %s", matches[1])
	}

	unit := matches[2]
	var multiplier float64 = 1

	switch unit {
	case "b", "":
		multiplier = 1
	case "kb":
		multiplier = 1024
	case "mb":
		multiplier = 1024 * 1024
	case "gb":
		multiplier = 1024 * 1024 * 1024
	case "tb":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown storage unit: %s", unit)
	}

	return uint64(value * multiplier), nil
}

// ParseHealth converts health string to uint8
func ParseHealth(health string) uint8 {
	switch strings.ToLower(health) {
	case "green":
		return 1
	case "yellow":
		return 2
	case "red":
		return 3
	default:
		return 0
	}
}

// ParseStatus returns true if status is "open"
func ParseStatus(status string) bool {
	return strings.ToLower(status) == "open"
}

// Contains checks if a string slice contains a value
func Contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// GetNodeTypes parses node type strings and returns type array
func GetNodeTypes(typeStr string) []string {
	types := make([]string, 0)
	typeStr = strings.ToLower(typeStr)

	if strings.Contains(typeStr, "master") {
		types = append(types, "master")
	}
	if strings.Contains(typeStr, "data") {
		types = append(types, "data")
	}
	if strings.Contains(typeStr, "logstash") {
		types = append(types, "logstash")
	}
	if strings.Contains(typeStr, "kibana") {
		types = append(types, "kibana")
	}
	if strings.Contains(typeStr, "ml") || strings.Contains(typeStr, "machine") {
		types = append(types, "ml")
	}

	return types
}

// ValidateClusterName checks if a cluster name is valid and not too long
func ValidateClusterName(name string) bool {
	return len(name) > 0 && len(name) <= 255
}

// TimeNowMillis returns current time in epoch milliseconds
func TimeNowMillis() int64 {
	return timeNowMillis()
}

// timeNowMillis is the actual implementation (can be mocked for testing)
var timeNowMillis = func() int64 {
	return getCurrentTimeMillis()
}

// getCurrentTimeMillis returns current epoch time in milliseconds
func getCurrentTimeMillis() int64 {
	return timeNow().UnixNano() / int64(1000000)
}

// timeNow returns current time (can be mocked for testing)
var timeNow = func() timeInterface {
	return realTime{}
}

type timeInterface interface {
	UnixNano() int64
}

type realTime struct{}

func (realTime) UnixNano() int64 {
	return time.Now().UnixNano()
}

// GetCurrentMasterForCluster retrieves the current master node name for a given cluster
// Returns the hostname of the master node, or empty string if unable to determine
func GetCurrentMasterForCluster(clusterName string) string {
	// Get cluster data
	types.ClustersMu.RLock()
	cluster, exists := types.AllClusters[clusterName]
	types.ClustersMu.RUnlock()

	if !exists {
		return ""
	}

	// Construct endpoint
	activeEndpoint := cluster.ActiveEndpoint
	if activeEndpoint == "" {
		return ""
	}

	// Ensure endpoint ends with /
	if !strings.HasSuffix(activeEndpoint, "/") {
		activeEndpoint += "/"
	}

	endpoint := activeEndpoint + "_cat/nodes?h=n,m"

	// Create HTTP client
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cluster.InsecureTLS,
			},
		},
	}

	// Create request
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return ""
	}

	// Add authentication
	addAuthentication(req, &cluster.AccessCred)

	// Perform request
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != 200 {
		return ""
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	// Parse response to find master node
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		// The master field will be '*' for the elected master node
		if len(fields) >= 2 && fields[1] == "*" {
			return fields[0]
		}
	}

	return ""
}

// GetCurrentMasterEndpointForCluster retrieves the API endpoint for the current master node
// Returns the endpoint URL (https://hostname:9200/), or empty string if unable to determine
func GetCurrentMasterEndpointForCluster(clusterName string) string {
	currentMaster := GetCurrentMasterForCluster(clusterName)
	if currentMaster == "" {
		return ""
	}

	// Get cluster data for port information
	types.ClustersMu.RLock()
	cluster, exists := types.AllClusters[clusterName]
	types.ClustersMu.RUnlock()

	port := "9200"
	if exists && cluster.ClusterPort != "" {
		port = cluster.ClusterPort
	}

	return fmt.Sprintf("https://%s:%s/", currentMaster, port)
}

// addAuthentication adds authentication headers to the HTTP request
func addAuthentication(req *http.Request, cred *types.AccessCred) {
	if cred == nil {
		return
	}

	switch cred.Preferred {
	case 1: // API Key
		if cred.APIKey != "" {
			req.Header.Set("Authorization", "ApiKey "+cred.APIKey)
		}
	case 2: // Basic Auth
		if cred.UserID != "" && cred.Password != "" {
			req.SetBasicAuth(cred.UserID, cred.Password)
		}
	case 3: // Certificate-based auth
		// Certificate auth is handled at transport level, not via headers
	}
}
