# ElasticObservability API Reference

Complete reference for all REST API endpoints provided by the ElasticObservability service.

## Base Information

**Default Base URL:** `http://localhost:9092/api`  
**Content-Type:** `application/json`  
**Authentication:** None (configure TLS certificates in config.yaml for secure deployments)

---

## Cluster Management

### List All Clusters
Get a list of all managed Elasticsearch clusters.

**Endpoint:** `GET /api/clusters`

**Response:**
```json
{
  "clusters": ["prod-cluster-01", "dev-cluster-01", "uat-cluster-01"],
  "count": 3
}
```

**Status Codes:**
- `200 OK` - Success

---

### Get Cluster Nodes
Get detailed information about all nodes in a specific cluster.

**Endpoint:** `GET /api/clusters/{clusterName}/nodes`

**Parameters:**
- `clusterName` (path) - Name of the cluster

**Response:**
```json
{
  "cluster": "prod-cluster-01",
  "nodes": [
    {
      "hostName": "es-node-01.example.com",
      "ipAddress": "10.0.1.10",
      "port": "9200",
      "type": ["master", "data"],
      "zone": "us-east-1a",
      "nodeTier": "hot",
      "dataCenter": "dc1"
    }
  ],
  "count": 1
}
```

**Status Codes:**
- `200 OK` - Success
- `400 Bad Request` - Invalid cluster name format
- `404 Not Found` - Cluster not found

---

## Indexing Rate

### Get Indexing Rate for Cluster
Retrieve indexing rate metrics for all indices in a cluster.

**Endpoint:** `GET /api/indexingRate/{clusterName}`

**Parameters:**
- `clusterName` (path) - Name of the cluster

**Response:**
```json
{
  "cluster": "prod-cluster-01",
  "timestamp": 1704567890000,
  "indices": {
    "logs-app": {
      "fromCreation": 0.125,
      "last3Minutes": 0.150,
      "last15Minutes": 0.140,
      "last60Minutes": 0.130,
      "numberOfShards": 5
    },
    "metrics-system": {
      "fromCreation": 0.080,
      "last3Minutes": 0.090,
      "last15Minutes": 0.085,
      "last60Minutes": 0.082,
      "numberOfShards": 3
    }
  }
}
```

**Metrics Explanation:**
- All rates are in bytes/millisecond per primary shard
- `fromCreation` - Average rate since index creation
- `last3Minutes` - Average rate over last 3 minutes
- `last15Minutes` - Average rate over last 15 minutes
- `last60Minutes` - Average rate over last 60 minutes
- Value of `-1` indicates insufficient data

**Status Codes:**
- `200 OK` - Success
- `400 Bad Request` - Invalid cluster name format
- `404 Not Found` - Cluster not found or indexing rate data not available

---

## Stale Indices

### Get Stale Indices
Identify indices that have not been modified (no new documents) in n days.

**Endpoint:** `GET /api/staleIndices/{clusterName}/{days}`

**Parameters:**
- `clusterName` (path) - Name of the cluster
- `days` (path) - Number of days to check (must be >= 1)

**Response:**
```json
{
  "cluster": "prod-cluster-01",
  "daysChecked": 7,
  "totalIndices": 150,
  "staleIndices": [
    {
      "indexName": "old-logs-2024",
      "docCount": 1000000,
      "currentSize": 5368709120,
      "currentTimestamp": 1704567890000,
      "oldSize": 5368709120,
      "oldTimestamp": 1704000000000,
      "daysStale": 7,
      "sizeChange": 0
    }
  ],
  "staleCount": 1,
  "insufficientDataCount": 5,
  "lastUpdateTime": 1704567890000
}
```

**Status Codes:**
- `200 OK` - Success
- `400 Bad Request` - Invalid cluster name or days parameter
- `404 Not Found` - Cluster not found or statistics not available

---

## Thread Pool Write Queue

### Get TPWQueue for Cluster
Retrieve thread pool write queue metrics for all hosts in a cluster.

**Endpoint:** `GET /api/tpwqueue/{clusterName}`

**Parameters:**
- `clusterName` (path) - Name of the cluster

**Response:**
```json
{
  "cluster": "prod-cluster-01",
  "hostnames": ["host1.example.com", "host2.example.com"],
  "hostCount": 2,
  "hosts": {
    "host1.example.com": {
      "numberOfDataPoints": 120,
      "dataPointCount": 85,
      "dataPoints": [
        {
          "timestamp": 1704567890000,
          "queue": 5,
          "index": 0
        },
        {
          "timestamp": 1704567860000,
          "queue": 3,
          "index": 1
        }
      ]
    },
    "host2.example.com": {
      "numberOfDataPoints": 120,
      "dataPointCount": 82,
      "dataPoints": [...]
    }
  }
}
```

**Notes:**
- Only returns data points where `dataExists` is true
- Data points ordered by index (0 = latest, higher = older)
- Queue depth is number of pending write operations

**Status Codes:**
- `200 OK` - Success
- `400 Bad Request` - Invalid cluster name format
- `404 Not Found` - Cluster not found or TPWQueue data not available

---

### Get TPWQueue for Host
Retrieve detailed thread pool write queue metrics for a specific host, including missing data points.

**Endpoint:** `GET /api/tpwqueue/{clusterName}/{hostName}`

**Parameters:**
- `clusterName` (path) - Name of the cluster
- `hostName` (path) - Hostname of the node

**Response:**
```json
{
  "cluster": "prod-cluster-01",
  "hostName": "host1.example.com",
  "numberOfDataPoints": 120,
  "existingCount": 85,
  "missingCount": 35,
  "dataPoints": [
    {
      "index": 0,
      "dataExists": true,
      "timestamp": 1704567890000,
      "queue": 5
    },
    {
      "index": 1,
      "dataExists": true,
      "timestamp": 1704567860000,
      "queue": 3
    },
    {
      "index": 2,
      "dataExists": false,
      "timestamp": null,
      "queue": null
    }
  ]
}
```

**Notes:**
- Returns ALL data points (existing and missing)
- Missing data points have `dataExists: false` and null values
- Useful for identifying gaps in data collection

**Status Codes:**
- `200 OK` - Success
- `400 Bad Request` - Invalid cluster name or host name
- `404 Not Found` - Cluster not found, host not found, or TPWQueue data not available

---

## Application Status

### Get Application Status
Retrieve current application health and statistics.

**Endpoint:** `GET /api/status`

**Response:**
```json
{
  "status": "running",
  "clusters": 15,
  "ratesTracked": 12,
  "timestamp": 1704567890000
}
```

**Status Codes:**
- `200 OK` - Success

---

### Get Job Status
Retrieve status and statistics for all scheduled jobs.

**Endpoint:** `GET /api/jobs`

**Response:**
```json
{
  "jobs": [
    {
      "name": "fetch_indices",
      "type": "preDefined",
      "enabled": true,
      "lastRun": 1704567890000,
      "lastStatus": "success",
      "nextRun": 1704568070000,
      "runCount": 1250
    },
    {
      "name": "analyze_rates",
      "type": "preDefined",
      "enabled": true,
      "lastRun": 1704567895000,
      "lastStatus": "success",
      "nextRun": null,
      "runCount": 1250
    }
  ]
}
```

**Status Codes:**
- `200 OK` - Success

---

## Job Control

### Trigger Job Manually
Manually trigger execution of a scheduled job.

**Endpoint:** `POST /api/jobs/{jobName}/trigger`

**Parameters:**
- `jobName` (path) - Name of the job to trigger

**Response:**
```json
{
  "message": "Job fetch_indices triggered successfully"
}
```

**Status Codes:**
- `200 OK` - Job triggered successfully
- `404 Not Found` - Job not found

---

## Prometheus Metrics

### Get Prometheus Metrics
Export application and job metrics in Prometheus format.

**Endpoint:** `GET /metrics`  
**Port:** Configured by `metricsPort` (default: 9091)

**Response Format:** Prometheus text-based exposition format

**Example Response:**
```
# HELP elasticobservability_job_runs_total Total number of job runs
# TYPE elasticobservability_job_runs_total counter
elasticobservability_job_runs_total{job="fetch_indices"} 1250

# HELP elasticobservability_job_duration_seconds Job execution duration
# TYPE elasticobservability_job_duration_seconds histogram
elasticobservability_job_duration_seconds_bucket{job="fetch_indices",le="1"} 1200
elasticobservability_job_duration_seconds_bucket{job="fetch_indices",le="5"} 1250
```

**Status Codes:**
- `200 OK` - Success

---

## Error Responses

All error responses follow this format:

```json
{
  "error": "Error message describing what went wrong"
}
```

**Common Status Codes:**
- `400 Bad Request` - Invalid input parameters
- `404 Not Found` - Resource not found
- `500 Internal Server Error` - Server-side error

---

## Rate Limiting

Currently, no rate limiting is implemented. For production deployments, consider placing the service behind a reverse proxy with rate limiting capabilities.

---

## Examples

### Using cURL

```bash
# Get all clusters
curl http://localhost:9092/api/clusters

# Get indexing rate for a cluster
curl http://localhost:9092/api/indexingRate/prod-cluster-01

# Find stale indices (not modified in 30 days)
curl http://localhost:9092/api/staleIndices/prod-cluster-01/30

# Get TPWQueue for cluster
curl http://localhost:9092/api/tpwqueue/prod-cluster-01

# Trigger a job
curl -X POST http://localhost:9092/api/jobs/fetch_indices/trigger

# Get Prometheus metrics
curl http://localhost:9091/metrics
```

### Using JavaScript/Fetch

```javascript
// Get clusters
const clusters = await fetch('http://localhost:9092/api/clusters')
  .then(res => res.json());

// Get indexing rate
const rates = await fetch(`http://localhost:9092/api/indexingRate/${clusterName}`)
  .then(res => res.json());

// Get stale indices
const staleIndices = await fetch(`http://localhost:9092/api/staleIndices/${clusterName}/30`)
  .then(res => res.json());

// Trigger job
await fetch(`http://localhost:9092/api/jobs/${jobName}/trigger`, {
  method: 'POST'
});
```

### Using Python/requests

```python
import requests

base_url = 'http://localhost:9092/api'

# Get clusters
clusters = requests.get(f'{base_url}/clusters').json()

# Get indexing rate
rates = requests.get(f'{base_url}/indexingRate/prod-cluster-01').json()

# Get stale indices
stale = requests.get(f'{base_url}/staleIndices/prod-cluster-01/30').json()

# Trigger job
response = requests.post(f'{base_url}/jobs/fetch_indices/trigger')
```

---

## Thread Safety

All API endpoints are thread-safe:
- Use RWMutex locks for shared data access
- Acquire locks only during data copying
- Process data after releasing locks
- Multiple concurrent requests are supported

---

## Best Practices

1. **Polling Intervals**: Don't poll faster than job execution intervals
2. **Error Handling**: Always check status codes and handle errors appropriately
3. **Data Freshness**: Check timestamps to ensure data currency
4. **Caching**: Consider caching responses for frequently accessed endpoints
5. **Monitoring**: Use the `/api/status` and `/api/jobs` endpoints for health checks

---

## Support

For issues, feature requests, or questions:
- Review application logs in `./logs/application.log`
- Review job logs in `./logs/job.log`
- Check Prometheus metrics for performance insights
- Refer to main documentation in `README.md`
