package api

import (
	"slices"

	"github.com/coroot/coroot/api/views/logs"
	"github.com/coroot/coroot/api/views/profiling"
	"github.com/coroot/coroot/api/views/tracing"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/rbac"
	"k8s.io/klog"
)

// canReadProjectConfig reports whether the user may read project-wide
// configuration guarded by the given edit action. Users who may edit it keep
// access, and so do users whose application view is not limited (e.g. the
// builtin Viewer). Users limited to some applications get no access, because
// the configuration covers every application of the project.
func (api *Api) canReadProjectConfig(u *db.User, projectId string, edit rbac.Action) bool {
	if api.IsAllowed(u, edit) {
		return true
	}
	return !api.hasRestrictedAppAccess(u, projectId)
}

// hasRestrictedAccessInAnyProject reports whether the user's application view
// is limited in any project they can access. It fails closed.
func (api *Api) hasRestrictedAccessInAnyProject(u *db.User) bool {
	if u == nil {
		return false
	}
	names, err := api.db.GetProjectNames()
	if err != nil {
		klog.Errorln(err)
		return true
	}
	for id := range names {
		if !api.IsAllowed(u, rbac.Actions.Project(string(id)).List()...) {
			continue
		}
		if api.hasRestrictedAppAccess(u, string(id)) {
			return true
		}
	}
	return false
}

// visibleRoles returns the roles the user may list: all of them for users who
// manage users or roles or whose application view is not limited, otherwise
// only the user's own roles.
func (api *Api) visibleRoles(u *db.User, roles []rbac.Role) []rbac.Role {
	if api.IsAllowed(u, rbac.Actions.Roles().Edit(), rbac.Actions.Users().Edit()) || !api.hasRestrictedAccessInAnyProject(u) {
		return roles
	}
	var res []rbac.Role
	for _, r := range roles {
		if slices.Contains(u.Roles, r.Name) {
			res = append(res, r)
		}
	}
	return res
}

// filterCheckConfigs keeps the project-wide defaults and the overrides of
// applications the user may view.
func (api *Api) filterCheckConfigs(u *db.User, project *db.Project, w *model.World, checkConfigs model.CheckConfigs) model.CheckConfigs {
	projectId := string(project.Id)
	if !api.hasRestrictedAppAccess(u, projectId) {
		return checkConfigs
	}
	res := model.CheckConfigs{}
	for appId, cfg := range checkConfigs {
		if appId.IsZero() || api.canViewApplicationId(u, projectId, w, appId, project.CalcApplicationCategory(appId)) {
			res[appId] = cfg
		}
	}
	return res
}

// limitLogServices keeps only the OpenTelemetry service linked to the application.
func limitLogServices(v *logs.View) {
	if v == nil {
		return
	}
	var services []string
	if v.Service != "" && slices.Contains(v.Services, v.Service) {
		services = []string{v.Service}
	}
	v.Services = services
}

// limitTraceServices keeps the service linked to the application and the
// services accepted by allow.
func limitTraceServices(v *tracing.View, allow func(string) bool) {
	if v == nil {
		return
	}
	var services []tracing.Service
	for _, s := range v.Services {
		if s.Linked || (allow != nil && allow(s.Name)) {
			services = append(services, s)
		}
	}
	v.Services = services
}

// limitProfilingServices keeps only the services linked to the application.
func limitProfilingServices(v *profiling.View) {
	if v == nil {
		return
	}
	var services []profiling.Service
	for _, s := range v.Services {
		if s.Linked {
			services = append(services, s)
		}
	}
	v.Services = services
}
