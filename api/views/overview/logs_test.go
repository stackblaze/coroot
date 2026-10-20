package overview

import (
	"context"
	"testing"
	"time"

	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/stretchr/testify/assert"
)

func TestServiceBelongsToNamespaces(t *testing.T) {
	ns := map[string]bool{"twenty-production": true}

	assert.True(t, serviceBelongsToNamespaces("/k8s/twenty-production/twenty-kuberoapp-web", ns))
	assert.True(t, serviceBelongsToNamespaces("/k8s/twenty-production/twenty-kuberoapp-web-abc12-xyz99/web", ns))
	assert.True(t, serviceBelongsToNamespaces("/k8s-cronjob/twenty-production/backup", ns))
	assert.False(t, serviceBelongsToNamespaces("twenty-kuberoapp-web", ns))
	assert.False(t, serviceBelongsToNamespaces("/k8s/test-production/chi-clickhouse", ns))
	assert.False(t, serviceBelongsToNamespaces("/k8s/kube-system/coredns", ns))
	assert.False(t, serviceBelongsToNamespaces("/k8s/twenty-production/", ns))
	assert.False(t, serviceBelongsToNamespaces("/k8s/twenty-production", ns))
	assert.False(t, serviceBelongsToNamespaces("", ns))

	// Whole-segment matching only.
	shop := map[string]bool{"shop-prod": true}
	assert.False(t, serviceBelongsToNamespaces("/k8s/myshop-prod/x", shop))
	assert.False(t, serviceBelongsToNamespaces("/k8s/shop-prod-2/x", shop))
	assert.False(t, serviceBelongsToNamespaces("myshop-prod-api", shop))
	assert.True(t, serviceBelongsToNamespaces("api.shop-prod", shop))
	assert.True(t, serviceBelongsToNamespaces("shop-prod/api", shop))

	web := map[string]bool{"web": true}
	assert.False(t, serviceBelongsToNamespaces("/k8s/webshop/frontend", web))
	assert.False(t, serviceBelongsToNamespaces("webshop", web))
	// A namespace name in the workload segment does not count.
	assert.False(t, serviceBelongsToNamespaces("/k8s/other/web", web))
}

func TestClampSince(t *testing.T) {
	ctx := timeseries.NewContext(timeseries.Time(1000), timeseries.Time(2000), timeseries.Minute)
	assert.Equal(t, ctx.From.ToStandard(), ClampSince(time.Unix(1, 0), ctx))
	inside := time.Unix(1500, 0)
	assert.Equal(t, inside, ClampSince(inside, ctx))
}

func TestClampTsRange(t *testing.T) {
	ctx := timeseries.NewContext(timeseries.Time(1000), timeseries.Time(2000), timeseries.Minute)
	from, to := ClampTsRange(1, 999999, ctx)
	assert.Equal(t, ctx.From, from)
	assert.Equal(t, ctx.To, to)
	from, to = ClampTsRange(1200, 1800, ctx)
	assert.Equal(t, timeseries.Time(1200), from)
	assert.Equal(t, timeseries.Time(1800), to)
	from, to = ClampTsRange(1800, 1200, ctx)
	assert.Equal(t, ctx.From, from)
	assert.Equal(t, ctx.To, to)
}

func TestFilterSpansByService(t *testing.T) {
	spans := []*model.TraceSpan{
		{ServiceName: "/k8s/team-a/api"},
		{ServiceName: "/k8s/team-b/db"},
		{ServiceName: "frontend"},
	}
	out := FilterSpansByService(spans, utils.NewStringSet("/k8s/team-a/api", "frontend"))
	assert.Len(t, out, 2)
	for _, s := range out {
		assert.NotEqual(t, "/k8s/team-b/db", s.ServiceName)
	}
	assert.Empty(t, FilterSpansByService([]*model.TraceSpan{{ServiceName: "x"}}, utils.NewStringSet()))
}

func TestLogServicesForEmptyWorld(t *testing.T) {
	ctx := context.Background()
	assert.Empty(t, logServicesForApps(ctx, clickhouse.Clients{}, &model.World{}, nil))
	assert.Empty(t, traceServicesForApps(ctx, clickhouse.Clients{}, &model.World{}, nil))
	assert.Empty(t, logServicesForApps(ctx, clickhouse.Clients{}, nil, nil))
}
