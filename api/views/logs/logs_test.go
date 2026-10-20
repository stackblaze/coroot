package logs

import (
	"context"
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
)

func TestSelectSource(t *testing.T) {
	s, ok := selectSource("", "")
	assert.True(t, ok)
	assert.Equal(t, model.LogSourceAgent, s)

	s, ok = selectSource("", "svc")
	assert.True(t, ok)
	assert.Equal(t, model.LogSourceOtel, s)

	s, ok = selectSource(model.LogSourceAgent, "svc")
	assert.True(t, ok)
	assert.Equal(t, model.LogSourceAgent, s)

	s, ok = selectSource(model.LogSourceOtel, "")
	assert.True(t, ok)
	assert.Equal(t, model.LogSourceOtel, s)

	for _, bad := range []model.LogSource{"bogus", "AGENT", "agent ", "*"} {
		_, ok = selectSource(bad, "svc")
		assert.False(t, ok, bad)
	}
}

func TestRenderEntriesWithoutServicesReturnsNothing(t *testing.T) {
	app := model.NewApplication(model.NewApplicationId("", "team-a", model.ApplicationKindDeployment, "web"))
	app.GetOrCreateInstance("web-1", nil)
	w := &model.World{Ctx: timeseries.NewContext(0, 3600, timeseries.Minute)}

	for _, tc := range []struct {
		source      model.LogSource
		otelService string
	}{
		{model.LogSourceOtel, ""},      // OTel without a resolved service
		{model.LogSourceAgent, ""},     // instance without containers
		{model.LogSource("bogus"), ""}, // unknown source
	} {
		v := &View{Source: tc.source}
		// A nil ClickHouse client would panic if a query were attempted.
		renderEntries(context.Background(), v, nil, app, w, Query{Limit: 10}, tc.otelService)
		assert.Empty(t, v.Entries, tc.source)
		assert.Nil(t, v.Chart, tc.source)
	}
}
