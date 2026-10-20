package api

import (
	"fmt"

	"github.com/coroot/coroot/api/views/overview"
	"github.com/coroot/coroot/cache"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/rbac"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

type DataWithContext struct {
	Context Context `json:"context"`
	Data    any     `json:"data"`
}

type Context struct {
	Status         Status                            `json:"status"`
	Search         Search                            `json:"search"`
	Incidents      map[model.ApplicationCategory]int `json:"incidents"`
	Alerts         map[string]int                    `json:"alerts"`
	Fluxcd         *GitOpsStatus                     `json:"fluxcd"`
	Argocd         *GitOpsStatus                     `json:"argocd"`
	License        *License                          `json:"license,omitempty"`
	Multicluster   bool                              `json:"multicluster"`
	MemberProjects []string                          `json:"member_projects,omitempty"`
}

type GitOpsStatus struct {
	Issues int `json:"issues"`
}

type Status struct {
	Status           model.Status      `json:"status"`
	Error            string            `json:"error"`
	Prometheus       Prometheus        `json:"prometheus"`
	NodeAgent        NodeAgent         `json:"node_agent"`
	KubeStateMetrics *KubeStateMetrics `json:"kube_state_metrics"`
}

type Prometheus struct {
	Status  model.Status `json:"status"`
	Message string       `json:"message"`
	Error   string       `json:"error"`
	Action  string       `json:"action"`
}

type NodeAgent struct {
	Status model.Status `json:"status"`
	Nodes  int          `json:"nodes"`
}

type KubeStateMetrics struct {
	Status       model.Status `json:"status"`
	Applications int          `json:"applications"`
}

type Search struct {
	Applications []Application `json:"applications"`
	Nodes        []Node        `json:"nodes"`
}

type Application struct {
	Id model.ApplicationId `json:"id"`
}

type Node struct {
	Name string `json:"name"`
}

type License struct {
	Invalid bool   `json:"invalid"`
	Message string `json:"message"`
}

type LicenseManager interface {
	CheckLicense() *License
}

func (api *Api) WithContext(u *db.User, p *db.Project, cacheStatus *cache.Status, w *model.World, data any) DataWithContext {
	if p == nil {
		return DataWithContext{}
	}
	alerts := api.renderAlertCounts(u, string(p.Id), w)
	gitOpsVisible := !api.hasRestrictedAppAccess(u, string(p.Id))
	res := DataWithContext{
		Context: Context{
			Status:         renderStatus(p, cacheStatus, w, api.globalPrometheus),
			Search:         api.renderSearch(u, string(p.Id), w),
			Incidents:      api.renderIncidents(u, string(p.Id), w),
			Alerts:         alerts,
			Fluxcd:         gitOpsStatus(w, gitOpsVisible && w != nil && w.Flux != nil, overview.CountFluxIssues),
			Argocd:         gitOpsStatus(w, gitOpsVisible && w != nil && w.ArgoCD != nil, overview.CountArgoCDIssues),
			Multicluster:   p.Multicluster(),
			MemberProjects: p.Settings.MemberProjects,
		},
		Data: data,
	}
	if lm := api.licenseMgr; lm != nil {
		if l := lm.CheckLicense(); l != nil {
			res.Context.License = l
			if l.Invalid {
				res.Data = nil
			}
		}
	}
	return res
}

func (api *Api) canViewApplication(u *db.User, projectId string, app *model.Application) bool {
	if app == nil {
		return false
	}
	if u == nil {
		return true
	}
	return api.IsAllowed(u, rbac.Actions.Project(projectId).Application(app.Category, app.Id.Namespace, app.Id.Kind, app.Id.Name).View())
}

func (api *Api) canViewAlert(u *db.User, projectId string, w *model.World, a *model.Alert) bool {
	if a == nil {
		return false
	}
	return api.canViewApplicationId(u, projectId, w, a.ApplicationId, a.ApplicationCategory)
}

// canViewApplicationId checks application view permission by id. The
// application is looked up in w when present; otherwise the permission is
// checked against the id and the given category.
func (api *Api) canViewApplicationId(u *db.User, projectId string, w *model.World, id model.ApplicationId, category model.ApplicationCategory) bool {
	if u == nil {
		return true
	}
	if w != nil {
		if app := w.GetApplication(id); app != nil {
			return api.canViewApplication(u, projectId, app)
		}
	}
	return api.IsAllowed(u, rbac.Actions.Project(projectId).Application(category, id.Namespace, id.Kind, id.Name).View())
}

func (api *Api) renderAlertCounts(u *db.User, projectId string, w *model.World) map[string]int {
	if api.db == nil {
		return map[string]int{}
	}
	if !api.hasRestrictedAppAccess(u, projectId) {
		alerts, _ := api.db.GetFiringAlertCountsBySeverity(db.ProjectId(projectId))
		if alerts == nil {
			return map[string]int{}
		}
		return alerts
	}
	result, err := api.db.QueryAlerts(db.ProjectId(projectId), db.AlertsQuery{Limit: 10000})
	if err != nil || result == nil {
		return map[string]int{}
	}
	counts := map[string]int{}
	for _, a := range result.Alerts {
		if !api.canViewAlert(u, projectId, w, a) {
			continue
		}
		counts[a.Severity.String()]++
	}
	return counts
}

// filterAlertsResult keeps only alerts the user may view and re-applies pagination.
func (api *Api) filterAlertsResult(u *db.User, projectId string, w *model.World, result *db.AlertsResult, includeResolved bool, limit, offset int) *db.AlertsResult {
	if result == nil {
		return result
	}
	filtered := make([]*model.Alert, 0, len(result.Alerts))
	firing, resolved := 0, 0
	for _, a := range result.Alerts {
		if !api.canViewAlert(u, projectId, w, a) {
			continue
		}
		filtered = append(filtered, a)
		if a.ResolvedAt == 0 && a.ManuallyResolvedAt == 0 && !a.Suppressed {
			firing++
		} else {
			resolved++
		}
	}
	out := &db.AlertsResult{Firing: firing, Resolved: resolved}
	if includeResolved {
		out.Total = firing + resolved
	} else {
		out.Total = firing
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(filtered) {
		offset = len(filtered)
	}
	filtered = filtered[offset:]
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	out.Alerts = filtered
	return out
}

// hasRestrictedAppAccess is true when the user's roles limit which
// applications of the project they may view (e.g. namespace-scoped roles).
// It is derived from role permissions only, so the answer does not depend on
// which applications happen to be present in the requested time window.
// It fails closed: if roles cannot be loaded the user is treated as restricted.
func (api *Api) hasRestrictedAppAccess(u *db.User, projectId string) bool {
	if u == nil {
		return false
	}
	roles, err := api.roles.GetRoles()
	if err != nil {
		klog.Errorln(err)
		return true
	}
	var permissions rbac.PermissionSet
	for _, rn := range u.Roles {
		for _, role := range roles {
			if role.Name == rn {
				permissions = append(permissions, role.Permissions...)
			}
		}
	}
	return permissions.RestrictsApplicationView(projectId)
}

// worldWithViewableApps returns a shallow copy of w whose Applications map
// contains only apps the user may view.
func (api *Api) worldWithViewableApps(u *db.User, projectId string, w *model.World) *model.World {
	if w == nil {
		return nil
	}
	cp := *w
	cp.Applications = map[model.ApplicationId]*model.Application{}
	for id, app := range w.Applications {
		if api.canViewApplication(u, projectId, app) {
			cp.Applications[id] = app
		}
	}
	return &cp
}

// consumerVisibility returns the filter used to name applications in node
// consumer charts: nil (everything) for project-wide users, otherwise only
// the applications the user may view.
func (api *Api) consumerVisibility(u *db.User, projectId string) func(*model.Application) bool {
	if !api.hasRestrictedAppAccess(u, projectId) {
		return nil
	}
	return func(app *model.Application) bool {
		return api.canViewApplication(u, projectId, app)
	}
}

func (api *Api) canViewNode(u *db.User, projectId, nodeName string) bool {
	if u == nil {
		return true
	}
	return api.IsAllowed(u, rbac.Actions.Project(projectId).Node(nodeName).View())
}

func (api *Api) filterOverviewForUser(u *db.User, projectId string, w *model.World, ov *overview.Overview) *overview.Overview {
	if ov == nil || u == nil || w == nil {
		return ov
	}
	if len(ov.Nodes) > 0 {
		filtered := make([]overview.Node, 0, len(ov.Nodes))
		for _, n := range ov.Nodes {
			if api.canViewNode(u, projectId, n.Name) {
				filtered = append(filtered, n)
			}
		}
		ov.Nodes = filtered
	}
	if len(ov.Deployments) > 0 {
		filtered := make([]*overview.Deployment, 0, len(ov.Deployments))
		for _, d := range ov.Deployments {
			if api.canViewApplication(u, projectId, w.GetApplication(d.Application.Id)) {
				filtered = append(filtered, d)
			}
		}
		ov.Deployments = filtered
	}
	if api.hasRestrictedAppAccess(u, projectId) {
		// GitOps resources span namespaces and are not mapped to applications.
		ov.FluxCD = nil
		ov.ArgoCD = nil
	}
	if len(ov.Applications) > 0 {
		filtered := make([]*overview.ApplicationStatus, 0, len(ov.Applications))
		for _, a := range ov.Applications {
			app := w.GetApplication(a.Id)
			if api.canViewApplication(u, projectId, app) {
				filtered = append(filtered, a)
			}
		}
		ov.Applications = filtered
	}
	if len(ov.Map) > 0 {
		filtered := make([]*overview.Application, 0, len(ov.Map))
		for _, a := range ov.Map {
			app := w.GetApplication(a.Id)
			if api.canViewApplication(u, projectId, app) {
				filtered = append(filtered, a)
			}
		}
		ov.Map = filtered
	}
	if len(ov.Risks) > 0 {
		filtered := make([]*overview.Risk, 0, len(ov.Risks))
		for _, r := range ov.Risks {
			app := w.GetApplication(r.ApplicationId)
			if api.canViewApplication(u, projectId, app) {
				filtered = append(filtered, r)
			}
		}
		ov.Risks = filtered
	}
	return ov
}

func (api *Api) renderIncidents(u *db.User, projectId string, w *model.World) map[model.ApplicationCategory]int {
	res := map[model.ApplicationCategory]int{}
	if w == nil {
		return res
	}
	for _, app := range w.Applications {
		if !api.canViewApplication(u, projectId, app) {
			continue
		}
		if len(app.Incidents) == 0 {
			continue
		}
		if last := app.Incidents[len(app.Incidents)-1]; !last.Resolved() {
			res[app.Category]++
		}
	}
	return res
}

func gitOpsStatus(w *model.World, present bool, count func(*model.World) int) *GitOpsStatus {
	if !present {
		return nil
	}
	return &GitOpsStatus{Issues: count(w)}
}

func renderStatus(p *db.Project, cacheStatus *cache.Status, w *model.World, globalPrometheus *db.IntegrationPrometheus) Status {
	res := Status{
		Status: model.OK,
	}

	if p == nil {
		res.Status = model.WARNING
		res.Error = "Project not found"
		return res
	}

	res.Prometheus.Status = model.OK
	res.Prometheus.Message = "ok"
	promCfg := p.PrometheusConfig(globalPrometheus)
	refreshInterval := promCfg.RefreshInterval
	if refreshInterval < cache.MinRefreshInterval {
		refreshInterval = cache.MinRefreshInterval
	}
	switch {
	case promCfg.Url == "" && !promCfg.UseClickHouse && !p.Multicluster():
		res.Prometheus.Status = model.WARNING
		res.Prometheus.Message = "Prometheus is not configured."
		res.Prometheus.Action = "configure"
	case cacheStatus != nil && cacheStatus.Error != "":
		res.Prometheus.Status = model.WARNING
		res.Prometheus.Message = "An error has been occurred while querying Prometheus:"
		res.Prometheus.Error = cacheStatus.Error
		res.Prometheus.Action = "configure"
	case cacheStatus != nil && cacheStatus.LagMax > 5*refreshInterval:
		lag := utils.FormatDuration(cacheStatus.LagAvg, 1)
		res.Prometheus.Status = model.WARNING
		res.Prometheus.Message = fmt.Sprintf("The Prometheus cache lag is %s, likely due to a restart or upgrade. Synchronization is in progress.", lag)
		res.Prometheus.Action = "wait"
	}

	if res.Prometheus.Status >= model.WARNING {
		res.Status = model.WARNING
	}

	if w == nil {
		return res
	}

	is := w.IntegrationStatus
	if !is.NodeAgent.Installed {
		res.NodeAgent.Status = model.WARNING
		res.Status = model.WARNING
	} else {
		res.NodeAgent.Status = model.OK
		res.NodeAgent.Nodes = len(w.Nodes)
	}

	if is.KubeStateMetrics.Required {
		res.KubeStateMetrics = &KubeStateMetrics{}
		if is.KubeStateMetrics.Installed {
			res.KubeStateMetrics.Status = model.OK
			res.KubeStateMetrics.Applications = len(w.Applications) // TODO: count k8s apps only,
		} else {
			res.KubeStateMetrics.Status = model.WARNING
			res.Status = model.WARNING
		}
	}

	return res
}

func (api *Api) renderSearch(u *db.User, projectId string, w *model.World) Search {
	search := Search{}
	if w == nil {
		return search
	}
	for _, app := range w.Applications {
		if !api.canViewApplication(u, projectId, app) {
			continue
		}
		search.Applications = append(search.Applications, Application{
			Id: app.Id,
		})
	}
	for _, node := range w.Nodes {
		name := node.GetName()
		if !api.canViewNode(u, projectId, name) {
			continue
		}
		search.Nodes = append(search.Nodes, Node{
			Name: name,
		})
	}
	return search
}
