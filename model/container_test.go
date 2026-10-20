package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGuessServiceExact(t *testing.T) {
	web := NewApplication(NewApplicationId("", "team-a", ApplicationKindDeployment, "web"))
	other := NewApplication(NewApplicationId("", "team-b", ApplicationKindDeployment, "shop-web"))
	w := &World{Applications: map[ApplicationId]*Application{web.Id: web}}

	assert.Equal(t, "web", GuessServiceExact([]string{"web", "shop-web"}, w, web))
	// Suffix matches are not used.
	assert.Equal(t, "", GuessServiceExact([]string{"shop-web"}, w, web))
	assert.Equal(t, "", GuessServiceExact([]string{"/k8s/team-b/web"}, w, web))
	assert.Equal(t, "/k8s/team-a/web", GuessServiceExact([]string{"/k8s/team-a/web"}, w, web))
	// The loose guess still matches by suffix (unchanged behaviour).
	assert.Equal(t, "shop-web", GuessService([]string{"shop-web"}, w, web))

	// Ambiguous services are not resolved.
	twin := NewApplication(NewApplicationId("", "team-b", ApplicationKindDeployment, "web"))
	w2 := &World{Applications: map[ApplicationId]*Application{web.Id: web, twin.Id: twin, other.Id: other}}
	assert.Equal(t, "", GuessServiceExact([]string{"web"}, w2, web))
}

func TestRestrictedOtelServices(t *testing.T) {
	app := NewApplication(NewApplicationId("", "team-a", ApplicationKindDeployment, "web"))
	w := &World{Applications: map[ApplicationId]*Application{app.Id: app}}
	assert.Equal(t, "", app.RestrictedOtelLogService([]string{"shop-web"}, w))
	assert.Equal(t, "", app.RestrictedOtelTraceService([]string{"shop-web"}, w))
	app.Settings = &ApplicationSettings{
		Logs:    &ApplicationSettingsLogs{Service: "configured-logs"},
		Tracing: &ApplicationSettingsTracing{Service: "configured-traces"},
	}
	assert.Equal(t, "configured-logs", app.RestrictedOtelLogService(nil, w))
	assert.Equal(t, "configured-traces", app.RestrictedOtelTraceService(nil, w))
}
