# ElasticObservability - Quick Start Guide

## Prerequisites

- Go 1.21 or later
- Access to Elasticsearch clusters (with credentials)

## Installation

1. **Navigate to the project directory:**
   ```bash
   cd /Users/nagarjunaarigapudi/work/ElasticObservability
   ```

2. **Download dependencies:**
   ```bash
   make deps
   ```

3. **Setup directories:**
   ```bash
   make setup
   ```

## Configuration

### 1. Update the Cluster CSV File

Edit `data/clusters.csv` with your actual Elasticsearch cluster information:

```csv
Cluster Name,API Endpoint,Kibana Endpoint,Owner,Node Name,IP Address,Zone,Data Center,Status,Environment,Node Type
my-cluster,https://my-es.com:9200,https://my-kb.com:5601,Team,node-01,10.0.1.1,zone-a,dc1,active,prod,master data
```

### 2. Configure Global Settings

Edit `config.yaml` to adjust ports and settings:

```yaml
logLevel: info
metricsPort: 9091
apiPort: 9092
historyForIndices: 20
```

### 3. Configure Jobs

The application uses two separate job configuration files:

- **`configs/initialization_jobs.yaml`** - Jobs that run once during application startup
- **`configs/scheduled_jobs.yaml`** - Jobs that run on schedules

Edit these files to adjust job schedules and parameters.

## Building

```bash
make build
```

This creates the `elasticobservability` binary.

## Running

```bash
make run
```

Or directly:
```bash
./elasticobservability -config config.yaml
```

## Testing the API

Once running, you can test the API endpoints:

### Get all clusters
```bash
curl http://localhost:9092/api/clusters
```

### Get nodes for a cluster
```bash
curl http://localhost:9092/api/clusters/prod-cluster-01/nodes
```

### Get indexing rate for a cluster
```bash
curl http://localhost:9092/api/indexingRate/prod-cluster-01
```

### Get application status
```bash
curl http://localhost:9092/api/status
```

### Get job status
```bash
curl http://localhost:9092/api/jobs
```

### Trigger a job manually
```bash
curl -X POST http://localhost:9092/api/jobs/fetch_indices/trigger
```

## Viewing Metrics

Prometheus metrics are available at:
```bash
curl http://localhost:9091/metrics
```

## Logs

Application logs and job logs are written to:
- `logs/application.log`
- `logs/job.log`

## Common Commands

```bash
# Build the application
make build

# Run the application
make run

# Clean artifacts
make clean

# Run tests
make test

# Format code
make fmt

# Show all available commands
make help
```

## Directory Structure After Setup

```
ElasticObservability/
├── config.yaml                      # Global configuration
├── configs/
│   ├── initialization_jobs.yaml    # Jobs that run at startup
│   ├── scheduled_jobs.yaml         # Jobs that run on schedule
│   ├── oneTime/                    # One-time jobs (auto-executed)
│   └── processedOneTime/           # Processed one-time jobs
├── data/
│   ├── clusters.csv                # Cluster and node data
│   └── credentials.csv             # Cluster credentials
├── logs/
│   ├── application.log             # Application logs
│   └── job.log                     # Job execution logs
└── outputs/                         # Generated outputs
```

## Authentication

To connect to Elasticsearch clusters, update the CSV file with credentials or configure them through the AccessCred structure in the code. The application supports:

1. API Key authentication (preferred = 1)
2. Username/Password (preferred = 2)
3. Certificate-based (preferred = 3)

## Troubleshooting

### Application won't start
- Check `config.yaml` is valid YAML
- Ensure required directories exist: `make setup`
- Check logs in `logs/application.log`

### Clusters not loading
- Verify CSV file format and location
- Check that `csv_fileName` in `configs/jobs.yaml` is correct
- Review `logs/job.log` for errors

### Cannot connect to clusters
- Verify endpoints in CSV are accessible
- Check authentication credentials
- Ensure `insecureTLS` is set correctly for self-signed certificates
- Review `logs/job.log` for connection errors

### No indexing rate data
- Ensure clusters are loaded successfully
- Check that `fetch_indices` job is running (view with `/api/jobs`)
- Verify the cluster has active endpoints (`/api/status`)
- Wait for at least 2 job cycles (6+ minutes with default 3m interval)

## Next Steps

1. **Add your clusters** to `data/clusters.csv`
2. **Configure credentials** for accessing your Elasticsearch clusters
3. **Adjust job intervals** in `configs/jobs.yaml` based on your needs
4. **Build a React frontend** to visualize the indexing rates and cluster health
5. **Set up Prometheus** to scrape metrics from port 9091
6. **Create dashboards** in Grafana using the Prometheus metrics

## Support

For issues or questions, refer to the main README.md file or check the application logs.
