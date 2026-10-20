package rbac

import (
	"github.com/coroot/coroot/utils"
)

type Permission struct {
	Scope  Scope  `json:"scope"`
	Action Verb   `json:"action"`
	Object Object `json:"object"`
}

func NewPermission(scope Scope, action Verb, object Object) Permission {
	return Permission{Scope: scope, Action: action, Object: object}
}

func (p Permission) allows(action Action) bool {
	if !utils.GlobMatch(string(action.Scope), string(p.Scope)) {
		return false
	}
	if !utils.GlobMatch(string(action.Action), string(p.Action)) {
		return false
	}
	for k, av := range action.Object {
		pv, ok := p.Object[k]
		if !ok {
			continue
		}
		if pv == "" {
			return false
		}
		if av == "*" {
			continue
		}
		if !utils.GlobMatch(av, pv) {
			return false
		}
	}
	return true
}

func (p Permission) allowsForObject(action Action) (bool, Object) {
	if !utils.GlobMatch(string(action.Scope), string(p.Scope)) {
		return false, nil
	}
	if !utils.GlobMatch(string(action.Action), string(p.Action)) {
		return false, nil
	}
	object := Object{}
	for k, pv := range p.Object {
		if pv == "*" {
			continue
		}
		_, ok := action.Object[k]
		if ok && pv == "" {
			return false, nil
		}
		if ok {
			object[k] = pv
		}
	}
	if len(object) == 0 {
		object = nil
	}
	return true, object
}

type PermissionSet []Permission

func (ps PermissionSet) Allows(action Action) bool {
	for _, p := range ps {
		if p.allows(action) {
			return true
		}
	}
	return false
}

func (ps PermissionSet) AllowsForObjects(action Action) []Object {
	var objects []Object
	for _, p := range ps {
		ok, obj := p.allowsForObject(action)
		if !ok {
			continue
		}
		if obj == nil {
			return []Object{}
		}
		objects = append(objects, obj)
	}
	return objects
}

var applicationObjectKeys = []string{
	"application_category",
	"application_namespace",
	"application_kind",
	"application_name",
}

// RestrictsApplicationView reports whether the permission set limits which
// applications of the project can be viewed. It is true when any permission
// granting application view in the project is narrowed to specific
// categories, namespaces, kinds or names, or when no permission grants
// project-wide application view. The result depends only on the permissions,
// never on which applications currently exist.
func (ps PermissionSet) RestrictsApplicationView(projectId string) bool {
	projectWide := false
	for _, p := range ps {
		if !utils.GlobMatch(string(ScopeApplication), string(p.Scope)) {
			continue
		}
		if !utils.GlobMatch(string(ActionView), string(p.Action)) {
			continue
		}
		if pid, ok := p.Object["project_id"]; ok && (pid == "" || !utils.GlobMatch(projectId, pid)) {
			continue
		}
		for _, k := range applicationObjectKeys {
			if v, ok := p.Object[k]; ok && v != "*" {
				return true
			}
		}
		projectWide = true
	}
	return !projectWide
}
