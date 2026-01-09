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

4. **All structures share cluster names as keys**:
   - Enables easy cross-referencing
   - Maintained by LoadFromMasterCSV job

### Data Flow:
```
CSV Files → AllClusters → API Calls → AllHistory → AllIndexingRate → API Responses
```
