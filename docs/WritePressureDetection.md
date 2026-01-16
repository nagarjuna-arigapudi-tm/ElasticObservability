# Write Pressure Detection

## Overview

The `checkForWritePressure` job is a periodic monitoring job that detects write pressure on Elasticsearch hosts by analyzing thread pool write queue metrics. It runs after the `getThreadPoolWriteQueue` job collects metrics and identifies hosts experiencing sustained high write queue depths.

## How It Works

### Detection Logic

The job analyzes thread pool write queue data for each host in every cluster and checks if the queue depth exceeds a configurable threshold for a specified number of consecutive time intervals. When a host is detected to be under write pressure, an event is logged to a dedicated log file.

### Runtime Tracking

The job maintains three runtime variables to track execution history:
- **lastRunTime**: Timestamp of the current execution
- **previousRunTime**: Timestamp of the previous execution
- **oldRunTime**: Timestamp two executions ago

These timestamps are used for event lifecycle management and cleanup of resolved pressure events.

### Event Management

Detected write pressure events are stored in a global map (`WritePressureMap`) with keys in the format `hostname_epochseconds`. Each event contains:
- **EventStartTime**: Epoch seconds when the pressure event was first observed
- **HostName**: Name of the affected host
- **ClusterName**: Name of the cluster

Events are automatically cleaned up after they are no longer detected (older than `oldRunTime`).

## Configuration Parameters

### Required Parameters

None - the job uses data collected by `getThreadPoolWriteQueue`

### Optional Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `excludeClusters` | []string | [] | List of cluster names to exclude from write pressure checks |
| `thresholdValue` | int | 700 | Thread pool write queue threshold value. Hosts with queue depth above this value for consecutive intervals are flagged |
| `noOfConsecutiveIntervals` | int | 3 | Number of consecutive intervals where the threshold must be exceeded to trigger a pressure event |
| `considerMissingDataPoint` | string | "missing" | How to handle missing data points (see below) |

### considerMissingDataPoint Options

This parameter determines how missing data points are treated during pressure detection:

#### 1. "missing" (Default)
Missing data points are filtered out, and the remaining data points are analyzed sequentially.

**Example**: If you have 20 data points and the 7th and 16th are missing:
- The dataset is treated as 18 elements
- Elements 8-15 shift to positions 7-14
- Elements 17-20 shift to positions 15-18
- Consecutive checks are performed on the compacted dataset

**Use Case**: Best when missing data is due to collection issues and you want to ignore gaps.

#### 2. "nonOffending"
Missing data points are treated as if they have values below the threshold (non-offending).

**Effect**: A missing data point breaks the sequence of consecutive threshold violations.

**Use Case**: Conservative approach - only alert when you have complete data showing sustained pressure.

#### 3. "offending"
Missing data points are treated as if they have values above the threshold (offending).

**Effect**: Missing data points count toward consecutive threshold violations.

**Use Case**: Aggressive approach - treat missing data as a sign of problems.

## Job Configuration

### Basic Configuration

```yaml
jobs:
  - name: check_writeThreadQueues
    type: preDefined
    internalJobName: checkForWritePressure
    enabled: true
    dependsOn: ["get_host_threadpool_metrics"]
    parameters:
      excludeClusters: []
      thresholdValue: 700
      noOfConsecutiveIntervals: 3
      considerMissingDataPoint: "missing"
```

### Custom Configuration Examples

#### High Sensitivity (Early Warning)
```yaml
parameters:
  thresholdValue: 500  # Lower threshold
  noOfConsecutiveIntervals: 2  # Fewer consecutive intervals
  considerMissingDataPoint: "offending"  # Treat missing as problematic
```

#### Conservative (Fewer False Positives)
```yaml
parameters:
  thresholdValue: 1000  # Higher threshold
  noOfConsecutiveIntervals: 5  # More consecutive intervals required
  considerMissingDataPoint: "nonOffending"  # Ignore missing data
```

#### Exclude Specific Clusters
```yaml
parameters:
  excludeClusters:
    - "test-cluster"
    - "dev-cluster"
  thresholdValue: 700
  noOfConsecutiveIntervals: 3
```

## Log File Output

Write pressure events are logged to `logs/writePressure.log` with the following format:

```
[2026-01-15 18:45:23.456] [PRESSURE_EVENT] CurrentTime=2026-01-15 18:45:23, ObservedTime=2026-01-15 18:35:00, Host=es-node-01, Cluster=production-cluster
```

### Log Entry Fields

- **Timestamp**: When the log entry was written
- **CurrentTime**: Current timestamp when the event was detected
- **ObservedTime**: When the pressure event actually started (from metric data)
- **Host**: Hostname experiencing write pressure
- **Cluster**: Cluster name

### Log File Management

- The log file is automatically created if it doesn't exist
- Logs are appended (not overwritten)
- Each unique event (host + start time) is logged only once
- Consider implementing log rotation for production use

## Dependencies

### Job Dependencies

```yaml
get_host_threadpool_metrics  →  check_writeThreadQueues
```

The `checkForWritePressure` job depends on `getThreadPoolWriteQueue` to collect the thread pool write queue metrics. It can be triggered either:

1. Via `dependsOn` relationship (executes after parent job completes)
2. Via `triggerJobs` parameter in the parent job
3. Both (recommended for clarity)

### Data Dependencies

- Requires `AllThreadPoolWriteQueues` global map to be populated by `getThreadPoolWriteQueue`
- Uses the `ClustersTPWQueue` data structure containing host metrics

## Monitoring and Troubleshooting

### Job Logs

The job writes detailed execution information to the job log (`logs/job.log`):

```
[2026-01-15 18:45:23.456] [INFO] [checkForWritePressure] Starting write pressure check
[2026-01-15 18:45:23.457] [INFO] [checkForWritePressure] Config: threshold=700, consecutiveIntervals=3, missingDataPoint=missing
[2026-01-15 18:45:23.458] [INFO] [checkForWritePressure] Checking 5 clusters for write pressure
[2026-01-15 18:45:24.123] [INFO] [checkForWritePressure] New write pressure event: cluster=prod-cluster, host=es-node-01, startTime=1736981100
[2026-01-15 18:45:24.234] [INFO] [checkForWritePressure] Completed: checked 25 hosts, detected 1 pressure events
[2026-01-15 18:45:24.235] [INFO] [checkForWritePressure] Cleaned up 2 old write pressure events
```

### Common Issues

#### No Events Detected
**Possible Causes**:
- Threshold value too high for your environment
- `noOfConsecutiveIntervals` value too high
- `getThreadPoolWriteQueue` not collecting data
- All clusters excluded via `excludeClusters`

**Solution**: 
- Lower threshold or consecutive intervals
- Check that thread pool metrics are being collected
- Review cluster exclusion list

#### Too Many False Positives
**Possible Causes**:
- Threshold value too low
- `noOfConsecutiveIntervals` value too low
- `considerMissingDataPoint: "offending"` treating gaps as problems

**Solution**:
- Increase threshold value
- Increase consecutive intervals requirement
- Change to `"missing"` or `"nonOffending"` mode

#### Missing Data Points
**Possible Causes**:
- Metrics collection gaps
- Network issues
- Monitoring cluster problems

**Solution**:
- Check `getThreadPoolWriteQueue` job status
- Verify monitoring cluster connectivity
- Adjust `considerMissingDataPoint` parameter based on needs

## API Access

### Get Current Write Pressure Events

The current write pressure events can be accessed via the API:

```bash
curl http://localhost:8080/api/v1/write-pressure
```

**Response**:
```json
{
  "events": {
    "es-node-01_1736981100": {
      "eventStartTime": 1736981100,
      "hostName": "es-node-01",
      "clusterName": "production-cluster"
    }
  },
  "count": 1
}
```

### Manually Trigger the Job

```bash
curl -X POST http://localhost:8080/api/v1/jobs/check_writeThreadQueues/trigger
```

## Best Practices

### 1. Tuning Thresholds

Start with the default threshold (700) and adjust based on your environment:
- **Low-volume clusters**: Consider lower thresholds (400-600)
- **High-volume clusters**: May need higher thresholds (800-1200)
- Monitor for 1-2 weeks before finalizing threshold values

### 2. Consecutive Intervals

The default value of 3 consecutive intervals balances responsiveness and false positives:
- **Increase** (4-5) for more stable environments to reduce noise
- **Decrease** (2) for critical clusters where early warning is important

### 3. Missing Data Handling

Choose based on your data quality and alerting philosophy:
- **"missing"**: Best for most use cases with occasional data gaps
- **"nonOffending"**: When you want high confidence in alerts
- **"offending"**: When missing data itself indicates problems

### 4. Alerting Integration

Consider integrating the write pressure log with your monitoring system:
- Parse `logs/writePressure.log` with log aggregation tools
- Set up alerts based on event frequency or specific clusters/hosts
- Create dashboards showing pressure trends over time

### 5. Cluster Exclusions

Exclude clusters that:
- Are in maintenance windows
- Are test/development environments
- Have known temporary write pressure that's acceptable
- Are being decommissioned

## Performance Considerations

- The job processes clusters and hosts sequentially
- Lock contention is minimized by making private copies of metric data
- Memory footprint is proportional to the number of hosts across all clusters
- Typical execution time: 100-500ms for 50-100 hosts across multiple clusters

## Example Scenarios

### Scenario 1: Production Cluster Under Load

**Situation**: A production cluster experiences sustained write pressure during business hours.

**Detection**:
```
[2026-01-15 09:15:23] [PRESSURE_EVENT] CurrentTime=2026-01-15 09:15:23, ObservedTime=2026-01-15 09:12:30, Host=es-prod-01, Cluster=prod-cluster
[2026-01-15 09:15:23] [PRESSURE_EVENT] CurrentTime=2026-01-15 09:15:23, ObservedTime=2026-01-15 09:12:30, Host=es-prod-02, Cluster=prod-cluster
```

**Action**: Multiple hosts showing pressure indicates cluster-wide issue requiring investigation.

### Scenario 2: Single Host Issue

**Situation**: One host in a cluster shows write pressure while others are normal.

**Detection**:
```
[2026-01-15 14:30:45] [PRESSURE_EVENT] CurrentTime=2026-01-15 14:30:45, ObservedTime=2026-01-15 14:27:00, Host=es-prod-03, Cluster=prod-cluster
```

**Action**: Single host issue may indicate hardware problems, hot shards, or uneven shard distribution.

### Scenario 3: Resolved Pressure

**Job Log**:
```
[2026-01-15 15:45:23] [INFO] [checkForWritePressure] Cleaned up 3 old write pressure events
```

**Interpretation**: Previously detected pressure events have been resolved (no longer exceeding threshold).

## Related Documentation

- [ThreadPoolWriteQueue.md](./ThreadPoolWriteQueue.md) - Details on the metric collection job
- [API_Reference.md](./API_Reference.md) - Complete API documentation
- [QUICKSTART.md](../QUICKSTART.md) - Getting started guide
