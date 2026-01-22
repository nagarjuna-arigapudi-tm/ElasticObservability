# ElasticObservability - Global Data Structures

This document provides a visual representation of how global data structures are related in the ElasticObservability project.

## Overview

The application maintains four main global data structures:
1. **AllClusters** - Cluster information and topology
2. **AllClustersList** - Ordered list of cluster names
3. **AllHistory** - Historical indices data per cluster
4. **AllIndexingRate** - Calculated indexing rates per cluster

---

## 1. Cluster Information Structure

```
┌─────────────────────────────────────────────────────────────────┐
│                       GLOBAL DATA STRUCTURES                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  AllClusters: map[string]*ClusterData                          │
│  ├─ Key: "prod-cluster-01" ───────────┐                        │
│  │                                     │                        │
│  ├─ Key: "dev-cluster-01" ────────┐   │                        │
│  │                                │   │                        │
│  └─ Key: "uat-cluster-01" ──┐     │   │                        │
│                             │     │   │                        │
│  AllClustersList: []string  │     │   │                        │
│  ├─ [0]: "prod-cluster-01" ─┼─────┼───┘                        │
│  ├─ [1]: "dev-cluster-01" ──┼─────┘                            │
│  └─ [2]: "uat-cluster-01" ──┘                                  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### ClusterData Structure

```
┌──────────────────────────────────────────────────────────────────┐
│                          ClusterData                             │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ClusterName:     "prod-cluster-01"                             │
│  ClusterUUID:     "abc-123-xyz"                                 │
│  CurrentEndpoint: "https://es.example.com:9200"                 │
│  InsecureTLS:     true                                          │
│  Active:          true                                          │
│  ZoneIdentifier:  "us-east-1a"                                  │
│  ClusterSAN:      ["https://lb1.com", "https://lb2.com"]       │
│  ActiveEndpoint:  "https://lb1.com"                             │
│  KibanaSAN:       ["https://kb1.com:5601"]                      │
│  Owner:           "Platform Team"                               │
│  Env:             "prd"                                         │
│  ClusterPort:     "9200"                                        │
│  KibanaPort:      "5601"                                        │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ AccessCred:                                             │   │
│  │   Preferred:  1 (1=APIKey, 2=UserPass, 3=Cert)         │   │
│  │   APIKey:     "xyzabc123..."                            │   │
│  │   UserID:     ""                                        │   │
│  │   Password:   ""                                        │   │
│  │   ClientCert: ""                                        │   │
│  │   ClientKey:  ""                                        │   │
│  │   CaCert:     ""                                        │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Nodes: []*Node                                          │   │
│  │                                                         │   │
│  │   Node[0]: ──────────────────────────────┐             │   │
│  │   │  HostName:     "es-node-01"          │             │   │
│  │   │  IPAddress:    "10.0.1.10"           │             │   │
│  │   │  Port:         "9200"                │             │   │
│  │   │  Type:         ["master", "data"]    │             │   │
│  │   │  Zone:         "us-east-1a"          │             │   │
│  │   │  DataCenter:   "dc1"                 │             │   │
│  │   │  NodeTier:     "hot"                 │             │   │
│  │   └──────────────────────────────────────┘             │   │
│  │                                                         │   │
│  │   Node[1]: ──────────────────────────────┐             │   │
│  │   │  HostName:     "es-node-02"          │             │   │
│  │   │  IPAddress:    "10.0.1.11"           │             │   │
│  │   │  Port:         "9200"                │             │   │
│  │   │  Type:         ["data"]              │             │   │
│  │   │  Zone:         "us-east-1b"          │             │   │
│  │   │  DataCenter:   "dc1"                 │             │   │
│  │   │  NodeTier:     "warm"                │             │   │
│  │   └──────────────────────────────────────┘             │   │
│  │                                                         │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

---

## 2. Indices History Structure

```
┌────────────────────────────────────────────────────────────────────┐
│                  AllHistory: map[string]*IndicesHistory            │
├────────────────────────────────────────────────────────────────────┤
│                                                                    │
│  Key: "prod-cluster-01" ──────┐                                   │
│                               │                                   │
│  Key: "dev-cluster-01" ───┐   │                                   │
│                           │   │                                   │
│  Key: "uat-cluster-01" ───│───│───┐                               │
│                           │   │   │                               │
│                           ▼   ▼   ▼                               │
│                    ┌──────────────────────────────┐               │
│                    │   IndicesHistory             │               │
│                    │                              │               │
│                    │   SizeOfPtr: 20              │               │
│                    │   (from config)              │               │
│                    │                              │               │
│                    │   Ptr: []*IndicesSnapShot    │               │
│                    │   ├─ [0]  ────────┐          │               │
│                    │   ├─ [1]  ────┐   │          │               │
│                    │   ├─ [2]  ─┐  │   │          │               │
│                    │   │  ...   │  │   │          │               │
│                    │   └─ [20]  │  │   │          │               │
│                    │      ▲     │  │   │          │               │
│                    │      │     │  │   │          │               │
│                    │   Latest  │  │   │          │               │
│                    │            │  │   │          │               │
│                    └────────────│──│───│──────────┘               │
│                                 │  │   │                          │
│                                 │  │   │                          │
│     ┌───────────────────────────┘  │   │                          │
│     │  ┌────────────────────────────┘   │                          │
│     │  │  ┌──────────────────────────────┘                          │
│     ▼  ▼  ▼                                                       │
│  ┌────────────────────────────────────────────────┐               │
│  │         IndicesSnapShot                        │               │
│  │                                                │               │
│  │  SnapShotTime: 1704567890000 (epoch ms)       │               │
│  │                                                │               │
│  │  MapIndices: map[string]*IndexInfo             │               │
│  │  ├─ Key: "logs-app" ──────────┐               │               │
│  │  │                             │               │               │
│  │  ├─ Key: "metrics-system" ──┐ │               │               │
│  │  │                          │ │               │               │
│  │  └─ Key: "traces-api" ─┐    │ │               │               │
│  │                        │    │ │               │               │
│  └────────────────────────│────│─│───────────────┘               │
│                           │    │ │                               │
│                           ▼    ▼ ▼                               │
│                  ┌─────────────────────────────┐                 │
│                  │      IndexInfo              │                 │
│                  │                             │                 │
│                  │  Health:         1 (green)  │                 │
│                  │  IsOpen:         true       │                 │
│                  │  DocCount:       1000000    │                 │
│                  │  Index:          "logs-..01"│                 │
│                  │  IndexBase:      "logs-app" │                 │
│                  │  SeqNo:          1          │                 │
│                  │  PrimaryShards:  5          │                 │
│                  │  CreationTime:   1704560000 │                 │
│                  │  TotalStorage:   5368709120 │                 │
│                  │  PrimaryStorage: 2684354560 │                 │
│                  │                             │                 │
│                  └─────────────────────────────┘                 │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

### History Snapshot Roll-Over Mechanism

```
When a new snapshot is taken:

   Old State:                       New State:
   ┌──────────┐                    ┌──────────┐
   │ Ptr[0]   │ ← Oldest           │ Ptr[0]   │ ← Was Ptr[1]
   │ Ptr[1]   │                    │ Ptr[1]   │ ← Was Ptr[2]
   │ Ptr[2]   │                    │ Ptr[2]   │ ← Was Ptr[3]
   │   ...    │                    │   ...    │
   │ Ptr[19]  │                    │ Ptr[19]  │ ← Was Ptr[20]
   │ Ptr[20]  │ ← Latest           │ Ptr[20]  │ ← New snapshot
   └──────────┘                    └──────────┘
                                        ▲
                                        │
                                   New snapshot added
                                   (oldest deleted)
```

---

## 3. Indexing Rate Structure

```
┌──────────────────────────────────────────────────────────────────────┐
│            AllIndexingRate: map[string]*ClusterIndexingRate          │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Key: "prod-cluster-01" ─────────┐                                  │
│                                  │                                  │
│  Key: "dev-cluster-01" ──────┐   │                                  │
│                              │   │                                  │
│  Key: "uat-cluster-01" ───┐  │   │                                  │
│                           │  │   │                                  │
│                           ▼  ▼   ▼                                  │
│                  ┌──────────────────────────────────┐               │
│                  │   ClusterIndexingRate            │               │
│                  │                                  │               │
│                  │   Timestamp: 1704567890000       │               │
│                  │              (epoch ms)          │               │
│                  │                                  │               │
│                  │   MapIndices:                    │               │
│                  │   map[string]*IndexingRate       │               │
│                  │                                  │               │
│                  │   ├─ "logs-app" ────────┐       │               │
│                  │   │                      │       │               │
│                  │   ├─ "metrics-system" ─┐│       │               │
│                  │   │                     ││       │               │
│                  │   └─ "traces-api" ─┐    ││       │               │
│                  │                    │    ││       │               │
│                  └────────────────────│────││───────┘               │
│                                       │    ││                       │
│                                       ▼    ▼▼                       │
│                          ┌────────────────────────────┐             │
│                          │     IndexingRate           │             │
│                          │                            │             │
│                          │  FromCreation:   125.5     │             │
│                          │  Last3Minutes:   150.2     │             │
│                          │  Last15Minutes:  145.8     │             │
│                          │  Last60Minutes:  140.3     │             │
│                          │  NumberOfShards: 5         │             │
│                          │                            │             │
│                          │  (All rates in bytes/ms    │             │
│                          │   per shard)               │             │
│                          │                            │             │
│                          └────────────────────────────┘             │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 4. Complete Relationship Diagram

```
┌──────────────────────────────────────────────────────────────────────────┐
│                        APPLICATION STARTUP FLOW                          │
└──────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────┐
│  INITIALIZATION JOB: LoadFromMasterCSV                                   │
│  ├─ Reads: data/clusters.csv                                             │
│  ├─ Reads: data/credentials.csv (via updateAccessCredentials)            │
│  └─ Populates:                                                           │
│     ├─ AllClusters map[string]*ClusterData ◄────────────┐               │
│     └─ AllClustersList []string                         │               │
└──────────────────────────────────────────────────────────│───────────────┘
                                    │                      │
                                    ▼                      │
┌──────────────────────────────────────────────────────────│───────────────┐
│  SCHEDULED JOB: RunCatIndices (every 3 minutes)         │               │
│  ├─ For each cluster in AllClusters ────────────────────┘               │
│  ├─ Makes API call: GET /_cat/indices                                   │
│  ├─ Creates: IndicesSnapShot                                            │
│  └─ Updates: AllHistory map[clusterName]*IndicesHistory                 │
│     ├─ Stores snapshot in: IndicesHistory.Ptr[20]                       │
│     └─ Rolls over old snapshots (Ptr[0] = Ptr[1], ... Ptr[20] = new)    │
└──────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────┐
│  DEPENDENT JOB: AnalyseIngest (after RunCatIndices)                     │
│  ├─ For each cluster in AllHistory                                      │
│  ├─ Reads: Multiple snapshots from IndicesHistory.Ptr[]                 │
│  │   ├─ Ptr[20] = Latest (t_0)                                          │
│  │   ├─ Ptr[19] = 3 min ago (t_1)                                       │
│  │   ├─ Ptr[15] = 15 min ago (t_5)                                      │
│  │   └─ Ptr[0]  = 60 min ago (t_20)                                     │
│  ├─ Calculates: Indexing rates from storage deltas                      │
│  └─ Updates: AllIndexingRate map[clusterName]*ClusterIndexingRate       │
└──────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────┐
│  API ENDPOINTS                                                           │
│  ├─ GET /api/clusters                                                    │
│  │   └─ Returns: AllClustersList []string                               │
│  │                                                                        │
│  ├─ GET /api/clusters/{clusterName}/nodes                                │
│  │   └─ Returns: AllClusters[clusterName].Nodes                         │
│  │                                                                        │
│  ├─ GET /api/indexingRate/{clusterName}                                  │
│  │   └─ Returns: AllIndexingRate[clusterName]                           │
│  │                                                                        │
│  └─ GET /api/status                                                      │
│      └─ Returns: Summary of all data structures                         │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## 5. Data Flow Timeline

```
Time ──────────────────────────────────────────────────────────────►

T=0     │ Application Start
        │ └─ LoadFromMasterCSV
        │    ├─ AllClusters populated
        │    └─ AllClustersList populated
        │
T=1m    │ updateActiveEndpoint
        │ └─ Tests connectivity, updates ActiveEndpoint
        │
T=2m    │ RunCatIndices (1st run)
        │ └─ AllHistory[cluster].Ptr[20] = snapshot_1
        │
T=2m    │ AnalyseIngest (1st run)
        │ └─ Only "fromCreation" calculated (no history yet)
        │
T=5m    │ RunCatIndices (2nd run)
        │ ├─ Roll over: Ptr[20] = Ptr[19]
        │ └─ AllHistory[cluster].Ptr[20] = snapshot_2
        │
T=5m    │ AnalyseIngest (2nd run)
        │ ├─ "fromCreation" calculated
        │ └─ "last3Minutes" calculated (using Ptr[20] and Ptr[19])
        │
T=8m    │ RunCatIndices (3rd run)
        │ └─ AllHistory[cluster].Ptr[20] = snapshot_3
        │
        │ ... (continues every 3 minutes)
        │
T=60m   │ RunCatIndices (20th run)
        │ └─ Now have full 60-minute history
        │
T=60m   │ AnalyseIngest (20th run)
        │ ├─ "fromCreation" calculated
        │ ├─ "last3Minutes" calculated
        │ ├─ "last15Minutes" calculated
        │ └─ "last60Minutes" calculated ◄── Now have all metrics!
```

---

## 6. Thread Safety

All global data structures use mutexes for thread-safe access:

```
┌────────────────────────────────────────────────┐
│  Global Data Structure     │  Mutex            │
├────────────────────────────┼───────────────────┤
│  AllClusters               │  ClustersMu       │
│  AllClustersList           │  ClustersMu       │
│  AllHistory                │  HistoryMu        │
│  AllIndexingRate           │  IndexingRateMu   │
└────────────────────────────────────────────────┘

Usage Pattern:

    // Write operation
    types.ClustersMu.Lock()
    types.AllClusters[name] = cluster
    types.ClustersMu.Unlock()

    // Read operation
    types.ClustersMu.RLock()
    cluster := types.AllClusters[name]
    types.ClustersMu.RUnlock()

    // IndicesHistory has its own internal mutex
    history.AddSnapshot(snapshot)  // Thread-safe internally
    copy := history.GetCopy()      // Thread-safe copy
```

---

## 7. Memory Management

```
Configuration:
    historyForIndices: 20 (from config.yaml)

Memory per Cluster:
    IndicesHistory.Ptr = make([]*IndicesSnapShot, 21)
                          └─ Size = historyForIndices + 1

Example with 3 clusters, 20 history points, 100 indices each:

    AllClusters:        3 clusters × ~1KB        ≈ 3 KB
    AllHistory:         3 × 21 × 100 indices     ≈ 630 KB
    AllIndexingRate:    3 × 100 indices          ≈ 9 KB
                                          Total   ≈ 642 KB

Scales linearly with:
    - Number of clusters
    - History size (historyForIndices)
    - Number of indices per cluster
```

---

## 5. Bulk Write Tasks Monitoring Structure

```
┌────────────────────────────────────────────────────────────────────────────┐
│  AllClusterDataWriteBulk_sTasksHistory: map[string]*ClusterDataWriteBulk   │
│                                          _sTasksHistory                     │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  Key: "prod-cluster-01" ──────────┐                                       │
│                                   │                                       │
│  Key: "dev-cluster-01" ───────┐   │                                       │
│                               │   │                                       │
│  Key: "uat-cluster-01" ────┐  │   │                                       │
│                            │  │   │                                       │
│                            ▼  ▼   ▼                                       │
│              ┌──────────────────────────────────────────┐                 │
│              │  ClusterDataWriteBulk_sTasksHistory      │                 │
│              │                                          │                 │
│              │  LatestSnapShotTime: 1704567890 (epoch)  │                 │
│              │  HistorySize:        60 (configurable)   │                 │
│              │  ClusterName:        "prod-cluster-01"   │                 │
│              │                                          │                 │
│              │  PtrClusterDataWriteBulk_sTasks:         │                 │
│              │  []*ClusterDataWriteBulk_sTasks          │                 │
│              │  ├─ [0]  (Latest) ────────┐              │                 │
│              │  ├─ [1]  (1 min ago) ──┐  │              │                 │
│              │  ├─ [2]  (2 min ago) ┐ │  │              │                 │
│              │  │  ...               │ │  │              │                 │
│              │  └─ [59] (59 min ago)│ │  │              │                 │
│              │                       │ │  │              │                 │
│              └───────────────────────│─│──│──────────────┘                 │
│                                      │ │  │                               │
│                                      │ │  │                               │
│     ┌────────────────────────────────┘ │  │                               │
│     │  ┌─────────────────────────────────┘  │                               │
│     │  │  ┌──────────────────────────────────┘                               │
│     ▼  ▼  ▼                                                               │
│  ┌───────────────────────────────────────────────────────────┐            │
│  │        ClusterDataWriteBulk_sTasks (Snapshot)             │            │
│  │                                                           │            │
│  │  SnapShotTime: 1704567890 (epoch seconds)                │            │
│  │                                                           │            │
│  │  DataWriteBulk_sTasksByNode:                             │            │
│  │  map[string]*NodeDataWriteBulk_sTasks                    │            │
│  │  ├─ "host1.example.com" ───────┐                         │            │
│  │  ├─ "host2.example.com" ────┐  │                         │            │
│  │  └─ "host3.example.com" ─┐  │  │                         │            │
│  │                          │  │  │                         │            │
│  │  SortedHostsOnTasks:     ["host1", "host2", "host3"]    │            │
│  │  SortedHostsOnTimetaken: ["host2", "host1", "host3"]    │            │
│  │  SortedHostsOnRequest:   ["host1", "host3", "host2"]    │            │
│  │                          │  │  │                         │            │
│  │  DataWriteBulk_sTasksByIndex:                            │            │
│  │  map[string]*AggShardTaskDataWriteBulk_s                 │            │
│  │  ├─ "logs-app" ──────────────────┐                       │            │
│  │  ├─ "metrics-system" ─────────┐  │                       │            │
│  │  └─ "traces-api" ──────────┐  │  │                       │            │
│  │                            │  │  │                       │            │
│  │  IndicesSortedonTasks:     ["logs", "metrics", ...]     │            │
│  │  IndicesSortedOnRequests:  ["logs", "traces", ...]      │            │
│  │  IndicesSortedOnTimetaken: ["metrics", "logs", ...]     │            │
│  │                            │  │  │                       │            │
│  └────────────────────────────│──│──│───────────────────────┘            │
│                               │  │  │                                    │
│                               ▼  ▼  ▼                                    │
│              ┌───────────────────────────────────────┐                   │
│              │  AggShardTaskDataWriteBulk_s          │                   │
│              │  (Used for both Index & Shard data)   │                   │
│              │                                       │                   │
│              │  NumberOfTasks:     5                 │                   │
│              │  TotalRequests:     1200              │                   │
│              │  TotalTimeTaken_ms: 8500              │                   │
│              └───────────────────────────────────────┘                   │
│                                                                            │
│                      ┌──────────────────────────┐                         │
│                      │  Back to Node data ▲     │                         │
│                      └────────────────────│─────┘                         │
│                                           │                               │
│  ┌────────────────────────────────────────│───────────────────────┐      │
│  │        NodeDataWriteBulk_sTasks        │                       │      │
│  │                                        │                       │      │
│  │  TotalWiteBulk_sTasks:         45     │                       │      │
│  │  TotalWriteBulk_sRequests:     12500  │                       │      │
│  │  TotalWrietBulk_sTimeTaken_ms: 85000  │                       │      │
│  │  Zone:                         "us-east-1a"                   │      │
│  │                                                               │      │
│  │  DataWriteBulk_sByShard: map[string]*AggShardTaskDataWriteBulk_s│   │
│  │  ├─ "logs-app_0" ───────────┐                                 │      │
│  │  ├─ "logs-app_1" ────────┐  │                                 │      │
│  │  ├─ "metrics-system_0" ┐ │  │                                 │      │
│  │  └─ "metrics-system_1" │ │  │                                 │      │
│  │                        │ │  │                                 │      │
│  │  SortedShardsOnTasks:  │ │  │                                 │      │
│  │    ["logs-app_0", "metrics-system_0", ...]                    │      │
│  │                        │ │  │                                 │      │
│  │  SortedShardsOnTimetaken:                                     │      │
│  │    ["logs-app_0", "metrics-system_1", ...]                    │      │
│  │                        │ │  │                                 │      │
│  │  SortedShardsOnRequest:                                       │      │
│  │    ["logs-app_0", "logs-app_1", ...]                          │      │
│  │                        │ │  │                                 │      │
│  └────────────────────────│─│──│───────────────────────────────┘      │
│                           │ │  │                                       │
│                           ▼ ▼  ▼                                       │
│              ┌───────────────────────────────────────┐                  │
│              │  AggShardTaskDataWriteBulk_s          │                  │
│              │  (Shard: "logs-app_0")                │                  │
│              │                                       │                  │
│              │  NumberOfTasks:     5                 │                  │
│              │  TotalRequests:     1200              │                  │
│              │  TotalTimeTaken_ms: 8500              │                  │
│              └───────────────────────────────────────┘                  │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

### Bulk Write Tasks Data Flow

```
Time ──────────────────────────────────────────────────────────────►

T=2m    │ getTDataWriteBulk_sTasks (1st run)
        │ ├─ Query: /_tasks?pretty&human&detailed=true
        │ ├─ Filter: action = "indices:data/write/bulk[s]"
        │ ├─ Parse: description = "requests[236], index[idx][2]"
        │ ├─ Aggregate:
        │ │  ├─ Shard Level:  "idx_2" → AggShardTaskDataWriteBulk_s
        │ │  ├─ Node Level:   host → NodeDataWriteBulk_sTasks
        │ │  ├─ Index Level:  "idx" → AggShardTaskDataWriteBulk_s
        │ │  └─ Cluster Level: Complete snapshot
        │ └─ Store: AllClusterDataWriteBulk_sTasksHistory[cluster].Ptr[0]
        │
T=3m    │ getTDataWriteBulk_sTasks (2nd run)
        │ ├─ Roll over: Ptr[0] → Ptr[1]
        │ └─ Store new: Ptr[0] = new snapshot
        │
T=4m    │ getTDataWriteBulk_sTasks (3rd run)
        │ ├─ Roll over: Ptr[0] → Ptr[1], Ptr[1] → Ptr[2]
        │ └─ Store new: Ptr[0] = new snapshot
        │
        │ ... (continues every minute)
        │
T=62m   │ getTDataWriteBulk_sTasks (60th run)
        │ └─ Now have full 60-minute history (or configured historySize)
```

### Multi-level Aggregation

```
┌────────────────────────────────────────────────────────────────┐
│  ELASTICSEARCH _tasks API RESPONSE                            │
│  {                                                             │
│    "nodes": {                                                  │
│      "node_id_1": {                                            │
│        "host": "host1.example.com",                            │
│        "tasks": {                                              │
│          "task_id_1": {                                        │
│            "action": "indices:data/write/bulk[s]",             │
│            "description": "requests[236], index[logs-app][0]", │
│            "running_time_in_nanos": 8500000000                 │
│          }                                                     │
│        }                                                       │
│      }                                                         │
│    }                                                           │
│  }                                                             │
└────────────────────────────────────────────────────────────────┘
                         │
                         │ Parse & Aggregate
                         ▼
┌────────────────────────────────────────────────────────────────┐
│  SHARD LEVEL: "logs-app_0"                                     │
│  ├─ NumberOfTasks: 1                                           │
│  ├─ TotalRequests: 236                                         │
│  └─ TotalTimeTaken_ms: 8500                                    │
└────────────────────────────────────────────────────────────────┘
                         │
                         │ Aggregate by Host
                         ▼
┌────────────────────────────────────────────────────────────────┐
│  NODE LEVEL: "host1.example.com"                               │
│  ├─ TotalWiteBulk_sTasks: 45 (sum of all shards)              │
│  ├─ TotalWriteBulk_sRequests: 12500 (sum)                     │
│  ├─ TotalWrietBulk_sTimeTaken_ms: 85000 (sum)                 │
│  ├─ Zone: "us-east-1a"                                         │
│  ├─ DataWriteBulk_sByShard: {                                 │
│  │     "logs-app_0": {...},                                    │
│  │     "logs-app_1": {...},                                    │
│  │     ...                                                     │
│  │  }                                                          │
│  └─ SortedShardsOnTasks: ["logs-app_0", ...]                  │
└────────────────────────────────────────────────────────────────┘
                         │
                         │ Aggregate by Index
                         ▼
┌────────────────────────────────────────────────────────────────┐
│  INDEX LEVEL: "logs-app" (sum of _0, _1, _2 shards)           │
│  ├─ NumberOfTasks: 25 (sum across all shards)                 │
│  ├─ TotalRequests: 5000 (sum)                                 │
│  └─ TotalTimeTaken_ms: 45000 (sum)                            │
└────────────────────────────────────────────────────────────────┘
                         │
                         │ Aggregate all nodes
                         ▼
┌────────────────────────────────────────────────────────────────┐
│  CLUSTER LEVEL: Complete snapshot                             │
│  ├─ DataWriteBulk_sTasksByNode: {...all hosts...}             │
│  ├─ SortedHostsOnTasks: [...busiest hosts first...]           │
│  ├─ DataWriteBulk_sTasksByIndex: {...all indices...}          │
│  └─ IndicesSortedonTasks: [...busiest indices first...]       │
└────────────────────────────────────────────────────────────────┘
```

---

## 6. Thread Pool Write Queue Structure

*[Existing TPWQueue structure documentation remains unchanged]*

---

## 7. Write Pressure Detection Structure

```
┌────────────────────────────────────────────────────────────────┐
│  WritePressureMap: map[string]*WritePressureEvent              │
│  Key format: "hostname_epochSeconds"                          │
├────────────────────────────────────────────────────────────────┤
│                                                                │
│  Key: "host1.example.com_1704567890" ──┐                      │
│  Key: "host2.example.com_1704567920" ──┼──┐                   │
│  Key: "host3.example.com_1704567950" ──┼──┼──┐                │
│                                         │  │  │                │
│                                         ▼  ▼  ▼                │
│                    ┌───────────────────────────────┐           │
│                    │  WritePressureEvent           │           │
│                    │                               │           │
│                    │  EventStartTime: 1704567890   │           │
│                    │  HostName:       "host1..."   │           │
│                    │  ClusterName:    "prod-..."   │           │
│                    └───────────────────────────────┘           │
│                                                                │
│  Cleanup: Entries older than oldRunTime are removed           │
└────────────────────────────────────────────────────────────────┘
```

---

## 8. Current Master Endpoints Structure

```
┌────────────────────────────────────────────────────────────────┐
│  AllCurrentMasterEndPoints: map[string]string                  │
├────────────────────────────────────────────────────────────────┤
│                                                                │
│  Key: "prod-cluster-01" → "https://master-node-1:9200/"       │
│  Key: "dev-cluster-01"  → "https://dev-master:9200/"          │
│  Key: "uat-cluster-01"  → "https://uat-master:9200/"          │
│                                                                │
│  Updated by: updateCurrentMasterEndPoints (init job)           │
│  Used by: getTDataWriteBulk_sTasks (periodic job)             │
└────────────────────────────────────────────────────────────────┘
```

---

## Summary

### Key Relationships:

1. **AllClusters** ←→ **AllClustersList**: 
   - Same clusters, different access patterns
   - Map for O(1) lookup, List for ordered iteration

2. **AllClusters** → **AllHistory**: 
   - One-to-one relationship by cluster name
   - History stored per cluster

3. **AllHistory** → **AllIndexingRate**: 
   - History is input, IndexingRate is output
   - Calculated from historical snapshots

4. **AllClusterDataWriteBulk_sTasksHistory** → **Bulk Write Monitoring**:
   - Real-time bulk write task tracking
   - Multi-level aggregation (shard, node, index, cluster)
   - Historical snapshots (configurable 10-180, default 60)
   - Pre-sorted data for quick access

5. **AllCurrentMasterEndPoints** → **Master Node Detection**:
   - Maps cluster names to current master endpoints
   - Used by bulk write tasks monitoring
   - Updated during initialization

6. **WritePressureMap** → **Write Pressure Events**:
   - Tracks write pressure events per host
   - Key format: "hostname_epochSeconds"
   - Automatic cleanup of old entries

7. **All structures share cluster names as keys**:
   - Enables easy cross-referencing
   - Maintained by LoadFromMasterCSV job

### Data Flow:
```
CSV Files → AllClusters → API Calls → AllHistory → AllIndexingRate → API Responses
                                    ↓
                    Master Endpoints Detection → Bulk Write Tasks Monitoring
                                                ↓
                                    Thread Pool Monitoring → Write Pressure Detection
```

### Thread Safety (Updated):
```
┌─────────────────────────────────────────────────────────────────────┐
│  Global Data Structure                    │  Mutex                  │
├───────────────────────────────────────────┼─────────────────────────┤
│  AllClusters                              │  ClustersMu             │
│  AllClustersList                          │  ClustersMu             │
│  AllHistory                               │  HistoryMu              │
│  AllIndexingRate                          │  IndexingRateMu         │
│  AllStatsByDay                            │  StatsByDayMu           │
│  AllThreadPoolWriteQueues                 │  TPWQueueMu             │
│  AllClusterDataWriteBulk_sTasksHistory    │  ClusterDataWrite...Mu  │
│  WritePressureMap                         │  WritePressureMu        │
│  AllCurrentMasterEndPoints                │  CurrentMasterEndPtsMu  │
└─────────────────────────────────────────────────────────────────────┘
```

### Memory Footprint (with all features):
```
Example with 3 clusters, monitoring all features:

    AllClusters:                   3 × ~1KB                    ≈ 3 KB
    AllHistory:                    3 × 21 × 100 indices        ≈ 630 KB
    AllIndexingRate:               3 × 100 indices             ≈ 9 KB
    AllStatsByDay:                 3 × 30 days × 100 indices   ≈ 900 KB
    AllThreadPoolWriteQueues:      3 × 6 sets × 10 hosts       ≈ 180 KB
    AllClusterDataWriteBulk...:    3 × 60 snapshots × 10 hosts ≈ 1.8 MB
    WritePressureMap:              ~50 events                  ≈ 5 KB
    AllCurrentMasterEndPoints:     3 entries                   ≈ 1 KB
                                                      Total    ≈ 3.5 MB

Memory scales with:
    - Number of clusters
    - History sizes (configurable per feature)
    - Number of indices/hosts per cluster
    - Monitoring frequency
```
