package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coroot/coroot/api/views/logs"
	"github.com/coroot/coroot/api/views/profiling"
	"github.com/coroot/coroot/api/views/tracing"
	"github.com/coroot/coroot/config"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/rbac"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newProjectApi(t *testing.T) (*Api, *db.Project) {
	t.Helper()
	database := newTestDB(t)
	p := &db.Project{Name: "p1"}
	require.NoError(t, database.SaveProject(p))
	return &Api{db: database, roles: namespaceRoles(string(p.Id)), cfg: config.NewConfig()}, p
}

func TestProjectConfigReadsRequirePermission(t *testing.T) {
	api, p := newProjectApi(t)
	projectId := string(p.Id)

	handlers := map[string]func(http.ResponseWriter, *http.Request, *db.User){
		"application_categories": api.ApplicationCategories,
		"custom_applications":    api.CustomApplications,
		"custom_cloud_pricing":   api.CustomCloudPricing,
		"integrations":           api.Integrations,
	}
	get := func(name string, u *db.User) int {
		r := httptest.NewRequest(http.MethodGet, "/api/project/"+projectId+"/"+name, nil)
		r = mux.SetURLVars(r, map[string]string{"project": projectId})
		w := httptest.NewRecorder()
		handlers[name](w, r, u)
		return w.Code
	}

	for name := range handlers {
		assert.Equal(t, http.StatusForbidden, get(name, &db.User{Roles: []rbac.RoleName{namespaceRole}}), name)
		for _, u := range builtinUsers() {
			assert.Equal(t, http.StatusOK, get(name, u), name, u.Roles)
		}
	}
}

func TestIntegrationReadRequiresPermission(t *testing.T) {
	api, p := newProjectApi(t)
	get := func(u *db.User) int {
		r := httptest.NewRequest(http.MethodGet, "/api/project/"+string(p.Id)+"/integrations/slack", nil)
		r = mux.SetURLVars(r, map[string]string{"project": string(p.Id), "type": string(db.IntegrationTypeSlack)})
		w := httptest.NewRecorder()
		api.Integration(w, r, u)
		return w.Code
	}
	assert.Equal(t, http.StatusForbidden, get(&db.User{Roles: []rbac.RoleName{namespaceRole}}))
	for _, u := range builtinUsers() {
		assert.Equal(t, http.StatusOK, get(u), u.Roles)
	}
}

func TestRolesListedForRestrictedUser(t *testing.T) {
	api, _ := newProjectApi(t)
	list := func(u *db.User) []string {
		r := httptest.NewRequest(http.MethodGet, "/api/roles", nil)
		w := httptest.NewRecorder()
		api.Roles(w, r, u)
		require.Equal(t, http.StatusOK, w.Code)
		var res struct {
			Roles []struct {
				Name string `json:"name"`
			} `json:"roles"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		var names []string
		for _, role := range res.Roles {
			names = append(names, role.Name)
		}
		return names
	}

	assert.Equal(t, []string{string(namespaceRole)}, list(&db.User{Roles: []rbac.RoleName{namespaceRole}}))
	assert.Len(t, list(&db.User{Roles: []rbac.RoleName{rbac.RoleAdmin}}), 4)
	// Users who cannot manage users or roles see only their own roles.
	assert.Equal(t, []string{string(rbac.RoleEditor)}, list(&db.User{Roles: []rbac.RoleName{rbac.RoleEditor}}))
	assert.Equal(t, []string{string(rbac.RoleViewer)}, list(&db.User{Roles: []rbac.RoleName{rbac.RoleViewer}}))
	assert.Empty(t, list(&db.User{Roles: []rbac.RoleName{"unknown"}}))
}

func TestRolesListedForRoleAndUserManagers(t *testing.T) {
	api, _ := newProjectApi(t)
	roles := api.roles.(staticRoles)
	roles = append(roles,
		rbac.NewRole("role-manager", rbac.NewPermission(rbac.ScopeRoles, rbac.ActionEdit, nil)),
		rbac.NewRole("user-manager", rbac.NewPermission(rbac.ScopeUsers, rbac.ActionEdit, nil)),
	)
	api.roles = roles
	for _, name := range []rbac.RoleName{"role-manager", "user-manager"} {
		r := httptest.NewRequest(http.MethodGet, "/api/roles", nil)
		w := httptest.NewRecorder()
		api.Roles(w, r, &db.User{Roles: []rbac.RoleName{name}})
		require.Equal(t, http.StatusOK, w.Code)
		var res struct {
			Roles []struct {
				Name string `json:"name"`
			} `json:"roles"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		assert.Len(t, res.Roles, len(roles), name)
	}
}

func TestSSORolesListedForRestrictedUser(t *testing.T) {
	api, _ := newProjectApi(t)
	list := func(u *db.User) []rbac.RoleName {
		r := httptest.NewRequest(http.MethodGet, "/api/sso", nil)
		w := httptest.NewRecorder()
		api.SSO(w, r, u)
		require.Equal(t, http.StatusOK, w.Code)
		var res struct {
			Roles []rbac.RoleName `json:"roles"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		return res.Roles
	}
	assert.Equal(t, []rbac.RoleName{namespaceRole}, list(&db.User{Roles: []rbac.RoleName{namespaceRole}}))
	assert.Len(t, list(&db.User{Roles: []rbac.RoleName{rbac.RoleViewer}}), 4)
}

func TestProjectSettingsForRestrictedUser(t *testing.T) {
	api, p := newProjectApi(t)
	other := &db.Project{Name: "tenant-b"}
	require.NoError(t, api.db.SaveProject(other))

	get := func(projectId string, u *db.User) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/api/project/"+projectId, nil)
		r = mux.SetURLVars(r, map[string]string{"project": projectId})
		w := httptest.NewRecorder()
		api.Project(w, r, u)
		return w
	}

	scoped := &db.User{Roles: []rbac.RoleName{namespaceRole}}
	w := get(string(p.Id), scoped)
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "tenant-b")
	assert.Contains(t, w.Body.String(), `"name":"p1"`)

	// A project the role does not cover is not readable.
	assert.Equal(t, http.StatusForbidden, get(string(other.Id), scoped).Code)

	// The project list is empty for the restricted user.
	assert.NotContains(t, get("", scoped).Body.String(), "tenant-b")

	for _, u := range builtinUsers() {
		w = get(string(p.Id), u)
		require.Equal(t, http.StatusOK, w.Code, u.Roles)
		assert.Contains(t, w.Body.String(), "tenant-b", u.Roles)
	}
}

func TestFilterCheckConfigs(t *testing.T) {
	api, p := newProjectApi(t)
	own := model.NewApplicationId("", "team-a", model.ApplicationKindDeployment, "web")
	other := model.NewApplicationId("", "team-b", model.ApplicationKindDeployment, "web")
	cfg := json.RawMessage(`{}`)
	configs := model.CheckConfigs{
		model.ApplicationIdZero: {model.Checks.SLOAvailability.Id: cfg},
		own:                     {model.Checks.SLOAvailability.Id: cfg},
		other:                   {model.Checks.SLOAvailability.Id: cfg},
	}

	res := api.filterCheckConfigs(&db.User{Roles: []rbac.RoleName{namespaceRole}}, p, &model.World{}, configs)
	assert.Len(t, res, 2)
	assert.Contains(t, res, model.ApplicationIdZero)
	assert.Contains(t, res, own)
	assert.NotContains(t, res, other)
	assert.Len(t, configs, 3, "the input is not modified")

	for _, u := range builtinUsers() {
		assert.Len(t, api.filterCheckConfigs(u, p, &model.World{}, configs), 3, u.Roles)
	}
}

func TestLimitServiceLists(t *testing.T) {
	lv := &logs.View{Service: "web", Services: []string{"other", "web", "zzz"}}
	limitLogServices(lv)
	assert.Equal(t, []string{"web"}, lv.Services)

	lv = &logs.View{Services: []string{"other", "web"}}
	limitLogServices(lv)
	assert.Empty(t, lv.Services)

	tv := &tracing.View{Services: []tracing.Service{{Name: "other"}, {Name: "web", Linked: true}, {Name: "worker"}}}
	limitTraceServices(tv, func(s string) bool { return s == "worker" })
	assert.Equal(t, []tracing.Service{{Name: "web", Linked: true}, {Name: "worker"}}, tv.Services)

	tv = &tracing.View{Services: []tracing.Service{{Name: "other"}, {Name: "web", Linked: true}}}
	limitTraceServices(tv, nil)
	assert.Equal(t, []tracing.Service{{Name: "web", Linked: true}}, tv.Services)

	pv := &profiling.View{Services: []profiling.Service{{Name: "other"}, {Name: "web", Linked: true}}}
	limitProfilingServices(pv)
	assert.Equal(t, []profiling.Service{{Name: "web", Linked: true}}, pv.Services)

	limitLogServices(nil)
	limitTraceServices(nil, nil)
	limitProfilingServices(nil)
}

func TestPromProxyRequiresPermission(t *testing.T) {
	api, p := newProjectApi(t)
	r := httptest.NewRequest(http.MethodGet, "/api/project/"+string(p.Id)+"/prom/api/v1/label/namespace/values", nil)
	r = mux.SetURLVars(r, map[string]string{"project": string(p.Id), "rest": "label/namespace/values"})
	w := httptest.NewRecorder()
	api.Prom(w, r, &db.User{Roles: []rbac.RoleName{namespaceRole}})
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.True(t, strings.Contains(w.Body.String(), "not allowed"))
}
