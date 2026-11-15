package auditor

import (
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dbTestAuditor(name string) (*appAuditor, timeseries.Context) {
	ctx := timeseries.NewContext(0, 600, 60)
	app := model.NewApplication(model.NewApplicationId("", "ns", model.ApplicationKindStatefulSet, name))
	return &appAuditor{w: &model.World{Ctx: ctx}, app: app, detailed: true}, ctx
}

func constSeries(ctx timeseries.Context, v float32) *timeseries.TimeSeries {
	data := make([]float32, ctx.PointsCount())
	for i := range data {
		data[i] = v
	}
	return timeseries.NewWithData(ctx.From, ctx.Step, data)
}

// failing returns the titles of a report's checks that are not OK.
func failing(r *model.AuditReport) []string {
	var res []string
	for _, ch := range r.Checks {
		ch.Calc()
		if ch.Status != model.OK {
			res = append(res, ch.Title)
		}
	}
	return res
}

func TestOpensearchReport(t *testing.T) {
	a, ctx := dbTestAuditor("opensearch")
	a.opensearch()
	assert.Empty(t, a.reports, "no tab until a node is scraped")

	healthy := model.NewInstance("opensearch-0", a.app)
	healthy.Opensearch = model.NewOpensearch()
	healthy.Opensearch.Up = constSeries(ctx, 1)
	healthy.Opensearch.ClusterName.Update(constSeries(ctx, 1), "os")
	// A single node with the default one replica per index is always yellow.
	healthy.Opensearch.ClusterStatus.Update(constSeries(ctx, 1), "yellow")
	healthy.Opensearch.PoolRejected["write"] = constSeries(ctx, 0)
	a.app.Instances = append(a.app.Instances, healthy)

	a.opensearch()
	require.Len(t, a.reports, 1)
	assert.Equal(t, model.AuditReportOpensearch, a.reports[0].Name)
	assert.Empty(t, failing(a.reports[0]))

	down := model.NewInstance("opensearch-1", a.app)
	down.Opensearch = model.NewOpensearch()
	down.Opensearch.Up = constSeries(ctx, 0)
	rejecting := model.NewInstance("opensearch-2", a.app)
	rejecting.Opensearch = model.NewOpensearch()
	rejecting.Opensearch.Up = constSeries(ctx, 1)
	rejecting.Opensearch.ClusterName.Update(constSeries(ctx, 1), "os")
	rejecting.Opensearch.ClusterStatus.Update(constSeries(ctx, 1), "red")
	rejecting.Opensearch.PoolRejected["write"] = constSeries(ctx, 2)
	a.app.Instances = append(a.app.Instances, down, rejecting)
	healthy.Opensearch.ClusterStatus.Update(constSeries(ctx, 1), "red")

	a.reports = nil
	a.opensearch()
	require.Len(t, a.reports, 1)
	assert.ElementsMatch(t,
		[]string{"OpenSearch availability", "OpenSearch cluster status", "OpenSearch rejected requests"},
		failing(a.reports[0]))
	for _, ch := range a.reports[0].Checks {
		if ch.Id == model.Checks.OpensearchClusterStatus.Id {
			assert.Equal(t, 1, ch.Items().Len(), "one red cluster, however many nodes report it")
		}
	}
}

func TestClickhouseReport(t *testing.T) {
	a, ctx := dbTestAuditor("clickhouse")
	a.clickhouse()
	assert.Empty(t, a.reports, "no tab until a server is scraped")

	i := model.NewInstance("clickhouse-0", a.app)
	c := model.NewClickhouse()
	i.Clickhouse = c
	c.Up = constSeries(ctx, 1)
	c.PartsToDelayInsert = constSeries(ctx, 1000)
	c.AsyncMetrics["MaxPartCountForPartition"] = constSeries(ctx, 300)
	// Not replicated: a delay value must not raise the replication check.
	c.ReplicatedTables = constSeries(ctx, 0)
	c.AsyncMetrics["ReplicasMaxAbsoluteDelay"] = constSeries(ctx, 120)
	c.MutationsFailing = constSeries(ctx, 0)
	a.app.Instances = append(a.app.Instances, i)

	a.clickhouse()
	require.Len(t, a.reports, 1)
	assert.Equal(t, model.AuditReportClickhouse, a.reports[0].Name)
	assert.Empty(t, failing(a.reports[0]))

	c.ReplicatedTables = constSeries(ctx, 3)
	c.AsyncMetrics["MaxPartCountForPartition"] = constSeries(ctx, 900)
	c.MutationsFailing = constSeries(ctx, 1)
	a.reports = nil
	a.clickhouse()
	assert.ElementsMatch(t,
		[]string{"ClickHouse replication", "ClickHouse parts", "ClickHouse mutations"},
		failing(a.reports[0]))

	c.Up = constSeries(ctx, 0)
	a.reports = nil
	a.clickhouse()
	assert.Equal(t, []string{"ClickHouse availability"}, failing(a.reports[0]), "a server that is down only fails availability")
}
