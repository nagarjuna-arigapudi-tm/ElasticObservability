package jobs

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ElasticObservability/pkg/logger"
	"ElasticObservability/pkg/types"
	"ElasticObservability/pkg/utils"
)

var (
	// Write pressure tracking variables
	oldRunTime      int64
	previousRunTime int64
	lastRunTime     int64

	// Write pressure log file
	writePressureLogger *log.Logger
)

// CheckForWritePressure detects write pressure on Elasticsearch hosts
func CheckForWritePressure(ctx context.Context, params map[string]interface{}) error {
	logger.JobInfo("checkForWritePressure", "Starting write pressure check")

	// Get parameters
	excludeClusters := getStringSliceParam(params, "excludeClusters")
	thresholdValue := getIntParam(params, "thresholdValue", 700)
	noOfConsecutiveIntervals := getIntParam(params, "noOfConsecutiveIntervals", 3)
	considerMissingDataPoint := getStringParam(params, "considerMissingDataPoint", "missing")

	// Validate considerMissingDataPoint parameter
	if considerMissingDataPoint != "missing" && considerMissingDataPoint != "nonOffending" && considerMissingDataPoint != "offending" {
		return fmt.Errorf("invalid considerMissingDataPoint value: %s (must be 'missing', 'nonOffending', or 'offending')", considerMissingDataPoint)
	}

	logger.JobInfo("checkForWritePressure", "Config: threshold=%d, consecutiveIntervals=%d, missingDataPoint=%s",
		thresholdValue, noOfConsecutiveIntervals, considerMissingDataPoint)

	// Initialize write pressure logger if not already done
	if writePressureLogger == nil {
		logDir := "./logs"
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create logs directory: %w", err)
		}

		logPath := filepath.Join(logDir, "writePressure.log")
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open write pressure log file: %w", err)
		}

		writePressureLogger = log.New(logFile, "", 0)
		logger.JobInfo("checkForWritePressure", "Initialized write pressure log: %s", logPath)
	}

	// Update runtime tracking variables
	oldRunTime = previousRunTime
	previousRunTime = lastRunTime
	lastRunTime = time.Now().Unix()

	logger.JobInfo("checkForWritePressure", "Runtime tracking: old=%d, previous=%d, last=%d",
		oldRunTime, previousRunTime, lastRunTime)

	// Build cluster list for assessment
	types.TPWQueueMu.RLock()
	clusterList := make([]string, 0)
	for clusterName := range types.AllThreadPoolWriteQueues {
		if !utils.Contains(excludeClusters, clusterName) {
			clusterList = append(clusterList, clusterName)
		}
	}
	types.TPWQueueMu.RUnlock()

	logger.JobInfo("checkForWritePressure", "Checking %d clusters for write pressure", len(clusterList))

	// Process each cluster
	totalHostsChecked := 0
	pressureEventsDetected := 0

	for _, clusterName := range clusterList {
		hostsChecked, eventsDetected := checkClusterForWritePressure(
			clusterName,
			thresholdValue,
			noOfConsecutiveIntervals,
			considerMissingDataPoint,
		)
		totalHostsChecked += hostsChecked
		pressureEventsDetected += eventsDetected
	}

	// Clean up old events from WritePressureMap
	cleanupOldEvents(oldRunTime)

	logger.JobInfo("checkForWritePressure", "Completed: checked %d hosts, detected %d pressure events",
		totalHostsChecked, pressureEventsDetected)

	return nil
}

// checkClusterForWritePressure checks all hosts in a cluster for write pressure
func checkClusterForWritePressure(clusterName string, threshold, consecutiveIntervals int, missingDataMode string) (int, int) {
	// Get a private copy of cluster's TPWQueue data
	types.TPWQueueMu.RLock()
	clusterData, exists := types.AllThreadPoolWriteQueues[clusterName]
	if !exists {
		types.TPWQueueMu.RUnlock()
		return 0, 0
	}

	// Make a shallow copy to avoid holding lock too long
	hostnames := make([]string, len(clusterData.HostnameList))
	copy(hostnames, clusterData.HostnameList)

	hostDataCopy := make(map[string]*types.TPWQueue)
	for hostname, tpwq := range clusterData.HostTPWQueue {
		hostDataCopy[hostname] = tpwq
	}
	types.TPWQueueMu.RUnlock()

	hostsChecked := 0
	eventsDetected := 0

	// Check each host for write pressure
	for _, hostname := range hostnames {
		tpwq, exists := hostDataCopy[hostname]
		if !exists {
			continue
		}

		hostsChecked++

		// Check if this host is under write pressure
		isPressured, eventStartTime := isHostUnderPressure(tpwq, threshold, consecutiveIntervals, missingDataMode)

		if isPressured {
			// Create event and check if it's new
			if recordWritePressureEvent(hostname, clusterName, eventStartTime) {
				eventsDetected++
			}
		}
	}

	return hostsChecked, eventsDetected
}

// isHostUnderPressure checks if a host is experiencing write pressure
func isHostUnderPressure(tpwq *types.TPWQueue, threshold, consecutiveIntervals int, missingDataMode string) (bool, int64) {
	if tpwq == nil || len(tpwq.ThreadPoolWriteQueues) == 0 {
		return false, 0
	}

	// Process data based on missingDataMode
	switch missingDataMode {
	case "missing":
		return checkPressureWithMissingFiltered(tpwq, threshold, consecutiveIntervals)
	case "nonOffending":
		return checkPressureWithMissingAsNonOffending(tpwq, threshold, consecutiveIntervals)
	case "offending":
		return checkPressureWithMissingAsOffending(tpwq, threshold, consecutiveIntervals)
	default:
		return checkPressureWithMissingFiltered(tpwq, threshold, consecutiveIntervals)
	}
}

// checkPressureWithMissingFiltered removes missing data points and checks sequential elements
func checkPressureWithMissingFiltered(tpwq *types.TPWQueue, threshold, consecutiveIntervals int) (bool, int64) {
	// Build array of valid data points (filtering out missing ones)
	type dataPoint struct {
		timestamp int64
		value     uint32
	}

	validPoints := make([]dataPoint, 0)
	for i := 0; i < len(tpwq.ThreadPoolWriteQueues); i++ {
		if tpwq.DataExists[i] {
			validPoints = append(validPoints, dataPoint{
				timestamp: tpwq.TimeStamps[i],
				value:     tpwq.ThreadPoolWriteQueues[i],
			})
		}
	}

	// Need at least consecutiveIntervals valid points
	if len(validPoints) < consecutiveIntervals {
		return false, 0
	}

	// Check for consecutive threshold violations (oldest to newest)
	for i := len(validPoints) - 1; i >= consecutiveIntervals-1; i-- {
		consecutiveCount := 0
		var startTime int64

		for j := 0; j < consecutiveIntervals; j++ {
			if validPoints[i-j].value >= uint32(threshold) {
				consecutiveCount++
				if j == consecutiveIntervals-1 {
					startTime = validPoints[i-j].timestamp
				}
			} else {
				break
			}
		}

		if consecutiveCount == consecutiveIntervals {
			return true, startTime
		}
	}

	return false, 0
}

// checkPressureWithMissingAsNonOffending treats missing data as below threshold
func checkPressureWithMissingAsNonOffending(tpwq *types.TPWQueue, threshold, consecutiveIntervals int) (bool, int64) {
	if len(tpwq.ThreadPoolWriteQueues) < consecutiveIntervals {
		return false, 0
	}

	// Check from oldest to newest
	for i := len(tpwq.ThreadPoolWriteQueues) - 1; i >= consecutiveIntervals-1; i-- {
		consecutiveCount := 0
		var startTime int64

		for j := 0; j < consecutiveIntervals; j++ {
			idx := i - j
			// If data doesn't exist, treat as non-offending (below threshold) - breaks the sequence
			if !tpwq.DataExists[idx] {
				break
			}

			if tpwq.ThreadPoolWriteQueues[idx] >= uint32(threshold) {
				consecutiveCount++
				if j == consecutiveIntervals-1 {
					startTime = tpwq.TimeStamps[idx]
				}
			} else {
				break
			}
		}

		if consecutiveCount == consecutiveIntervals {
			return true, startTime
		}
	}

	return false, 0
}

// checkPressureWithMissingAsOffending treats missing data as above threshold
func checkPressureWithMissingAsOffending(tpwq *types.TPWQueue, threshold, consecutiveIntervals int) (bool, int64) {
	if len(tpwq.ThreadPoolWriteQueues) < consecutiveIntervals {
		return false, 0
	}

	// Check from oldest to newest
	for i := len(tpwq.ThreadPoolWriteQueues) - 1; i >= consecutiveIntervals-1; i-- {
		consecutiveCount := 0
		var startTime int64

		for j := 0; j < consecutiveIntervals; j++ {
			idx := i - j
			// If data doesn't exist, treat as offending (above threshold)
			if !tpwq.DataExists[idx] {
				consecutiveCount++
				if j == consecutiveIntervals-1 {
					// Use the timestamp if available, otherwise use 0
					if tpwq.TimeStamps[idx] != 0 {
						startTime = tpwq.TimeStamps[idx]
					}
				}
			} else if tpwq.ThreadPoolWriteQueues[idx] >= uint32(threshold) {
				consecutiveCount++
				if j == consecutiveIntervals-1 {
					startTime = tpwq.TimeStamps[idx]
				}
			} else {
				break
			}
		}

		if consecutiveCount == consecutiveIntervals {
			return true, startTime
		}
	}

	return false, 0
}

// recordWritePressureEvent records a write pressure event if it's new
func recordWritePressureEvent(hostname, clusterName string, eventStartTime int64) bool {
	// Create event key: hostname_epochseconds
	eventKey := fmt.Sprintf("%s_%d", hostname, eventStartTime)

	types.WritePressureMu.Lock()
	defer types.WritePressureMu.Unlock()

	// Check if event already exists
	if _, exists := types.WritePressureMap[eventKey]; exists {
		return false // Event already recorded
	}

	// Create new event
	event := &types.WritePressureEvent{
		EventStartTime: eventStartTime,
		HostName:       hostname,
		ClusterName:    clusterName,
	}

	// Add to global map
	types.WritePressureMap[eventKey] = event

	// Log to write pressure log file
	logWritePressureEvent(event)

	logger.JobInfo("checkForWritePressure", "New write pressure event: cluster=%s, host=%s, startTime=%d",
		clusterName, hostname, eventStartTime)

	return true
}

// logWritePressureEvent writes an event to the write pressure log file
func logWritePressureEvent(event *types.WritePressureEvent) {
	currentTime := time.Now()
	observedTime := time.Unix(event.EventStartTime, 0)

	logEntry := fmt.Sprintf("[%s] [PRESSURE_EVENT] CurrentTime=%s, ObservedTime=%s, Host=%s, Cluster=%s",
		currentTime.Format("2006-01-02 15:04:05.000"),
		currentTime.Format("2006-01-02 15:04:05"),
		observedTime.Format("2006-01-02 15:04:05"),
		event.HostName,
		event.ClusterName,
	)

	if writePressureLogger != nil {
		writePressureLogger.Println(logEntry)
	}
}

// cleanupOldEvents removes events older than oldRunTime from the WritePressureMap
func cleanupOldEvents(oldRunTime int64) {
	if oldRunTime == 0 {
		// Not enough runs yet to clean up
		return
	}

	types.WritePressureMu.Lock()
	defer types.WritePressureMu.Unlock()

	removedCount := 0
	for key := range types.WritePressureMap {
		// Extract timestamp from key (format: hostname_epochseconds)
		parts := strings.Split(key, "_")
		if len(parts) < 2 {
			continue
		}

		timestamp, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
		if err != nil {
			continue
		}

		// Remove if timestamp is older than oldRunTime
		if timestamp < oldRunTime {
			delete(types.WritePressureMap, key)
			removedCount++
		}
	}

	if removedCount > 0 {
		logger.JobInfo("checkForWritePressure", "Cleaned up %d old write pressure events", removedCount)
	}
}
