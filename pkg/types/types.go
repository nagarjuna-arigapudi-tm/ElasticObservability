package types

import "sync"

// Node represents an Elasticsearch node
type Node struct {
	HostName     string   `json:"hostName" yaml:"hostName"`
	IPAddress    string   `json:"ipAddress" yaml:"ipAddress"`
	Port         string   `json:"port" yaml:"port"`
	Type         []string `json:"type" yaml:"type"` // master, data, logstash, kibana, ml
	Zone         string   `json:"zone" yaml:"zone"`
	KibanaPort   string   `json:"kibanaPort" yaml:"kibanaPort"`
	LogstashPort string   `json:"logstashPort" yaml:"logstashPort"`
	DataCenter   string   `json:"dataCenter" yaml:"dataCenter"`
	Rack         string   `json:"rack" yaml:"rack"`
	NodeTier     string   `json:"nodeTier" yaml:"nodeTier"` // hot, warm, cold
}

// AccessCred holds authentication credentials
type AccessCred struct {
	Preferred  uint8  `json:"preferred" yaml:"preferred"` // 1=apikey, 2=userid/password, 3=certificate
	APIKey     string `json:"apiKey" yaml:"apiKey"`
	UserID     string `json:"userID" yaml:"userID"`
	Password   string `json:"password" yaml:"password"`
	ClientCert string `json:"clientCert" yaml:"clientCert"`
	ClientKey  string `json:"clientKey" yaml:"clientKey"`
	CaCert     string `json:"caCert" yaml:"caCert"`
}

// ClusterData represents cluster information
type ClusterData struct {
	ClusterName     string
	ClusterUUID     string
	CurrentEndpoint string
	InsecureTLS     bool
	Active          bool
	ZoneIdentifier  string
	ClusterSAN      []string
	ActiveEndpoint  string
	KibanaSAN       []string
	Owner           string
	Env             string
	ClusterPort     string // Default: "9200"
	KibanaPort      string // Default: "5601"
	AccessCred      AccessCred
	Nodes           []*Node
}

// IndexInfo represents information about an index
type IndexInfo struct {
	Health         uint8  `json:"health"`         // 1=green, 2=yellow, 3=red
	IsOpen         bool   `json:"isOpen"`         // true if open
	DocCount       uint64 `json:"docCount"`       // docs.count
	Index          string `json:"index"`          // index name
	IndexBase      string `json:"indexBase"`      // base part without digits/timestamp
	SeqNo          uint64 `json:"seqNo"`          // sequence number from index name
	PrimaryShards  uint8  `json:"primaryShards"`  // pri
	CreationTime   int64  `json:"creationTime"`   // creation.date in epoch milliseconds
	TotalStorage   uint64 `json:"totalStorage"`   // ss in bytes
	PrimaryStorage uint64 `json:"primaryStorage"` // pri.store.size in bytes
}

// IndicesSnapShot represents a snapshot of indices at a point in time
type IndicesSnapShot struct {
	SnapShotTime int64                 `json:"snapShotTime"` // epoch milliseconds
	MapIndices   map[string]*IndexInfo `json:"mapIndices"`   // map[index_base]*IndexInfo
}

// IndicesHistory maintains history of index snapshots
type IndicesHistory struct {
	SizeOfPtr uint8              `json:"sizeOfPtr"`
	Ptr       []*IndicesSnapShot `json:"ptr"`
	mu        sync.RWMutex       // for thread-safe access
}

// IndexingRate represents indexing rate metrics
type IndexingRate struct {
	FromCreation   float64 `json:"fromCreation"`   // bytes/ms per shard
	Last3Minutes   float64 `json:"last3Minutes"`   // bytes/ms per shard
	Last15Minutes  float64 `json:"last15Minutes"`  // bytes/ms per shard
	Last60Minutes  float64 `json:"last60Minutes"`  // bytes/ms per shard
	NumberOfShards uint8   `json:"numberOfShards"` // number of primary shards
}

// ClusterIndexingRate represents indexing rate for all indices in a cluster
type ClusterIndexingRate struct {
	Timestamp  int64                    `json:"timestamp"`  // epoch milliseconds
	MapIndices map[string]*IndexingRate `json:"mapIndices"` // map[index_base]*IndexingRate
}

// IndexStat represents statistics for an index at a point in time
type IndexStat struct {
	StatTime  int64  `json:"statTime"`  // epoch milliseconds
	TotalSize uint64 `json:"totalSize"` // total storage in bytes
	DocCount  uint64 `json:"docCount"`  // document count
}

// IndexStatHistory maintains daily statistics for an index
type IndexStatHistory struct {
	IndexName string       `json:"indexName"`
	SizeOfPtr uint8        `json:"sizeOfPtr"`
	StatsPtr  []*IndexStat `json:"statsPtr"`
}

// IndicesStatsByDay maintains daily statistics for all indices in a cluster
type IndicesStatsByDay struct {
	LastUpdateTime int64                        `json:"lastUpdateTime"` // epoch milliseconds
	StatHistory    map[string]*IndexStatHistory `json:"statHistory"`    // map[indexName]*IndexStatHistory
}

// Global data structures
var (
	AllClusters     map[string]*ClusterData         // map[clusterName]*ClusterData
	AllClustersList []string                        // list of all cluster names
	AllHistory      map[string]*IndicesHistory      // map[clusterName]*IndicesHistory
	AllIndexingRate map[string]*ClusterIndexingRate // map[clusterName]*ClusterIndexingRate
	AllStatsByDay   map[string]*IndicesStatsByDay   // map[clusterName]*IndicesStatsByDay

	// Mutexes for thread-safe access
	ClustersMu     sync.RWMutex
	HistoryMu      sync.RWMutex
	IndexingRateMu sync.RWMutex
	StatsByDayMu   sync.RWMutex
)

func init() {
	AllClusters = make(map[string]*ClusterData)
	AllClustersList = make([]string, 0)
	AllHistory = make(map[string]*IndicesHistory)
	AllIndexingRate = make(map[string]*ClusterIndexingRate)
	AllStatsByDay = make(map[string]*IndicesStatsByDay)
}

// NewIndicesHistory creates a new IndicesHistory with specified size
func NewIndicesHistory(size uint8) *IndicesHistory {
	return &IndicesHistory{
		SizeOfPtr: size,
		Ptr:       make([]*IndicesSnapShot, size+1),
	}
}

// AddSnapshot adds a new snapshot to history (thread-safe)
func (ih *IndicesHistory) AddSnapshot(snapshot *IndicesSnapShot) {
	ih.mu.Lock()
	defer ih.mu.Unlock()

	// Roll over old snapshots
	for i := 0; i < int(ih.SizeOfPtr); i++ {
		ih.Ptr[i] = ih.Ptr[i+1]
	}
	ih.Ptr[ih.SizeOfPtr] = snapshot
}

// GetCopy returns a copy of the history (thread-safe, shallow copy of pointers)
func (ih *IndicesHistory) GetCopy() *IndicesHistory {
	ih.mu.RLock()
	defer ih.mu.RUnlock()

	copy := &IndicesHistory{
		SizeOfPtr: ih.SizeOfPtr,
		Ptr:       make([]*IndicesSnapShot, len(ih.Ptr)),
	}
	for i := range ih.Ptr {
		copy.Ptr[i] = ih.Ptr[i]
	}
	return copy
}

// GetLatestIndex returns the index of the latest non-nil snapshot
func (ih *IndicesHistory) GetLatestIndex() int {
	ih.mu.RLock()
	defer ih.mu.RUnlock()

	for i := int(ih.SizeOfPtr); i >= 0; i-- {
		if ih.Ptr[i] != nil {
			return i
		}
	}
	return -1
}
