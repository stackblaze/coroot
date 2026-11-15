package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

// The tab exists once the cluster agent scrapes at least one server. Query,
// insert, size and parts figures are the application's: the agent leaves out
// its own queries and the server's log tables (database system).
func (a *appAuditor) clickhouse() {
	if !a.app.IsClickhouse() {
		return
	}

	report := a.addReport(model.AuditReportClickhouse)

	availabilityCheck := report.CreateCheck(model.Checks.ClickhouseAvailability)
	replicationCheck := report.CreateCheck(model.Checks.ClickhouseReplication)
	partsCheck := report.CreateCheck(model.Checks.ClickhouseParts)
	mutationsCheck := report.CreateCheck(model.Checks.ClickhouseMutations)

	table := report.GetOrCreateTable("Instance", "Status", "Queries", "Replication delay", "Data size", "Version")
	queriesChart := report.GetOrCreateChartGroup("Queries on <selector>, per second", nil)
	latencyChart := report.GetOrCreateChart("Average query time, seconds", nil)
	runningChart := report.GetOrCreateChart("Running queries", nil)
	longestChart := report.GetOrCreateChart("Longest running query, seconds", nil)
	insertedChart := report.GetOrCreateChart("Rows inserted by queries, per second", nil)
	connectionsChart := report.GetOrCreateChartGroup("Connections on <selector>", nil)
	partsChart := report.GetOrCreateChartGroup("Active parts in the fullest partition on <selector>", nil)
	mergesChart := report.GetOrCreateChartGroup("Merges and mutations on <selector>", nil)
	delayChart := report.GetOrCreateChart("Replication delay, seconds", nil)
	queueChart := report.GetOrCreateChart("Replication queue size", nil)
	memoryChart := report.GetOrCreateChart("Memory tracked by the server, bytes", nil)
	tablesChart := report.GetOrCreateChartGroup("Table sizes on <selector>, bytes", nil)
	tablePartsChart := report.GetOrCreateChartGroup("Active parts by table on <selector>", nil)

	availabilityCheck.AddWidget(table.Widget())
	replicationCheck.AddWidget(delayChart.Widget())
	replicationCheck.AddWidget(queueChart.Widget())
	partsCheck.AddWidget(partsChart.Widget())
	partsCheck.AddWidget(tablePartsChart.Widget())
	mutationsCheck.AddWidget(mergesChart.Widget())

	for _, i := range a.app.Instances {
		c := i.Clickhouse
		if c == nil {
			continue
		}
		obsolete := i.IsObsolete()
		live := !obsolete && c.IsUp()

		if !obsolete && !c.IsUp() {
			availabilityCheck.AddItem("%s", i.Name)
		}

		delay := c.AsyncMetrics["ReplicasMaxAbsoluteDelay"]
		readonly := c.Metrics["ReadonlyReplica"]
		if live && c.IsReplicated() && (delay.Last() > replicationCheck.Threshold || readonly.Last() > 0) {
			replicationCheck.AddItem("%s", i.Name)
		}

		maxParts := c.MaxPartsInPartition
		if limit := c.PartsToDelayInsert.Last(); live && limit > 0 && maxParts.Last()/limit*100 > partsCheck.Threshold {
			partsCheck.AddItem("%s", i.Name)
		}

		if live && c.MutationsFailing.Last() > 0 {
			mutationsCheck.AddItem("%s", i.Name)
		}

		if !obsolete && table != nil {
			clickhouseInstanceRow(table, i)
		}

		if queriesChart != nil {
			queriesChart.GetOrCreateChart(i.Name).
				AddSeries("select", c.Queries["select"]).
				AddSeries("insert", c.Queries["insert"]).
				AddSeries("other", c.Queries["other"]).
				AddSeries("failed", c.FailedQueries, "black")
		}
		if latencyChart != nil {
			latencyChart.AddSeries(i.Name, timeseries.Div(c.QueryTime, clickhouseQueries(c)))
		}
		if runningChart != nil {
			runningChart.AddSeries(i.Name, c.RunningQueries)
		}
		if longestChart != nil {
			longestChart.AddSeries(i.Name, c.LongestQuery)
		}
		if insertedChart != nil {
			insertedChart.AddSeries(i.Name, c.InsertedRows)
		}
		if connectionsChart != nil {
			connectionsChart.GetOrCreateChart(i.Name).Stacked().
				AddSeries("native", c.Metrics["TCPConnection"]).
				AddSeries("http", c.Metrics["HTTPConnection"]).
				AddSeries("mysql", c.Metrics["MySQLConnection"]).
				AddSeries("postgresql", c.Metrics["PostgreSQLConnection"]).
				AddSeries("interserver", c.Metrics["InterserverConnection"])
		}
		if partsChart != nil {
			partsChart.GetOrCreateChart(i.Name).
				AddSeries("active parts", maxParts).
				AddSeries("inserts are slowed down", c.PartsToDelayInsert, "orange").
				SetThreshold("inserts are rejected", c.PartsToThrowInsert)
		}
		if mergesChart != nil {
			mergesChart.GetOrCreateChart(i.Name).
				AddSeries("running merges", c.Metrics["Merge"]).
				AddSeries("running mutations", c.Metrics["PartMutation"]).
				AddSeries("unfinished mutations", c.MutationsUnfinished).
				AddSeries("failing mutations", c.MutationsFailing, "red")
		}
		if c.IsReplicated() {
			if delayChart != nil {
				delayChart.AddSeries(i.Name, delay)
			}
			if queueChart != nil {
				queueChart.AddSeries(i.Name, c.AsyncMetrics["ReplicasMaxQueueSize"])
			}
		}
		if memoryChart != nil {
			memoryChart.AddSeries(i.Name, c.Metrics["MemoryTracking"])
		}
		if tablesChart != nil {
			byTable := map[string]model.SeriesData{}
			for t, ts := range c.TableSize {
				byTable[t.String()] = ts
			}
			tablesChart.GetOrCreateChart(i.Name).Stacked().Sorted().AddMany(byTable, 5, timeseries.Max)
		}
		if tablePartsChart != nil {
			byTable := map[string]model.SeriesData{}
			for t, ts := range c.TableParts {
				byTable[t.String()] = ts
			}
			tablePartsChart.GetOrCreateChart(i.Name).Sorted().AddMany(byTable, 5, timeseries.Max)
		}
	}
}

func clickhouseInstanceRow(table *model.Table, i *model.Instance) {
	c := i.Clickhouse
	name := model.NewTableCell(i.Name)

	status := model.NewTableCell().SetStatus(model.OK, "up")
	if !c.IsUp() {
		if v := c.Error.Value(); v != "" {
			status.SetStatus(model.WARNING, v)
		} else {
			status.SetStatus(model.WARNING, "down (no metrics)")
		}
	} else if c.Metrics["ReadonlyReplica"].Last() > 0 {
		status.SetStatus(model.WARNING, "replicated tables are read-only")
	}

	queries := model.NewTableCell()
	if v := clickhouseQueries(c).Last(); !timeseries.IsNaN(v) {
		queries.SetValue(utils.FormatFloat(v)).SetUnit("/s")
	}

	delay := model.NewTableCell()
	if c.IsReplicated() {
		if v := c.AsyncMetrics["ReplicasMaxAbsoluteDelay"].Last(); !timeseries.IsNaN(v) {
			delay.SetValue(utils.FormatDuration(timeseries.Duration(v), 1))
		}
	} else if c.IsUp() {
		delay.SetValue("not replicated")
	}

	size := model.NewTableCell()
	if v := c.TablesSize.Last(); !timeseries.IsNaN(v) {
		size.Value, size.Unit = utils.FormatBytes(v)
	}

	table.AddRow(name, status, queries, delay, size, model.NewTableCell(c.Version.Value()))
}

// clickhouseQueries is the rate of all the queries other users ran.
func clickhouseQueries(c *model.Clickhouse) *timeseries.TimeSeries {
	total := timeseries.NewAggregate(timeseries.NanSum)
	for _, ts := range c.Queries {
		total.Add(ts)
	}
	return total.Get()
}
