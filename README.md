

# ElasticObservability

A comprehensive Go-based service for monitoring and managing multiple Elasticsearch clusters with focus on indexing rate analysis and cluster health tracking.

## Features

- **Multi-Cluster Management**: Monitor and manage multiple Elasticsearch clusters
- **Indexing Rate Analysis**: Track and analyze indexing rates per shard across time windows
- **Job Scheduling**: Flexible job scheduling with cron, interval, and dependency support
- **REST API**: Expose cluster data, metrics, and indexing rates via API endpoints
- **CSV Data Import**: Load cluster configurations from CSV files
- **Historical Data Tracking**: Maintain configurable history of indices snapshots
- **Prometheus Metrics**: Export application and job metrics for monitoring
- **Dual Logging**: Separate application and job logs

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

## Configuration

### Global Configuration

Create `config.yaml`:

```yaml
logLevel: info
metricsPort: 9091
apiPort: 9092
historyForIndices: 20
out_dir: ./outputs
config_dir: ./configs
cert:
  cert: /path/to/cert.pem
  key: /path/to/key.pem
  caCert: /path/to/ca.pem
```

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
- `GET /api/clusters/{clusterName}/nodes` - Get nodes for a cluster

### Indexing Rate

- `GET /api/indexingRate/{clusterName}` - Get indexing rate for a cluster
- Returns metrics for all indices in the cluster with rates per shard

### Application Status

- `GET /api/status` - Application health and status
- `GET /api/jobs` - Job status and execution statistics

### Metrics

- `GET /metrics` - Prometheus-format metrics (on metricsPort)

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
- `ClustersMu` - Protects cluster data
- `HistoryMu` - Protects indices history
- `IndexingRateMu` - Protects indexing rate data

Each `IndicesHistory` also has its own mutex for internal operations.

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
