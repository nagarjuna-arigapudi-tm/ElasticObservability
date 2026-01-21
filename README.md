

# ElasticObservability

A comprehensive Go-based service for monitoring and managing multiple Elasticsearch clusters with focus on indexing rate analysis and cluster health tracking.

## Features

- **Multi-Cluster Management**: Monitor and manage multiple Elasticsearch clusters
- **Indexing Rate Analysis**: Track and analyze indexing rates per shard across time windows
- **Thread Pool Monitoring**: Real-time monitoring of thread pool write queue depths from monitoring cluster
- **Bulk Write Tasks Monitoring**: Track active bulk write operations across clusters with detailed shard-level metrics
- **Daily Statistics**: Track index growth and changes over configurable time periods
- **Stale Index Detection**: Identify indices with no modifications over n days
- **Job Scheduling**: Flexible job scheduling with cron, interval, and dependency support
- **REST API**: Comprehensive API for cluster data, metrics, indexing rates, and queue monitoring
- **CSV Data Import**: Load and update cluster configurations from CSV files
- **Historical Data Tracking**: Maintain configurable history of indices snapshots
- **Prometheus Metrics**: Export application and job metrics for monitoring
- **Dual Logging**: Separate application and job logs
- **Parallel Processing**: Configurable parallel execution for monitoring jobs

## Architecture

### Data Structures

1. **Cluster Data**: Maintains information about all monitored clusters
   - Cluster metadata (name, UUID, endpoints)
   - Node information
   - Access credentials
   - Active endpoint tracking

2. **Indices History**: Historical snapshots of indices for each cluster
   - Configurable retention (default: 20 snapshots)
   - Thread-safe rolling buffer
   - Per-index metrics (health, doc count, storage, shards)

3. **Indexing Rate**: Calculated metrics for indexing rate per shard
   - From creation
   - Last 3 minutes
   - Last 15 minutes
   - Last 60 minutes

4. **Daily Statistics**: Daily snapshots of index statistics
   - Configurable retention (default: 30 days)
   - Persistent backup to disk
   - Tracks document count and storage size
   - Automatic rollover and cleanup

5. **Thread Pool Write Queue**: Real-time queue depth monitoring
   - Collects data from Elasticsearch monitoring cluster
   - Configurable data retention (default: 6 data sets × 20 points = 120 points)
   - Rolling buffer for historical trends
   - Missing data tracking with flags
   - Per-host and per-cluster views

### Predefined Jobs

#### 1. loadFromMasterCSV
Loads cluster and node data from CSV file with configurable field mappings.

**Configuration Example:**
```yaml
jobs:
  - name: load_clusters
    type: preDefined
    internalJobName: loadFromMasterCSV
    enabled: true
    initJob: true
    parameters:
      csv_fileName: ./data/clusters.csv
      inputMapping:
        constant:
          insecureTLS: true
          port: "9200"
        straight:
          clusterName: "Cluster Name"
          clusterSAN: "API Endpoint"
          kibanaSAN: "Kibana Endpoint"
          owner: "Owner"
          hostName: "Node Name"
        derived:
          - field: active
            column: "Use Case"
            function: boolStringCompare
            arg: ["managed"]
          - field: env
            column: "Environment"
            function: strStringCompare
            arg: [["dev","development"], ["qa","testing","uat"], ["prd","production","prod"]]
            retVal: ["dev", "uat", "prd"]
```

#### 2. updateActiveEndpoint
Validates connectivity and updates active endpoints for clusters.

**Configuration Example:**
```yaml
jobs:
  - name: update_endpoints
    type: preDefined
    internalJobName: updateActiveEndpoint
    enabled: true
    initJob: true
    dependsOn: ["load_clusters"]
    parameters:
      excludeClusters: []
```

#### 3. runCatIndices
Fetches current indices information from all clusters using `_cat/indices` API.

**Configuration Example:**
```yaml
jobs:
  - name: fetch_indices
    type: preDefined
    internalJobName: runCatIndices
    enabled: true
    schedule:
      interval: 3m
      initialWait: 30s
    parameters:
      excludeClusters: []
```

#### 4. analyseIngest
Analyzes indexing rates based on historical data.

**Configuration Example:**
```yaml
jobs:
  - name: analyze_rates
    type: preDefined
    internalJobName: analyseIngest
    enabled: true
    dependsOn: ["fetch_indices"]
    parameters:
      excludeClusters: []
```

#### 5. updateAccessCredentials
Updates cluster access credentials and ClusterUUID from CSV file.

**Configuration Example:**
```yaml
jobs:
  - name: update_credentials
    type: preDefined
    internalJobName: updateAccessCredentials
    enabled: true
    parameters:
      csv_fileName: ./data/credentials.csv
```

#### 6. updateStatsByDay
Maintains daily statistics for indices with persistent backup.

**Configuration Example:**
```yaml
jobs:
  - name: update_statsByDay
    type: preDefined
    internalJobName: updateStatsByDay
    enabled: true
    dependsOn: ["fetch_indices"]
    schedule:
      interval: 24h
      initialWait: 2m
    parameters:
      excludeClusters: []
```

#### 7. getThreadPoolWriteQueue
Monitors thread pool write queue depths from Elasticsearch monitoring cluster.

**Configuration Example:**
```yaml
jobs:
  - name: get_host_threadpool_metrics
    type: preDefined
    internalJobName: getThreadPoolWriteQueue
    enabled: false  # Requires monitoring cluster setup
    schedule:
      interval: 10m
      initialWait: 3m
    parameters:
      excludeClusters: []
      spanInterval: "30s"
      timeSpan: "10m"
      parallelRoutines: 5
      insecureTLS: false
      APIKEY: "your-monitoring-api-key"
      APIEndPoints:
        - "https://monitoring-es:9200/.monitoring-es-*/_search"
```

See [Thread Pool Write Queue Documentation](./docs/ThreadPoolWriteQueue.md) for detailed information.

## Configuration

### Global Configuration

Create `config.yaml`:

```yaml
logLevel: info
metricsPort: 9091
apiPort: 9092
historyForIndices: 20
historyOfStatsInDays: 30
backupOfStatsInDays: ./data/backup/statsInDays.json
threadPoolWriteQueueDataSets: 6
out_dir: ./outputs
config_dir: ./configs
cert:
  cert: /path/to/cert.pem
  key: /path/to/key.pem
  caCert: /path/to/ca.pem
```

**Configuration Parameters:**
- `logLevel`: Logging level (debug, info, warn, error)
- `metricsPort`: Port for Prometheus metrics (default: 9091)
- `apiPort`: Port for REST API (default: 9092)
- `historyForIndices`: Number of index snapshots to retain (default: 20)
- `historyOfStatsInDays`: Days of daily statistics to retain (default: 30)
- `backupOfStatsInDays`: Path to daily statistics backup file
- `threadPoolWriteQueueDataSets`: Number of data sets for TPWQueue (default: 6)
- `out_dir`: Directory for generated outputs
- `config_dir`: Directory for job configurations
- `cert`: TLS certificate configuration (optional)

### Job Configuration

Place job configuration files in the `configs/` directory. Both YAML and JSON formats are supported.

### One-Time Jobs

Place one-time job configurations in `configs/oneTime/` directory. After execution:
- Successful jobs are moved to `configs/processedOneTime/`
- Failed jobs are moved with `.failed` extension
- Unparsable jobs are moved with `.unparsed` extension

## API Endpoints

### Cluster Management
- `GET /api/clusters` - List all managed clusters
- `GET /api/clusters/{clusterName}/nodes` - Get nodes for a specific cluster

### Indexing Rate
- `GET /api/indexingRate/{clusterName}` - Get indexing rate metrics for all indices in a cluster

### Stale Indices
- `GET /api/staleIndices/{clusterName}/{days}` - Get indices not modified in n days

### Thread Pool Write Queue
- `GET /api/tpwqueue/{clusterName}` - Get TPWQueue metrics for all hosts in a cluster
- `GET /api/tpwqueue/{clusterName}/{hostName}` - Get TPWQueue metrics for a specific host

### Bulk Write Tasks Monitoring
- `GET /api/bulkTasks/clusters` - List all clusters with bulk tasks history
- `GET /api/bulkTasks/{clusterName}` - Get complete bulk tasks history for a cluster
- `GET /api/bulkTasks/{clusterName}/latest` - Get latest bulk tasks snapshot for a cluster

### Application Status
- `GET /api/status` - Application health and status
- `GET /api/jobs` - Job status and execution statistics

### Job Control
- `POST /api/jobs/{jobName}/trigger` - Manually trigger a job

### Metrics
- `GET /metrics` - Prometheus-format metrics (on metricsPort)

See [API Reference](./docs/API_Reference.md) for detailed documentation of all endpoints.

## Building and Running

### Prerequisites

- Go 1.21 or later
- Access to Elasticsearch clusters (credentials required)

### Build

```bash
go build -o elasticobservability ./cmd/main.go
```

### Run

```bash
./elasticobservability -config config.yaml
```

### Command-line Flags

```bash
-config string
    Path to configuration file (default "config.yaml")
-log-dir string
    Directory for log files (default "./logs")
```

## Project Structure

```
ElasticObservability/
├── cmd/
│   └── main.go                 # Application entry point
├── pkg/
│   ├── api/                    # REST API handlers
│   │   └── handlers.go
│   ├── config/                 # Configuration management
│   │   └── config.go
│   ├── jobs/                   # Predefined job implementations
│   │   ├── load_csv.go
│   │   ├── update_endpoint.go
│   │   ├── cat_indices.go
│   │   └── analyse_ingest.go
│   ├── logger/                 # Logging system
│   │   └── logger.go
│   ├── scheduler/              # Job scheduling
│   │   └── scheduler.go
│   ├── types/                  # Data structures
│   │   └── types.go
│   └── utils/                  # Utility functions
│       ├── utils.go
│       └── csvparser.go
├── configs/                    # Job configurations
│   ├── oneTime/               # One-time jobs
│   └── processedOneTime/      # Processed one-time jobs
├── outputs/                    # Generated outputs
├── logs/                       # Application and job logs
├── go.mod
├── go.sum
└── README.md
```

## Timestamp Format

All timestamps in the application are stored as epoch milliseconds for consistency and ease of calculation.

## Thread Safety

The application uses mutexes to ensure thread-safe access to shared data structures:
- `ClustersMu` - Protects cluster data (RWMutex)
- `HistoryMu` - Protects indices history (RWMutex)
- `IndexingRateMu` - Protects indexing rate data (RWMutex)
- `StatsByDayMu` - Protects daily statistics data (RWMutex)
- `TPWQueueMu` - Protects thread pool write queue data (RWMutex)

Each `IndicesHistory` also has its own internal mutex for thread-safe rolling operations.

**Best Practices:**
- Use RLock() for read operations to allow concurrent reads
- Hold locks for minimal duration
- API endpoints copy data before releasing locks
- Jobs coordinate access through mutexes

## Logging

Two separate log files are maintained:

1. **Application Log** (`application.log`): General application events, errors, and status
2. **Job Log** (`job.log`): Job-specific events, execution status, and errors

Log levels: `debug`, `info`, `warn`, `error`

## Index Name Parsing

The application intelligently parses Elasticsearch index names to extract:
- Base name (without sequence number or timestamp)
- Sequence number
- Handles various formats including data streams

Examples:
- `.kibana_task_manager_7.17.2_001` → base: `.kibana_task_manager_7.17.2`, seq: 1
- `.ds-logs-2025.09.17-000012` → base: `.ds-logs`, seq: 12
- `169736-elk-transforms` → base: `169736-elk-transforms`, seq: 0

## Contributing

This is a specialized tool for Elasticsearch observability. Contributions should maintain thread safety, proper error handling, and logging standards.

## License

Internal use only.
