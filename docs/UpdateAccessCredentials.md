# UpdateAccessCredentials Job

## Overview

The `updateAccessCredentials` job updates access credentials for Elasticsearch clusters from a CSV file. This allows you to manage authentication details separately from cluster configuration.

## Purpose

- Update authentication credentials for existing clusters
- Support multiple authentication methods (API Key, Username/Password, Certificate)
- Allow credentials to be managed independently from cluster topology
- Enable secure credential updates without modifying cluster definitions

## CSV File Format

### Required Columns

| Column Name | Type | Description |
|------------|------|-------------|
| ClusterName | string | Name of the cluster (must already exist) |
| PrefferedAccess | uint8 | Preferred authentication method (1, 2, or 3) |
| APIKey | string | API key for authentication (optional) |
| UserID | string | Username for basic auth (optional) |
| Password | string | Password for basic auth (optional) |
| ClientCert | string | Path to client certificate file (optional) |
| ClientKey | string | Path to client key file (optional) |
| Cacert | string | Path to CA certificate file (optional) |
| ClusterPort | string | Port for cluster connections (optional, default: 9200) |
| ApplicationLBs | string | Comma-separated load balancer URLs for ClusterSAN (optional) |

### PrefferedAccess Values

- `1` - API Key authentication (preferred)
- `2` - Username/Password authentication
- `3` - Certificate-based authentication

### Sample CSV

```csv
ClusterName,PrefferedAccess,APIKey,UserID,Password,ClientCert,ClientKey,Cacert,ClusterPort,ApplicationLBs
prod-cluster-01,1,my-api-key-for-prod,,,,,9200,https://prod-lb1.example.com,https://prod-lb2.example.com
dev-cluster-01,2,,elastic,dev-password123,,,9200,https://dev-lb.example.com
uat-cluster-01,3,,,,/path/to/client.crt,/path/to/client.key,/path/to/ca.crt,9243,https://uat-lb1.example.com,https://uat-lb2.example.com
staging-cluster,1,staging-key-xyz,admin,backup-password,,9200,
```

**Note**: `ApplicationLBs` is comma-separated list of load balancer URLs. Empty fields will not update existing values.

## Job Configuration

### As Initialization Job

```yaml
jobs:
  - name: update_credentials
    type: preDefined
    internalJobName: updateAccessCredentials
    enabled: true
    initJob: true
    dependsOn: ["load_clusters"]
    parameters:
      csv_fileName: ./data/credentials.csv
```

### As One-Time Job

Place in `configs/oneTime/update_creds.yaml`:

```yaml
jobs:
  - name: update_credentials_onetime
    type: preDefined
    internalJobName: updateAccessCredentials
    enabled: true
    parameters:
      csv_fileName: ./data/credentials.csv
```

### As Scheduled Job

```yaml
jobs:
  - name: periodic_credential_update
    type: preDefined
    internalJobName: updateAccessCredentials
    enabled: true
    schedule:
      interval: 1h
      initialWait: 5m
    parameters:
      csv_fileName: ./data/credentials.csv
```

## Execution Flow

1. **Parse CSV File** - Reads and validates the CSV file
2. **For Each Row:**
   - Extract cluster name
   - Check if cluster exists in `AllClusters`
   - Update `AccessCred` fields with provided values
   - Skip empty fields (existing values preserved)
3. **Log Results** - Reports updated, not found, and skipped counts

## Important Notes

### Dependency Order

The job **must run after** `loadFromMasterCSV` and **before** `updateActiveEndpoint`:

```yaml
load_clusters → update_credentials → update_endpoints
```

This is because `updateActiveEndpoint` needs credentials to test cluster connectivity.

### Field Updates

- Only non-empty fields are updated
- Empty fields preserve existing values
- Clusters not found in the CSV are not modified
- Clusters that don't exist are skipped with a warning

### Security Considerations

1. **File Permissions** - Restrict access to the credentials CSV file
   ```bash
   chmod 600 data/credentials.csv
   ```

2. **Encrypted Storage** - Store sensitive CSV files in encrypted volumes

3. **Environment Variables** - Consider using environment variables for highly sensitive data

4. **Audit Logging** - All credential updates are logged in `logs/job.log`

5. **Rotation** - Regularly update credentials and archive old CSV files

## Usage Examples

### Example 1: Initial Setup

1. Create credentials CSV:
```csv
ClusterName,PrefferedAccess,APIKey,UserID,Password,ClientCert,ClientKey,Cacert
prod-cluster-01,1,prod-api-key-abc123,,,,,
```

2. Configure job in `configs/jobs.yaml`:
```yaml
- name: update_credentials
  type: preDefined
  internalJobName: updateAccessCredentials
  enabled: true
  initJob: true
  dependsOn: ["load_clusters"]
  parameters:
    csv_fileName: ./data/credentials.csv
```

3. Start application - credentials are loaded on initialization

### Example 2: Update Existing Credentials

1. Create one-time job file `configs/oneTime/update_prod_creds.yaml`:
```yaml
jobs:
  - name: update_prod_credentials
    type: preDefined
    internalJobName: updateAccessCredentials
    enabled: true
    parameters:
      csv_fileName: ./data/new_credentials.csv
```

2. File is automatically processed and moved to `processedOneTime/`

### Example 3: Update Single Cluster

CSV with only one cluster:
```csv
ClusterName,PrefferedAccess,APIKey,UserID,Password,ClientCert,ClientKey,Cacert
prod-cluster-01,1,new-rotated-api-key,,,,,
```

Only `prod-cluster-01` credentials are updated, others remain unchanged.

### Example 4: Mixed Authentication

Different authentication methods for different clusters:
```csv
ClusterName,PrefferedAccess,APIKey,UserID,Password,ClientCert,ClientKey,Cacert
prod-api-cluster,1,prod-api-key,,,,,
dev-basic-cluster,2,,devuser,devpass123,,,
secure-cert-cluster,3,,,,/certs/client.crt,/certs/client.key,/certs/ca.crt
```

## Error Handling

### Common Issues

**Issue**: Cluster not found
```
Row 2: Cluster 'unknown-cluster' not found, skipping
```
**Solution**: Ensure cluster name matches exactly (case-sensitive)

**Issue**: CSV parse error
```
Failed to parse CSV: record on line 3: wrong number of fields
```
**Solution**: Verify CSV format, check for missing commas

**Issue**: Invalid PrefferedAccess value
```
Invalid value for PrefferedAccess: 'five'
```
**Solution**: Use only 1, 2, or 3

## Monitoring

### Job Logs

Check execution in `logs/job.log`:
```
[2026-01-02 16:30:00] [INFO] [updateAccessCredentials] Starting credentials update job
[2026-01-02 16:30:00] [INFO] [updateAccessCredentials] Parsed 3 rows from CSV
[2026-01-02 16:30:00] [INFO] [updateAccessCredentials] Row 1: Updated credentials for cluster: prod-cluster-01
[2026-01-02 16:30:00] [INFO] [updateAccessCredentials] Completed: 3 clusters updated, 0 not found, 0 skipped
```

### API Verification

After updating credentials, verify via API:
```bash
# Check cluster list
curl http://localhost:9092/api/clusters

# Test connectivity (updateActiveEndpoint will use new credentials)
curl http://localhost:9092/api/jobs/update_endpoints/trigger -X POST
```

## Best Practices

1. **Version Control** - Keep credentials CSV out of git (add to `.gitignore`)
2. **Backup** - Backup credentials before updates
3. **Testing** - Test credential updates in dev environment first
4. **Validation** - Verify endpoint connectivity after credential updates
5. **Documentation** - Document which clusters use which auth methods
6. **Rotation Schedule** - Establish regular credential rotation policy
7. **Monitoring** - Monitor job logs for failures or warnings

## Related Jobs

- **loadFromMasterCSV** - Must run before this job (loads cluster data)
- **updateActiveEndpoint** - Should run after this job (needs credentials to test connectivity)
- **runCatIndices** - Uses credentials to fetch cluster data

## API Integration

Manually trigger credential update:
```bash
curl -X POST http://localhost:9092/api/jobs/update_credentials/trigger
```

Check job status:
```bash
curl http://localhost:9092/api/jobs
```

## Troubleshooting

### Credentials Not Working

1. Check job completed successfully in logs
2. Verify CSV format and cluster names
3. Test manual API call with credentials
4. Check `PrefferedAccess` matches available auth method
5. Verify file paths for certificates are correct

### Job Not Running

1. Ensure `enabled: true` in configuration
2. Check dependencies are satisfied
3. Verify CSV file exists and is readable
4. Check application logs for errors

## See Also

- [Main README](../README.md)
- [Quick Start Guide](../QUICKSTART.md)
- [Job Configuration](../configs/jobs.yaml)
- [Sample Credentials CSV](../data/credentials.csv)
