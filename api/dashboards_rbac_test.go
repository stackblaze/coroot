package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/rbac"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

const scopedRole = rbac.RoleName("scoped-viewer")

func scopedRoles(projectId db.ProjectId) staticRoles {
	roles := staticRoles(append([]rbac.Role{}, rbac.Roles...))
	return append(roles, rbac.NewRole(scopedRole,
		rbac.NewPermission(rbac.ScopeApplication, rbac.ActionView, rbac.Object{
			"project_id":            string(projectId),
			"application_namespace": "team-a",
			"application_kind":      "*",
			"application_name":      "*",
		}),
		rbac.NewPermission(rbac.ScopeProjectLogs, rbac.ActionView, rbac.Object{"project_id": string(projectId)}),
		rbac.NewPermission(rbac.ScopeDashboard, rbac.ActionView, rbac.Object{
			"project_id":     string(projectId),
			"dashboard_name": "team-a:*",
		}),
	))
}

func TestCanQueryPanelData(t *testing.T) {
	projectId := db.ProjectId("proj1")
	api := &Api{roles: scopedRoles(projectId)}

	for _, role := range []rbac.RoleName{rbac.RoleAdmin, rbac.RoleEditor, rbac.RoleViewer} {
		assert.True(t, api.canQueryPanelData(&db.User{Roles: []rbac.RoleName{role}}, projectId), role)
	}
	assert.False(t, api.canQueryPanelData(&db.User{Roles: []rbac.RoleName{scopedRole}}, projectId))
	assert.False(t, api.canQueryPanelData(&db.User{}, projectId))

	metricsRole := rbac.RoleName("metrics-viewer")
	dashboardsRole := rbac.RoleName("dashboards-editor")
	api.roles = staticRoles{
		rbac.NewRole(metricsRole, rbac.NewPermission(rbac.ScopeProjectMetrics, rbac.ActionView, rbac.Object{"project_id": string(projectId)})),
		rbac.NewRole(dashboardsRole, rbac.NewPermission(rbac.ScopeDashboards, rbac.ActionEdit, rbac.Object{"project_id": string(projectId)})),
	}
	assert.True(t, api.canQueryPanelData(&db.User{Roles: []rbac.RoleName{metricsRole}}, projectId))
	assert.True(t, api.canQueryPanelData(&db.User{Roles: []rbac.RoleName{dashboardsRole}}, projectId))
	assert.False(t, api.canQueryPanelData(&db.User{Roles: []rbac.RoleName{metricsRole}}, "proj2"))
}

func TestPanelDataRequiresPermission(t *testing.T) {
	projectId := db.ProjectId("proj1")
	api := &Api{roles: scopedRoles(projectId)}
	r := httptest.NewRequest(http.MethodGet, "/api/project/proj1/panel/data?query=%7B%7D", nil)
	r = mux.SetURLVars(r, map[string]string{"project": string(projectId)})
	w := httptest.NewRecorder()
	api.PanelData(w, r, &db.User{Roles: []rbac.RoleName{scopedRole}})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestFilterDashboardsForUser(t *testing.T) {
	projectId := db.ProjectId("proj1")
	api := &Api{roles: scopedRoles(projectId)}
	dashboards := []*db.Dashboard{
		{Id: "1", Name: "team-a:latency"},
		{Id: "2", Name: "team-b:latency"},
		{Id: "3", Name: "cluster overview"},
	}
	names := func(ds []*db.Dashboard) []string {
		var res []string
		for _, d := range ds {
			res = append(res, d.Name)
		}
		return res
	}

	scoped := &db.User{Roles: []rbac.RoleName{scopedRole}}
	assert.Equal(t, []string{"team-a:latency"}, names(api.filterDashboardsForUser(scoped, projectId, dashboards)))
	assert.Empty(t, api.filterDashboardsForUser(scoped, "proj2", dashboards))
	assert.Empty(t, api.filterDashboardsForUser(&db.User{}, projectId, dashboards))

	for _, role := range []rbac.RoleName{rbac.RoleAdmin, rbac.RoleEditor, rbac.RoleViewer} {
		u := &db.User{Roles: []rbac.RoleName{role}}
		assert.Len(t, api.filterDashboardsForUser(u, projectId, dashboards), 3, role)
	}
}
