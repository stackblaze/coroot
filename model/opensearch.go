package model

import (
	"github.com/coroot/coroot/timeseries"
)

// Opensearch holds the statistics the cluster agent reads from an OpenSearch
// (or Elasticsearch) node. Cluster* values are cluster-wide and reported by
// every node; the rest is the node's own.
type Opensearch struct {
	Up    *timeseries.TimeSeries
	Error LabelLastValue

	Version       LabelLastValue
	Roles         LabelLastValue
	ClusterName   LabelLastValue
	ClusterStatus LabelLastValue

	ClusterManager      *timeseries.TimeSeries
	ClusterNodes        *timeseries.TimeSeries
	ClusterShards       map[string]*timeseries.TimeSeries // by state
	ClusterPendingTasks *timeseries.TimeSeries

	Docs      *timeseries.TimeSeries
	StoreSize *timeseries.TimeSeries

	IndexingOps    *timeseries.TimeSeries
	IndexingTime   *timeseries.TimeSeries
	IndexingFailed *timeseries.TimeSeries
	SearchOps      map[string]*timeseries.TimeSeries // by phase
	SearchTime     map[string]*timeseries.TimeSeries // by phase

	HeapUsed *timeseries.TimeSeries
	HeapMax  *timeseries.TimeSeries
	GcTime   map[string]*timeseries.TimeSeries // by collector

	PoolRejected   map[string]*timeseries.TimeSeries // by thread pool
	PoolQueue      map[string]*timeseries.TimeSeries // by thread pool
	BreakerTripped map[string]*timeseries.TimeSeries // by circuit breaker
}

func NewOpensearch() *Opensearch {
	return &Opensearch{
		ClusterShards:  map[string]*timeseries.TimeSeries{},
		SearchOps:      map[string]*timeseries.TimeSeries{},
		SearchTime:     map[string]*timeseries.TimeSeries{},
		GcTime:         map[string]*timeseries.TimeSeries{},
		PoolRejected:   map[string]*timeseries.TimeSeries{},
		PoolQueue:      map[string]*timeseries.TimeSeries{},
		BreakerTripped: map[string]*timeseries.TimeSeries{},
	}
}

func (o *Opensearch) IsUp() bool {
	return o.Up.Last() > 0
}

func (o *Opensearch) IsClusterManager() bool {
	return o.ClusterManager.Last() > 0
}
