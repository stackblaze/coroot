package watchers

import (
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
)

// The fixtures use a 1-minute step over a 20-minute window (t=60..1200);
// "now" is the last sample. newIncidentRolloutSettle is 5 minutes and
// newIncidentRecentBadWindow is 3 minutes.
func TestSuppressNewIncident(t *testing.T) {
	step := timeseries.Minute
	from := timeseries.Time(60)
	points := 20
	now := from.Add(timeseries.Duration(points-1) * step)

	newApp := func() *model.Application {
		return model.NewApplication(model.NewApplicationId("", "default", model.ApplicationKindDeployment, "docuseal-kuberoapp-web"))
	}
	addPod := func(app *model.Application, name string, aliveFrom, aliveTo int) {
		data := make([]float32, points)
		for i := aliveFrom; i <= aliveTo && i < points; i++ {
			data[i] = 1
		}
		i := app.GetOrCreateInstance(name, nil)
		i.Pod = &model.Pod{}
		i.Pod.LifeSpan = timeseries.NewWithData(from, step, data)
	}
	// badAt: a sumFromFunc with failures at the given minute offsets.
	badAt := func(offsets ...int) sumFromFunc {
		return func(f timeseries.Time) float32 {
			var sum float32
			for _, o := range offsets {
				if ts := from.Add(timeseries.Duration(o) * step); !ts.Before(f) {
					sum++
				}
			}
			return sum
		}
	}

	// Fresh rollout: the only pod started 2 minutes ago and its boot-window
	// probe failures are recent — settling, no incident.
	app := newApp()
	addPod(app, "new", 17, 19)
	assert.True(t, suppressNewIncident(app, now, badAt(17), nil))

	// Recreate: predecessor died 10 minutes ago, replacement is 2 minutes
	// old — still settling even though the teardown noise is in the windows.
	app = newApp()
	addPod(app, "old", 0, 9)
	addPod(app, "new", 17, 19)
	assert.True(t, suppressNewIncident(app, now, badAt(9, 17), nil))

	// Established pod, bad events stopped 10 minutes ago: the burn windows
	// are dirty but nothing is wrong NOW — no new incident.
	app = newApp()
	addPod(app, "steady", 0, 19)
	assert.True(t, suppressNewIncident(app, now, badAt(8, 9), nil))

	// Established pod with CURRENT bad events: a real outage, open it.
	app = newApp()
	addPod(app, "steady", 0, 19)
	assert.False(t, suppressNewIncident(app, now, badAt(18, 19), nil))

	// Latency-only badness counts too (availability func nil).
	app = newApp()
	addPod(app, "steady", 0, 19)
	assert.False(t, suppressNewIncident(app, now, nil, badAt(19)))

	// Rolling update of an established app: the old pod is still live, so the
	// app is NOT settling — current bad events open an incident.
	app = newApp()
	addPod(app, "old", 0, 19)
	addPod(app, "new", 17, 19)
	assert.False(t, suppressNewIncident(app, now, badAt(19), nil))

	// A young pod that is past the settle window opens incidents normally: a
	// rollout that is still failing after 5 minutes is a real problem.
	app = newApp()
	addPod(app, "young", 10, 19)
	assert.False(t, suppressNewIncident(app, now, badAt(19), nil))

	// The delete→recreate gap: no live pods, but the predecessor died within
	// the settle window and its teardown probe failures are recent — still a
	// transition, not an outage.
	app = newApp()
	addPod(app, "gone", 0, 16)
	assert.True(t, suppressNewIncident(app, now, badAt(16, 17), nil))

	// Pod-less for LONGER than the settle window: with recent bad events this
	// is a real outage, not a transition.
	app = newApp()
	addPod(app, "long-gone", 0, 10)
	assert.False(t, suppressNewIncident(app, now, badAt(19), nil))
}

func TestAppRolloutSettling(t *testing.T) {
	step := timeseries.Minute
	from := timeseries.Time(60)
	points := 20
	now := from.Add(timeseries.Duration(points-1) * step)

	app := model.NewApplication(model.NewApplicationId("", "default", model.ApplicationKindDeployment, "web"))
	// No instances at all: not settling.
	assert.False(t, appRolloutSettling(app, now))

	// Instance without pod data: not settling.
	app.GetOrCreateInstance("nopod", nil)
	assert.False(t, appRolloutSettling(app, now))
}
