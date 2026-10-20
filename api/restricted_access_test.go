package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coroot/coroot/api/views/overview"
	"github.com/coroot/coroot/config"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/rbac"
	"github.com/coroot/coroot/utils"
	"github.com/gorilla/mux"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const namespaceRole = rbac.RoleName("namespace-viewer")

// namespaceRoles returns the builtin roles plus a role that can view the
// applications of a single namespace together with project-level logs,
// traces, alerts and alerting rules.
func namespaceRoles(projectId string) staticRoles {
	roles := staticRoles(append([]rbac.Role{}, rbac.Roles...))
	return append(roles, rbac.NewRole(namespaceRole,
		rbac.NewPermission(rbac.ScopeApplication, rbac.ActionView, rbac.Object{
			"project_id":            projectId,
			"application_namespace": "team-a",
			"application_kind":      "*",
			"application_name":      "*",
		}),
		rbac.NewPermission(rbac.ScopeProjectLogs, rbac.ActionView, rbac.Object{"project_id": projectId}),
		rbac.NewPermission(rbac.ScopeProjectTraces, rbac.ActionView, rbac.Object{"project_id": projectId}),
		rbac.NewPermission(rbac.ScopeProjectAlerts, rbac.ActionView, rbac.Object{"project_id": projectId}),
		rbac.NewPermission(rbac.ScopeProjectAlertingRules, rbac.ActionView, rbac.Object{"project_id": projectId}),
		rbac.NewPermission(rbac.ScopeProjectAlertingRules, rbac.ActionEdit, rbac.Object{"project_id": projectId}),
	))
}

func builtinUsers() []*db.User {
	return []*db.User{
		{Roles: []rbac.RoleName{rbac.RoleAdmin}},
		{Roles: []rbac.RoleName{rbac.RoleEditor}},
		{Roles: []rbac.RoleName{rbac.RoleViewer}},
	}
}

func TestRestrictedAccessDoesNotDependOnWorld(t *testing.T) {
	projectId := "proj1"
	api := &Api{roles: namespaceRoles(projectId)}
	scoped := &db.User{Roles: []rbac.RoleName{namespaceRole}}

	// Restriction comes from the role, so it holds regardless of the world
	// loaded for the requested time window.
	assert.True(t, api.hasRestrictedAppAccess(scoped, projectId))

	empty := &model.World{Applications: map[model.ApplicationId]*model.Application{}}
	namespaces, restricted := api.restrictedNamespaces(scoped, projectId, empty)
	assert.True(t, restricted)
	assert.Equal(t, []string{"team-a"}, namespaces)
	namespaces, restricted = api.restrictedNamespaces(scoped, projectId, nil)
	assert.True(t, restricted)
	assert.Equal(t, []string{"team-a"}, namespaces)

	// No roles at all: restricted.
	assert.True(t, api.hasRestrictedAppAccess(&db.User{}, projectId))

	for _, u := range builtinUsers() {
		assert.False(t, api.hasRestrictedAppAccess(u, projectId), u.Roles)
		_, restricted := api.restrictedNamespaces(u, projectId, empty)
		assert.False(t, restricted, u.Roles)
	}
}

func TestRestrictedAccessFailsClosedOnRoleError(t *testing.T) {
	api := &Api{roles: failingRoles{}}
	assert.True(t, api.hasRestrictedAppAccess(&db.User{Roles: []rbac.RoleName{rbac.RoleAdmin}}, "proj1"))
}

type failingRoles struct{}

func (failingRoles) GetRoles() ([]rbac.Role, error) { return nil, assert.AnError }

func TestAlertingRulesFilteredWithEmptyWorld(t *testing.T) {
	projectId := "proj1"
	api := &Api{roles: namespaceRoles(projectId)}
	scoped := &db.User{Roles: []rbac.RoleName{namespaceRole}}

	own := &model.AlertingRule{Id: "own", Selector: model.AppSelector{
		Type: model.AppSelectorTypeApplications, ApplicationIdPatterns: []string{"team-a:*:*"},
	}}
	other := &model.AlertingRule{Id: "other", Selector: model.AppSelector{
		Type: model.AppSelectorTypeApplications, ApplicationIdPatterns: []string{"team-b:*:*"},
	}}
	all := &model.AlertingRule{Id: "all", Selector: model.AppSelector{Type: model.AppSelectorTypeAll}}
	rules := []*model.AlertingRule{own, other, all}

	for _, w := range []*model.World{nil, {}} {
		filtered := api.filterAlertingRules(scoped, projectId, w, rules)
		require.Len(t, filtered, 1)
		assert.Equal(t, model.AlertingRuleId("own"), filtered[0].Id)

		err := api.validateAlertingRuleForUser(scoped, projectId, w, &model.AlertingRule{
			Source:   model.AlertSource{Type: model.AlertSourceTypePromQL},
			Selector: model.AppSelector{Type: model.AppSelectorTypeApplications, ApplicationIdPatterns: []string{"team-a:*:*"}},
		})
		assert.Error(t, err)
		err = api.validateAlertingRuleForUser(scoped, projectId, w, &model.AlertingRule{
			Source:   model.AlertSource{Type: model.AlertSourceTypeCheck},
			Selector: model.AppSelector{Type: model.AppSelectorTypeApplications, ApplicationIdPatterns: []string{"team-b:*:*"}},
		})
		assert.Error(t, err)
	}

	for _, u := range builtinUsers() {
		assert.Len(t, api.filterAlertingRules(u, projectId, nil, rules), 3, u.Roles)
	}
}

func TestCanViewAlertOutsideWorld(t *testing.T) {
	projectId := "proj1"
	api := &Api{roles: namespaceRoles(projectId)}
	scoped := &db.User{Roles: []rbac.RoleName{namespaceRole}}
	own := &model.Alert{ApplicationId: model.NewApplicationId(projectId, "team-a", model.ApplicationKindDeployment, "web")}
	other := &model.Alert{ApplicationId: model.NewApplicationId(projectId, "team-b", model.ApplicationKindDeployment, "web")}

	for _, w := range []*model.World{nil, {}} {
		assert.True(t, api.canViewAlert(scoped, projectId, w, own))
		assert.False(t, api.canViewAlert(scoped, projectId, w, other))
		for _, u := range builtinUsers() {
			assert.True(t, api.canViewAlert(u, projectId, w, other), u.Roles)
		}
	}
	assert.False(t, api.canViewAlert(scoped, projectId, nil, nil))
}

func TestFilterOverviewNodesDeploymentsGitOps(t *testing.T) {
	projectId := "proj1"
	api := &Api{roles: namespaceRoles(projectId)}
	scoped := &db.User{Roles: []rbac.RoleName{namespaceRole}}

	ownId := model.NewApplicationId(projectId, "team-a", model.ApplicationKindDeployment, "web")
	otherId := model.NewApplicationId(projectId, "team-b", model.ApplicationKindDeployment, "web")
	w := &model.World{Applications: map[model.ApplicationId]*model.Application{
		ownId:   model.NewApplication(ownId),
		otherId: model.NewApplication(otherId),
	}}
	newOverview := func() *overview.Overview {
		return &overview.Overview{
			Nodes: []overview.Node{{Name: "node-1"}, {Name: "node-2"}},
			Deployments: []*overview.Deployment{
				{Application: overview.Application{Id: ownId}},
				{Application: overview.Application{Id: otherId}},
			},
			FluxCD: []*overview.FluxCDResource{{Name: "release"}},
			ArgoCD: []*overview.ArgoCDResource{{Name: "app"}},
		}
	}

	ov := api.filterOverviewForUser(scoped, projectId, w, newOverview())
	assert.Empty(t, ov.Nodes)
	require.Len(t, ov.Deployments, 1)
	assert.Equal(t, ownId, ov.Deployments[0].Application.Id)
	assert.Nil(t, ov.FluxCD)
	assert.Nil(t, ov.ArgoCD)

	for _, u := range builtinUsers() {
		ov := api.filterOverviewForUser(u, projectId, w, newOverview())
		assert.Len(t, ov.Nodes, 2, u.Roles)
		assert.Len(t, ov.Deployments, 2, u.Roles)
		assert.Len(t, ov.FluxCD, 1, u.Roles)
		assert.Len(t, ov.ArgoCD, 1, u.Roles)
	}
}

func TestConsumerVisibility(t *testing.T) {
	projectId := "proj1"
	api := &Api{roles: namespaceRoles(projectId)}
	scoped := &db.User{Roles: []rbac.RoleName{namespaceRole}}
	own := model.NewApplication(model.NewApplicationId(projectId, "team-a", model.ApplicationKindDeployment, "web"))
	other := model.NewApplication(model.NewApplicationId(projectId, "team-b", model.ApplicationKindDeployment, "web"))

	visible := api.consumerVisibility(scoped, projectId)
	require.NotNil(t, visible)
	assert.True(t, visible(own))
	assert.False(t, visible(other))
	for _, u := range builtinUsers() {
		assert.Nil(t, api.consumerVisibility(u, projectId), u.Roles)
	}
}

func TestMCPAllowedForProjects(t *testing.T) {
	projectId := "proj1"
	api := &Api{roles: namespaceRoles(projectId)}
	ids := []db.ProjectId{db.ProjectId(projectId), "proj2"}

	assert.False(t, api.mcpAllowedForProjects(&db.User{Roles: []rbac.RoleName{namespaceRole}}, ids))
	assert.False(t, api.mcpAllowedForProjects(nil, ids))
	for _, u := range builtinUsers() {
		assert.True(t, api.mcpAllowedForProjects(u, ids), u.Roles)
	}
}

func TestMCPAllowedUsesProjects(t *testing.T) {
	database := newTestDB(t)
	p := &db.Project{Name: "p1"}
	require.NoError(t, database.SaveProject(p))
	api := &Api{db: database, roles: namespaceRoles(string(p.Id))}

	assert.False(t, api.mcpAllowed(&db.User{Roles: []rbac.RoleName{namespaceRole}}))
	for _, u := range builtinUsers() {
		assert.True(t, api.mcpAllowed(u), u.Roles)
	}
}

func TestMCPToolsRejectRestrictedUser(t *testing.T) {
	database := newTestDB(t)
	p := &db.Project{Name: "p1"}
	require.NoError(t, database.SaveProject(p))
	api := &Api{db: database, roles: namespaceRoles(string(p.Id))}
	h := &MCPHandler{Api: api}

	call := func(u *db.User) *mcp.CallToolResult {
		ctx := context.WithValue(context.Background(), mcpUserCtxKey{}, u)
		st := &mcpSessionState{projectId: p.Id}
		_, _, res := h.requireUserAndProjectWithState(ctx, st)
		return res
	}
	res := call(&db.User{Roles: []rbac.RoleName{namespaceRole}})
	require.NotNil(t, res)
	assert.True(t, res.IsError)
	for _, u := range builtinUsers() {
		assert.Nil(t, call(u), u.Roles)
	}
}

func TestMCPResolveLogSourceRejectsUnknownSource(t *testing.T) {
	h := &MCPHandler{Api: &Api{}}
	_, _, res := h.resolveLogSource(context.Background(), nil, &model.World{}, nil, "bogus")
	require.NotNil(t, res)
	assert.True(t, res.IsError)

	source, services, res := h.resolveLogSource(context.Background(), nil, &model.World{}, nil, "")
	assert.Nil(t, res)
	assert.Equal(t, model.LogSourceAgent, source)
	assert.Nil(t, services)
}

func TestRCAAccessRejectsIncidentOfAnotherApp(t *testing.T) {
	api := &Api{roles: namespaceRoles("proj1")}
	appId := model.NewApplicationId("", "team-a", model.ApplicationKindDeployment, "web")
	r := httptest.NewRequest(http.MethodGet, "/api/project/proj1/app/x/rca?incident=abc", nil)
	r = mux.SetURLVars(r, map[string]string{"project": "proj1", "app": appId.String()})
	incident := &model.ApplicationIncident{Key: "abc", ApplicationId: model.NewApplicationId("", "team-b", model.ApplicationKindDeployment, "web")}

	status, _ := api.checkRCAAccess(r, &db.User{Roles: []rbac.RoleName{rbac.RoleAdmin}}, "proj1", incident, 0, 0)
	assert.Equal(t, http.StatusNotFound, status)

	// An incident key that does not resolve is rejected too.
	status, _ = api.checkRCAAccess(r, &db.User{Roles: []rbac.RoleName{rbac.RoleAdmin}}, "proj1", nil, 0, 0)
	assert.Equal(t, http.StatusNotFound, status)
}

func TestSameApplication(t *testing.T) {
	a := model.NewApplicationId("c1", "ns", model.ApplicationKindDeployment, "web")
	assert.True(t, sameApplication(a, model.NewApplicationId("", "ns", model.ApplicationKindDeployment, "web")))
	assert.True(t, sameApplication(a, a))
	assert.False(t, sameApplication(a, model.NewApplicationId("c2", "ns", model.ApplicationKindDeployment, "web")))
	assert.False(t, sameApplication(a, model.NewApplicationId("c1", "other", model.ApplicationKindDeployment, "web")))
}

func TestCanViewApplicationIdOutsideWorld(t *testing.T) {
	api := &Api{roles: namespaceRoles("proj1")}
	scoped := &db.User{Roles: []rbac.RoleName{namespaceRole}}
	own := model.NewApplicationId("", "team-a", model.ApplicationKindDeployment, "web")
	other := model.NewApplicationId("", "team-b", model.ApplicationKindDeployment, "web")
	assert.True(t, api.canViewApplicationId(scoped, "proj1", nil, own, ""))
	assert.False(t, api.canViewApplicationId(scoped, "proj1", nil, other, ""))
	assert.False(t, api.canViewApplicationId(scoped, "proj1", &model.World{}, other, ""))
	for _, u := range builtinUsers() {
		assert.True(t, api.canViewApplicationId(u, "proj1", nil, other, ""), u.Roles)
	}
}

func TestWebhookIntegrationRequiresEdit(t *testing.T) {
	database := newTestDB(t)
	p := &db.Project{Name: "p1"}
	require.NoError(t, database.SaveProject(p))
	p.Settings.Integrations.Webhook = &db.IntegrationWebhook{
		Url:           "https://hooks.example.com/x",
		BasicAuth:     &utils.BasicAuth{User: "user", Password: "secret"},
		CustomHeaders: []utils.Header{{Key: "Authorization", Value: "Bearer token"}},
		CustomFields:  map[string]string{"api_key": "value"},
	}
	require.NoError(t, database.SaveProjectSettings(p))
	api := &Api{db: database, roles: namespaceRoles(string(p.Id)), cfg: config.NewConfig()}

	get := func(u *db.User) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/api/project/"+string(p.Id)+"/integrations/webhook", nil)
		r = mux.SetURLVars(r, map[string]string{"project": string(p.Id), "type": string(db.IntegrationTypeWebhook)})
		w := httptest.NewRecorder()
		api.Integration(w, r, u)
		return w
	}

	for _, u := range []*db.User{
		{Roles: []rbac.RoleName{namespaceRole}},
		{Roles: []rbac.RoleName{rbac.RoleViewer}},
		{Roles: []rbac.RoleName{rbac.RoleEditor}},
	} {
		w := get(u)
		assert.Equal(t, http.StatusForbidden, w.Code, u.Roles)
		assert.NotContains(t, w.Body.String(), "secret", u.Roles)
	}

	w := get(&db.User{Roles: []rbac.RoleName{rbac.RoleAdmin}})
	require.Equal(t, http.StatusOK, w.Code)
	var form db.IntegrationWebhook
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &form))
	assert.Equal(t, "secret", form.BasicAuth.Password)
}

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.NewSqlite(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, database.Migrate())
	return database
}
