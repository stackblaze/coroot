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
	Events       map[string]*timeseries.TimeSeries // system.events: per-second rates
	AsyncMetrics map[string]*timeseries.TimeSeries // system.asynchronous_metrics

	ReplicatedTables    *timeseries.TimeSeries
	MutationsUnfinished *timeseries.TimeSeries
	MutationsFailing    *timeseries.TimeSeries
	LongestQuery        *timeseries.TimeSeries
	PartsToThrowInsert  *timeseries.TimeSeries
	PartsToDelayInsert  *timeseries.TimeSeries

	TableSize  map[ClickhouseTable]*timeseries.TimeSeries
	TableParts map[ClickhouseTable]*timeseries.TimeSeries
}

func NewClickhouse() *Clickhouse {
	return &Clickhouse{
		Metrics:      map[string]*timeseries.TimeSeries{},
		Events:       map[string]*timeseries.TimeSeries{},
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
