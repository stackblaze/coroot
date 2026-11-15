package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

// The tab exists once the cluster agent scrapes at least one node.
func (a *appAuditor) opensearch() {
	if !a.app.IsOpensearch() {
		return
	}

	report := a.addReport(model.AuditReportOpensearch)

	availabilityCheck := report.CreateCheck(model.Checks.OpensearchAvailability)
	statusCheck := report.CreateCheck(model.Checks.OpensearchClusterStatus)
	rejectionsCheck := report.CreateCheck(model.Checks.OpensearchRejections)

	table := report.GetOrCreateTable("Node", "Roles", "Status", "Cluster", "Documents", "Size", "Version")
	shardsChart := report.GetOrCreateChart("Shards by state", nil)
	searchChart := report.GetOrCreateChart("Search requests, per second", nil)
	searchLatencyChart := report.GetOrCreateChart("Search average latency, seconds", nil)
	indexingChart := report.GetOrCreateChart("Indexing operations, per second", nil)
	indexingLatencyChart := report.GetOrCreateChart("Indexing average latency, seconds", nil)
	rejectionsChart := report.GetOrCreateChartGroup("Rejected requests on <selector>, per second", nil)
	queueChart := report.GetOrCreateChartGroup("Thread pool queues on <selector>", nil)
	heapChart := report.GetOrCreateChart("JVM heap usage, %", nil)
	gcChart := report.GetOrCreateChart("Time spent in garbage collection, seconds/second", nil)
	docsChart := report.GetOrCreateChart("Documents", nil)
	sizeChart := report.GetOrCreateChart("Index size, bytes", nil)

	availabilityCheck.AddWidget(table.Widget())
	statusCheck.AddWidget(shardsChart.Widget())
	rejectionsCheck.AddWidget(rejectionsChart.Widget())
	rejectionsCheck.AddWidget(queueChart.Widget())

	redClusters := map[string]bool{}

	// Shards are cluster-wide, so one node's view is enough: the elected
	// cluster manager's, or the first live node's when it is not scraped.
	var shardsSource *model.Instance
	for _, i := range a.app.Instances {
		if o := i.Opensearch; o != nil && !i.IsObsolete() && o.IsUp() {
			if shardsSource == nil || o.IsClusterManager() {
				shardsSource = i
			}
		}
	}

	for _, i := range a.app.Instances {
		o := i.Opensearch
		if o == nil {
			continue
		}
		obsolete := i.IsObsolete()

		if !obsolete && !o.IsUp() {
			availabilityCheck.AddItem("%s", i.Name)
		}

		cluster := o.ClusterName.Value()
		if !obsolete && o.IsUp() && o.ClusterStatus.Value() == "red" && !redClusters[cluster] {
			redClusters[cluster] = true
			statusCheck.AddItem("%s", cluster)
		}

		rejected := timeseries.NewAggregate(timeseries.NanSum)
		rejectedBy := map[string]model.SeriesData{}
		for pool, ts := range o.PoolRejected {
			rejected.Add(ts)
			rejectedBy[pool+" pool"] = ts
		}
		for breaker, ts := range o.BreakerTripped {
			rejected.Add(ts)
			rejectedBy[breaker+" breaker"] = ts
		}
		if !obsolete && rejected.Get().Last() > 0 {
			rejectionsCheck.AddItem("%s", i.Name)
		}

		if !obsolete && table != nil {
			opensearchNodeRow(table, i)
		}

		if shardsChart != nil && i == shardsSource {
			shardsChart.Stacked()
			for _, state := range []string{"active", "relocating", "initializing", "unassigned"} {
				shardsChart.AddSeries(state, o.ClusterShards[state])
			}
		}

		if searchChart != nil {
			searchChart.AddSeries(i.Name, o.SearchOps["query"])
		}
		if searchLatencyChart != nil {
			searchLatencyChart.AddSeries(i.Name, timeseries.Div(o.SearchTime["query"], o.SearchOps["query"]))
		}
		if indexingChart != nil {
			indexingChart.AddSeries(i.Name, o.IndexingOps)
		}
		if indexingLatencyChart != nil {
			indexingLatencyChart.AddSeries(i.Name, timeseries.Div(o.IndexingTime, o.IndexingOps))
		}
		if rejectionsChart != nil {
			rejectionsChart.GetOrCreateChart(i.Name).Stacked().Sorted().AddMany(rejectedBy, 5, timeseries.NanSum)
		}
		if queueChart != nil {
			byPool := map[string]model.SeriesData{}
			for pool, ts := range o.PoolQueue {
				byPool[pool] = ts
			}
			queueChart.GetOrCreateChart(i.Name).Sorted().AddMany(byPool, 5, timeseries.Max)
		}
		if heapChart != nil {
			heapChart.AddSeries(i.Name, timeseries.Aggregate2(o.HeapUsed, o.HeapMax, func(used, max float32) float32 { return used / max * 100 }))
		}
		if gcChart != nil {
			gc := timeseries.NewAggregate(timeseries.NanSum)
			for _, ts := range o.GcTime {
				gc.Add(ts)
			}
			gcChart.AddSeries(i.Name, gc.Get())
		}
		if docsChart != nil {
			docsChart.AddSeries(i.Name, o.Docs)
		}
		if sizeChart != nil {
			sizeChart.AddSeries(i.Name, o.StoreSize)
		}
	}
}

func opensearchNodeRow(table *model.Table, i *model.Instance) {
	o := i.Opensearch
	name := model.NewTableCell(i.Name)
	roles := model.NewTableCell(o.Roles.Value())
	if o.IsClusterManager() {
		roles.SetIcon("mdi-database-edit-outline", "rgba(0,0,0,0.87)")
		roles.AddTag("elected cluster manager")
	}

	status := model.NewTableCell().SetStatus(model.OK, "up")
	if !o.IsUp() {
		if v := o.Error.Value(); v != "" {
			status.SetStatus(model.WARNING, v)
		} else {
			status.SetStatus(model.WARNING, "down (no metrics)")
		}
	}

	// Yellow (a replica shard is not assigned) is what a single-node cluster
	// with the default one replica per index always shows, so it is not a
	// warning here; red (a primary is missing) is the check.
	cluster := model.NewTableCell()
	if o.IsUp() {
		switch s := o.ClusterStatus.Value(); s {
		case "green", "yellow":
			cluster.SetStatus(model.OK, s)
		case "red":
			cluster.SetStatus(model.CRITICAL, s)
		}
	}

	docs := model.NewTableCell()
	if v := o.Docs.Last(); !timeseries.IsNaN(v) {
		docs.SetValue(utils.FormatFloat(v))
	}
	size := model.NewTableCell()
	if v := o.StoreSize.Last(); !timeseries.IsNaN(v) {
		size.Value, size.Unit = utils.FormatBytes(v)
	}
	table.AddRow(name, roles, status, cluster, docs, size, model.NewTableCell(o.Version.Value()))
}
