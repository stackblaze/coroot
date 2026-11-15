package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func clickhouse(instance *model.Instance, queryName string, m *model.MetricValues) {
	if instance == nil {
		return
	}
	if instance.Clickhouse == nil {
		instance.Clickhouse = model.NewClickhouse()
	}
	c := instance.Clickhouse
	switch queryName {
	case "clickhouse_up":
		c.Up = merge(c.Up, m.Values, timeseries.Any)
	case "clickhouse_scrape_error":
		c.Error.Update(m.Values, m.Labels["error"])
	case "clickhouse_info":
		c.Version.Update(m.Values, m.Labels["version"])
	case "clickhouse_metric":
		n := m.Labels["metric"]
		c.Metrics[n] = merge(c.Metrics[n], m.Values, timeseries.Any)
	case "clickhouse_queries_per_second":
		k := m.Labels["kind"]
		c.Queries[k] = merge(c.Queries[k], m.Values, timeseries.Any)
	case "clickhouse_failed_queries_per_second":
		c.FailedQueries = merge(c.FailedQueries, m.Values, timeseries.Any)
	case "clickhouse_query_seconds_per_second":
		c.QueryTime = merge(c.QueryTime, m.Values, timeseries.Any)
	case "clickhouse_inserted_rows_per_second":
		c.InsertedRows = merge(c.InsertedRows, m.Values, timeseries.Any)
	case "clickhouse_running_queries":
		c.RunningQueries = merge(c.RunningQueries, m.Values, timeseries.Any)
	case "clickhouse_tables_size_bytes":
		c.TablesSize = merge(c.TablesSize, m.Values, timeseries.Any)
	case "clickhouse_max_parts_in_partition":
		c.MaxPartsInPartition = merge(c.MaxPartsInPartition, m.Values, timeseries.Any)
	case "clickhouse_async_metric":
		n := m.Labels["metric"]
		c.AsyncMetrics[n] = merge(c.AsyncMetrics[n], m.Values, timeseries.Any)
	case "clickhouse_replicated_tables":
		c.ReplicatedTables = merge(c.ReplicatedTables, m.Values, timeseries.Any)
	case "clickhouse_mutations_unfinished":
		c.MutationsUnfinished = merge(c.MutationsUnfinished, m.Values, timeseries.Any)
	case "clickhouse_mutations_failing":
		c.MutationsFailing = merge(c.MutationsFailing, m.Values, timeseries.Any)
	case "clickhouse_longest_query_seconds":
		c.LongestQuery = merge(c.LongestQuery, m.Values, timeseries.Any)
	case "clickhouse_parts_to_throw_insert":
		c.PartsToThrowInsert = merge(c.PartsToThrowInsert, m.Values, timeseries.Any)
	case "clickhouse_parts_to_delay_insert":
		c.PartsToDelayInsert = merge(c.PartsToDelayInsert, m.Values, timeseries.Any)
	case "clickhouse_table_size_bytes":
		t := model.ClickhouseTable{Db: m.Labels["db"], Table: m.Labels["table"]}
		c.TableSize[t] = merge(c.TableSize[t], m.Values, timeseries.Any)
	case "clickhouse_table_parts":
		t := model.ClickhouseTable{Db: m.Labels["db"], Table: m.Labels["table"]}
		c.TableParts[t] = merge(c.TableParts[t], m.Values, timeseries.Any)
	}
}
