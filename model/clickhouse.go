package model

import (
	"github.com/coroot/coroot/timeseries"
)

type ClickhouseTable struct {
	Db    string
	Table string
}

func (t ClickhouseTable) String() string {
	return t.Db + "." + t.Table
}

// Clickhouse holds the statistics the cluster agent reads from a ClickHouse
// server's system tables.
type Clickhouse struct {
	Up    *timeseries.TimeSeries
	Error LabelLastValue

	Version LabelLastValue

	Metrics      map[string]*timeseries.TimeSeries // system.metrics: current values
	AsyncMetrics map[string]*timeseries.TimeSeries // system.asynchronous_metrics

	// What other users do (system.query_log, without the agent's own
	// queries and the server's log tables), per second.
	Queries        map[string]*timeseries.TimeSeries // by kind: select, insert, other
	FailedQueries  *timeseries.TimeSeries
	QueryTime      *timeseries.TimeSeries // seconds spent in queries
	InsertedRows   *timeseries.TimeSeries
	RunningQueries *timeseries.TimeSeries

	ReplicatedTables    *timeseries.TimeSeries
	MutationsUnfinished *timeseries.TimeSeries
	MutationsFailing    *timeseries.TimeSeries
	LongestQuery        *timeseries.TimeSeries
	PartsToThrowInsert  *timeseries.TimeSeries
	PartsToDelayInsert  *timeseries.TimeSeries

	// Tables outside the system database.
	TablesSize          *timeseries.TimeSeries
	MaxPartsInPartition *timeseries.TimeSeries

	TableSize  map[ClickhouseTable]*timeseries.TimeSeries
	TableParts map[ClickhouseTable]*timeseries.TimeSeries
}

func NewClickhouse() *Clickhouse {
	return &Clickhouse{
		Metrics:      map[string]*timeseries.TimeSeries{},
		Queries:      map[string]*timeseries.TimeSeries{},
		AsyncMetrics: map[string]*timeseries.TimeSeries{},
		TableSize:    map[ClickhouseTable]*timeseries.TimeSeries{},
		TableParts:   map[ClickhouseTable]*timeseries.TimeSeries{},
	}
}

func (c *Clickhouse) IsUp() bool {
	return c.Up.Last() > 0
}

// IsReplicated: the server has replicated tables (it is part of an HA setup).
func (c *Clickhouse) IsReplicated() bool {
	return c.ReplicatedTables.Last() > 0
}
