package tracing

import (
	"sort"
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/stretchr/testify/assert"
)

func TestFilterSpans(t *testing.T) {
	spans := []*model.TraceSpan{{ServiceName: "a"}, {ServiceName: "b"}, {ServiceName: "a"}}
	out := filterSpans(spans, func(s string) bool { return s == "a" })
	assert.Len(t, out, 2)
	for _, s := range out {
		assert.Equal(t, "a", s.ServiceName)
	}
}

func TestGetAuxPodIpsCoversMonitoringAndControlPlane(t *testing.T) {
	w := &model.World{Applications: map[model.ApplicationId]*model.Application{}}
	add := func(ns, name string, category model.ApplicationCategory, ips ...string) {
		app := model.NewApplication(model.NewApplicationId("", ns, model.ApplicationKindDeployment, name))
		app.Category = category
		i := app.GetOrCreateInstance(name+"-0", nil)
		for n, ip := range ips {
			i.TcpListens[model.Listen{IP: ip, Port: []string{"0", "10250", "9100"}[n%3]}] = true
		}
		w.Applications[app.Id] = app
	}
	add("coroot", "coroot-cluster-agent", model.ApplicationCategoryMonitoring, "192.168.205.139", "127.0.0.1")
	add("", "k3s-agent", model.ApplicationCategoryControlPlane, "10.10.33.34", "10.10.33.34", "10.10.33.35")
	add("shop-production", "web", model.ApplicationCategoryApplication, "192.168.2.206")

	got := getAuxPodIps(w)
	sort.Strings(got)
	assert.Equal(t, []string{"10.10.33.34", "10.10.33.35", "192.168.205.139"}, got)
}
