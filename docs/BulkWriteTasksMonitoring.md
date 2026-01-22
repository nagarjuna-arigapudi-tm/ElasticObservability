# Bulk Write Tasks Monitoring

Complete guide to the bulk write tasks monitoring feature in ElasticObservability.

## Overview

The bulk write tasks monitoring feature provides real-time visibility into active bulk write operations across Elasticsearch clusters. It queries the `_tasks` API every minute to collect detailed metrics about bulk write operations, aggregating data at shard, node, index, and cluster levels with configurable historical retention.

## Key Features

- **Real-time Monitoring**: Collects data every minute from all clusters
- **Multi-level Aggregation**: Metrics aggregated at shard, node, index, and cluster levels
- **Historical Tracking**: Configurable history (10-180 snapshots, default 60)
- **Pre-sorted Data**: All data pre-sorted by tasks, time taken, and requests
- **Thread-safe**: Safe concurrent access via RWMutex
- **Dashboard Ready**: Complete REST API for integration
- **Master Node Querying**: Queries only the current elected master node
- **Zone Awareness**: Includes availability zone information when available

## Architecture

### Data Flow

```
┌─────────────────────────────────────────────────────────┐
│  Job: getTDataWriteBulk_sTasks (every 1 minute)       │
└───────────────────┬─────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────┐
│  Query Master Node: /_tasks?pretty&human&detailed=true  │
└───────────────────┬─────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────┐
│  Filter Tasks: action = "indices:data/write/bulk[s]"   │
└───────────────────┬─────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────┐
│  Parse Description: "requests[236], index[idx][2]"      │
└───────────────────┬─────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────┐
│  Aggregate Data:                                         │
│  • Shard Level  (index_0, index_1, ...)                │
│  • Node Level   (host1, host2, ...)                    │
│  • Index Level  (index_name aggregated)                │
│  • Cluster Level (complete overview)                    │
└───────────────────┬─────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────┐
│  Store in Global History (rolling buffer)               │
│  AllClusterDataWriteBulk_sTasksHistory                  │
└─────────────────────────────────────────────────────────┘
```

### Data Structures

#### 1. AggShardTaskDataWriteBulk_s
Aggregates metrics for a specific shard.

```go
type AggShardTaskDataWriteBulk_s struct {
    NumberOfTasks     uint8   // Number of active tasks
    TotalRequests     uint    // Sum of all requests
    TotalTimeTaken_ms uint64  // Sum of running time (ms)
}
```

**Example:**
```json
{
  "numberOfTasks": 5,
  "totalRequests": 1200,
  "totalTimeTakenMs": 8500
}
```

#### 2. NodeDataWriteBulk_sTasks
Aggregates all bulk write task data for a node.

```go
type NodeDataWriteBulk_sTasks struct {
    TotalWiteBulk_sTasks         uint     // Total tasks on node
    TotalWriteBulk_sRequests     uint     // Total requests on node
    TotalWrietBulk_sTimeTaken_ms uint64   // Total time on node
    Zone                         string   // Availability zone
    DataWriteBulk_sByShard       map[string]*AggShardTaskDataWriteBulk_s
    SortedShardsOnTasks          []string // Sorted by task count
    SortedShardsOnTimetaken      []string // Sorted by time taken
    SortedShardsOnRequest        []string // Sorted by requests
}
```

#### 3. ClusterDataWriteBulk_sTasks
Complete snapshot of cluster bulk write activity.

```go
type ClusterDataWriteBulk_sTasks struct {
    SnapShotTime                int64    // Epoch seconds
    DataWriteBulk_sTasksByNode  map[string]*NodeDataWriteBulk_sTasks
    SortedHostsOnTasks          []string // Busiest hosts
    SortedHostsOnTimetaken      []string // Slowest hosts
    SortedHostsOnRequest        []string // Hosts with most requests
    DataWriteBulk_sTasksByIndex map[string]*AggShardTaskDataWriteBulk_s
    IndicesSortedonTasks        []string // Busiest indices
    IndicesSortedOnRequests     []string // Indices with most requests
    IndicesSortedOnTimetaken    []string // Slowest indices
}
```

#### 4. ClusterDataWriteBulk_sTasksHistory
Historical tracking for a cluster.

```go
type ClusterDataWriteBulk_sTasksHistory struct {
    LatestSnapShotTime             int64  // Epoch seconds
    HistorySize                    uint   // Number of snapshots to retain
    ClusterName                    string
    PtrClusterDataWriteBulk_sTasks []*ClusterDataWriteBulk_sTasks
}
```

## Configuration

### Job Configuration (scheduled_jobs.yaml)

```yaml
jobs:
  - name: monitor_bulk_write_tasks
    type: preDefined
    internalJobName: getTDataWriteBulk_sTasks
    enabled: true
    schedule:
      interval: 1m       # Run every minute
      initialWait: 2m    # Wait 2 minutes after startup
    parameters:
      excludeClusters: []      # Clusters to exclude
      includeClusters: []      # Clusters to include (overrides excludeClusters)
      historySize: 60          # Number of snapshots (10-180, default 60)
      insecureTLS: false       # Skip TLS verification
```

### Parameters

#### excludeClusters
**Type:** `[]string`  
**Default:** `[]` (empty)  
**Description:** List of cluster names to exclude from monitoring.

**Example:**
```yaml
excludeClusters: ["dev-cluster", "test-cluster"]
```

#### includeClusters
**Type:** `[]string`  
**Default:** `[]` (empty)  
**Description:** List of cluster names to monitor. When specified, only these clusters are monitored (excludeClusters is ignored).

**Example:**
```yaml
includeClusters: ["prod-cluster-01", "prod-cluster-02"]
```

#### historySize
**Type:** `uint`  
**Default:** `60`  
**Range:** `10-180`  
**Description:** Number of historical snapshots to maintain. Values outside range are automatically adjusted.

**Examples:**
- `60` = 1 hour of history (at 1-minute intervals)
- `120` = 2 hours of history
- `180` = 3 hours of history (maximum)

#### insecureTLS
**Type:** `bool`  
**Default:** `false`  
**Description:** Whether to skip TLS certificate verification. Use only for non-production environments.

## API Endpoints

### 1. List Clusters with Bulk Tasks History

**Endpoint:** `GET /api/bulkTasks/clusters`

**Description:** Returns list of all clusters that have bulk tasks monitoring data.

**Response:**
```json
{
  "clusters": [
    {
      "clusterName": "prod-cluster-01",
      "historySize": 60,
      "latestSnapshotTime": 1704567890
    },
    {
      "clusterName": "prod-cluster-02",
      "historySize": 60,
      "latestSnapshotTime": 1704567892
    }
  ],
  "count": 2
}
```

**Use Cases:**
- Build cluster selection dropdowns
- Display cluster monitoring status
- Navigate between clusters

---

### 2. Get Complete Bulk Tasks History

**Endpoint:** `GET /api/bulkTasks/{clusterName}`

**Description:** Returns complete history (all snapshots) for a cluster.

**Response Structure:**
```json
{
  "clusterName": "prod-cluster-01",
  "historySize": 60,
  "latestSnapshotTime": 1704567890,
  "snapshots": [
    { /* Latest snapshot at index 0 */ },
    { /* Previous snapshot at index 1 */ },
    { /* ... more snapshots ... */ }
  ],
  "snapshotCount": 60
}
```

**Use Cases:**
- Historical trend analysis
- Capacity planning
- Performance comparisons over time
- Identifying patterns

---

### 3. Get Latest Bulk Tasks Snapshot

**Endpoint:** `GET /api/bulkTasks/{clusterName}/latest`

**Description:** Returns only the most recent snapshot (optimized for real-time dashboards).

**Complete Response Example:**
```json
{
  "clusterName": "prod-cluster-01",
  "latestSnapshotTime": 1704567890,
  "snapshot": {
    "snapShotTime": 1704567890,
    "dataWriteBulkSTasksByNode": {
      "host1.example.com": {
        "totalWriteBulkSTasks": 45,
        "totalWriteBulkSRequests": 12500,
        "totalWriteBulkSTimeTakenMs": 85000,
        "zone": "us-east-1a",
        "dataWriteBulkSByShard": {
          "logs-app_0": {
            "numberOfTasks": 5,
            "totalRequests": 1200,
            "totalTimeTakenMs": 8500
          },
          "logs-app_1": {
            "numberOfTasks": 3,
            "totalRequests": 800,
            "totalTimeTakenMs": 5000
          },
          "metrics-system_0": {
            "numberOfTasks": 4,
            "totalRequests": 950,
            "totalTimeTakenMs": 6200
          }
        },
        "sortedShardsOnTasks": ["logs-app_0", "metrics-system_0", "logs-app_1"],
        "sortedShardsOnTimetaken": ["logs-app_0", "metrics-system_0", "logs-app_1"],
        "sortedShardsOnRequest": ["logs-app_0", "metrics-system_0", "logs-app_1"]
      },
      "host2.example.com": {
        "totalWriteBulkSTasks": 38,
        "totalWriteBulkSRequests": 10000,
        "totalWriteBulkSTimeTakenMs": 72000,
        "zone": "us-east-1b",
        "dataWriteBulkSByShard": { /* ... */ },
        "sortedShardsOnTasks": [ /* ... */ ],
        "sortedShardsOnTimetaken": [ /* ... */ ],
        "sortedShardsOnRequest": [ /* ... */ ]
      }
    },
    "sortedHostsOnTasks": ["host1.example.com", "host2.example.com"],
    "sortedHostsOnTimetaken": ["host1.example.com", "host2.example.com"],
    "sortedHostsOnRequest": ["host1.example.com", "host2.example.com"],
    "dataWriteBulkSTasksByIndex": {
      "logs-app": {
        "numberOfTasks": 25,
        "totalRequests": 5000,
        "totalTimeTakenMs": 45000
      },
      "metrics-system": {
        "numberOfTasks": 18,
        "totalRequests": 3200,
        "totalTimeTakenMs": 28000
      }
    },
    "indicesSortedonTasks": ["logs-app", "metrics-system"],
    "indicesSortedOnRequests": ["logs-app", "metrics-system"],
    "indicesSortedOnTimetaken": ["logs-app", "metrics-system"]
  }
}
```

**Use Cases:**
- Real-time dashboard updates
- Current cluster status
- Immediate performance insights
- Hotspot identification

## Dashboard Integration Examples

### Example 1: Top 5 Busiest Hosts

```javascript
async function getTopBusiestHosts(clusterName) {
  const response = await fetch(`/api/bulkTasks/${clusterName}/latest`);
  const data = await response.json();
  
  const topHosts = data.snapshot.sortedHostsOnTasks.slice(0, 5);
  
  return topHosts.map(hostName => {
    const hostData = data.snapshot.dataWriteBulkSTasksByNode[hostName];
    return {
      host: hostName,
      tasks: hostData.totalWriteBulkSTasks,
      requests: hostData.totalWriteBulkSRequests,
      timeMs: hostData.totalWriteBulkSTimeTakenMs,
      zone: hostData.zone
    };
  });
}
```

### Example 2: Trending Over Time

```javascript
async function getBulkTasksTrend(clusterName, metric = 'tasks') {
  const response = await fetch(`/api/bulkTasks/${clusterName}`);
  const data = await response.json();
  
  return data.snapshots.map(snapshot => {
    const totalTasks = Object.values(snapshot.dataWriteBulkSTasksByNode)
      .reduce((sum, node) => sum + node.totalWriteBulkSTasks, 0);
    
    return {
      timestamp: snapshot.snapShotTime,
      value: totalTasks
    };
  }).reverse(); // Oldest first for time-series chart
}
```

### Example 3: Shard-Level Hotspots

```javascript
async function getShardHotspots(clusterName, hostName) {
  const response = await fetch(`/api/bulkTasks/${clusterName}/latest`);
  const data = await response.json();
  
  const hostData = data.snapshot.dataWriteBulkSTasksByNode[hostName];
  if (!hostData) return [];
  
  // Get top 10 busiest shards
  return hostData.sortedShardsOnTasks.slice(0, 10).map(shard => {
    const shardData = hostData.dataWriteBulkSByShard[shard];
    return {
      shard: shard,
      tasks: shardData.numberOfTasks,
      requests: shardData.totalRequests,
      timeMs: shardData.totalTimeTakenMs
    };
  });
}
```

## Metrics Interpretation

### Task Count
**Field:** `numberOfTasks` / `totalWriteBulkSTasks`

**Meaning:** Number of concurrent bulk write operations.

**Normal Range:** 0-50 per node  
**Warning:** > 50 per node  
**Critical:** > 100 per node

**Action:** High task counts may indicate:
- Heavy indexing load
- Slow bulk operations
- Need for additional nodes

### Request Count
**Field:** `totalRequests` / `totalWriteBulkSRequests`

**Meaning:** Total number of documents/operations being processed.

**Action:** Use to identify:
- Which indices receive most writes
- Write distribution across cluster
- Bulk request sizing

### Time Taken
**Field:** `totalTimeTakenMs` / `totalWriteBulkSTimeTakenMs`

**Meaning:** Cumulative time spent processing bulk operations (milliseconds).

**Action:** High values indicate:
- Slow disk I/O
- CPU constraints
- Network issues
- Large documents

## Troubleshooting

### No Data Available

**Symptoms:**
- API returns 404: "Bulk tasks history not available"
- Empty snapshots array

**Possible Causes:**
1. Job is disabled in configuration
2. Job hasn't run yet (check initialWait)
3. All clusters are excluded
4. Master endpoint not detected

**Solutions:**
```bash
# Check job status
curl http://localhost:9092/api/jobs | jq '.jobs[] | select(.name=="monitor_bulk_write_tasks")'

# Verify master endpoints
curl http://localhost:9092/api/bulkTasks/clusters

# Manually trigger job
curl -X POST http://localhost:9092/api/jobs/monitor_bulk_write_tasks/trigger
```

### High Memory Usage

**Symptoms:**
- Application memory grows over time
- OOM errors

**Possible Causes:**
- History size too large
- Too many clusters being monitored

**Solutions:**
1. Reduce history size:
   ```yaml
   historySize: 30  # Reduce from 60
   ```

2. Use includeClusters to monitor only critical clusters:
   ```yaml
   includeClusters: ["prod-cluster-01", "prod-cluster-02"]
   ```

### Missing Zone Information

**Symptoms:**
- `zone` field is empty string

**Cause:**
- Zone information not available in cluster metadata

**Solution:**
- Ensure zones are specified in clusters CSV file
- Zone is optional and doesn't affect functionality

## Performance Considerations

### API Query Performance

**Latest Snapshot (`/latest`):**
- Fast: Returns only most recent data
- Minimal memory allocation
- Best for real-time dashboards
- Recommended polling: 10-30 seconds

**Complete History:**
- Slower: Returns all snapshots
- Higher memory allocation
- Best for trend analysis
- Recommended polling: 60+ seconds

### Job Performance

**Execution Time:** Typically 100-500ms per cluster

**Factors Affecting Performance:**
- Number of active bulk tasks
- Number of nodes in cluster
- Network latency to master node
- TLS negotiation overhead

**Optimization:**
- Job queries only master node (not all nodes)
- Uses connection pooling
- Minimal JSON parsing
- Pre-computed sorted lists

## Security Considerations

### Authentication

The job uses cluster credentials configured in:
- API Keys (preferred)
- Basic Auth (username/password)
- Certificate-based auth

### TLS Verification

**Production:**
```yaml
insecureTLS: false  # Verify certificates (default)
```

**Development/Testing:**
```yaml
insecureTLS: true   # Skip verification (not recommended)
```

### API Access

No authentication required for API endpoints (internal service).

For external access, place behind authenticated reverse proxy.

## Best Practices

### 1. Monitor Appropriate Clusters

```yaml
# Production example - monitor only prod clusters
includeClusters: ["prod-cluster-01", "prod-cluster-02", "prod-cluster-03"]
```

### 2. Set Appropriate History Size

```yaml
# For real-time dashboards: 30 minutes
historySize: 30

# For trend analysis: 2 hours
historySize: 120

# For capacity planning: 3 hours (maximum)
historySize: 180
```

### 3. Dashboard Polling Intervals

```javascript
// Real-time updates: 15-30 seconds
setInterval(() => updateDashboard(), 15000);

// Trend analysis: 60+ seconds
setInterval(() => updateTrends(), 60000);
```

### 4. Alert Thresholds

Set alerts based on your cluster capacity:

```javascript
const thresholds = {
  tasks: {
    warning: 50,   // per node
    critical: 100  // per node
  },
  timeMs: {
    warning: 5000,   // per task
    critical: 10000  // per task
  }
};
```

## See Also

- [API Reference](./API_Reference.md) - Complete API documentation
- [Thread Pool Write Queue](./ThreadPoolWriteQueue.md) - Related monitoring feature
- [Write Pressure Detection](./WritePressureDetection.md) - Complementary feature
- [README](../README.md) - Main documentation
