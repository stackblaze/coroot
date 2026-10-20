package api

import (
	"regexp"
	"slices"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/rbac"
)

const maxHandoffRoleLength = 64

// handoffRolePattern is the shape of the custom roles managed by the trusted
// handoff caller: a lowercase slug with the "kubero-" prefix.
var handoffRolePattern = regexp.MustCompile(`^kubero-[a-z0-9_]+(-[a-z0-9_]+)*-?$`)

// handoffRoleAllowed reports whether the handoff endpoint may assign the role.
// Handoff only assigns the custom roles it manages, never a builtin role.
func handoffRoleAllowed(role rbac.RoleName) bool {
	if role.Builtin() || len(role) > maxHandoffRoleLength {
		return false
	}
	return handoffRolePattern.MatchString(string(role))
}

// listableRoles returns the roles the user may list: all roles for users who
// manage users or roles, otherwise only the user's own roles.
func (api *Api) listableRoles(u *db.User, roles []rbac.Role) []rbac.Role {
	if api.IsAllowed(u, rbac.Actions.Roles().Edit(), rbac.Actions.Users().Edit()) {
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
