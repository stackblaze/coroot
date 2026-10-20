package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPermissions(t *testing.T) {
	p := NewPermission(ScopeProjectAll, ActionView, nil)
	assert.True(t, p.allows(Actions.Project("*").Node("*").View()))
	assert.True(t, p.allows(Actions.Project("foo").Node("foo").View()))
	assert.True(t, p.allows(Actions.Project("bar").Node("bar").View()))

	p = NewPermission(ScopeProjectAll, ActionView, Object{"project_id": "foo"})
	assert.True(t, p.allows(Actions.Project("*").Node("*").View()))
	assert.True(t, p.allows(Actions.Project("foo").Node("foo").View()))
	assert.False(t, p.allows(Actions.Project("bar").Node("foo").View()))

	p = NewPermission(ScopeNode, ActionView, nil)
	assert.True(t, p.allows(Actions.Project("*").Node("*").View()))
	assert.True(t, p.allows(Actions.Project("foo").Node("foo").View()))
	assert.True(t, p.allows(Actions.Project("bar").Node("bar").View()))

	p = NewPermission(ScopeNode, ActionView, Object{"node_name": "*"})
	assert.True(t, p.allows(Actions.Project("*").Node("*").View()))
	assert.True(t, p.allows(Actions.Project("foo").Node("foo").View()))
	assert.True(t, p.allows(Actions.Project("bar").Node("bar").View()))

	p = NewPermission(ScopeNode, ActionView, Object{"node_name": "foo*"})
	assert.True(t, p.allows(Actions.Project("*").Node("*").View()))
	assert.True(t, p.allows(Actions.Project("foo").Node("foo").View()))
	assert.True(t, p.allows(Actions.Project("foo").Node("foobar").View()))
	assert.False(t, p.allows(Actions.Project("bar").Node("bar").View()))

	p = NewPermission(ScopeNode, ActionView, Object{"node_name": ""})
	assert.False(t, p.allows(Actions.Project("*").Node("*").View()))
	assert.False(t, p.allows(Actions.Project("foo").Node("foo").View()))
	assert.False(t, p.allows(Actions.Project("bar").Node("bar").View()))

	p = NewPermission(ScopeApplication, ActionView, nil)
	assert.False(t, p.allows(Actions.Project("*").Node("*").View()))
	assert.False(t, p.allows(Actions.Project("foo").Node("foo").View()))
	assert.False(t, p.allows(Actions.Project("bar").Node("bar").View()))
}

func TestRestrictsApplicationView(t *testing.T) {
	for _, r := range Roles {
		assert.False(t, r.Permissions.RestrictsApplicationView("p1"), r.Name)
	}

	scoped := PermissionSet{
		NewPermission(ScopeApplication, ActionView, Object{
			"project_id":            "p1",
			"application_namespace": "team-a",
			"application_kind":      "*",
			"application_name":      "*",
		}),
		NewPermission(ScopeProjectLogs, ActionView, Object{"project_id": "p1"}),
	}
	assert.True(t, scoped.RestrictsApplicationView("p1"))

	// A narrowed permission restricts even alongside a project-wide one.
	mixed := append(PermissionSet{NewPermission(ScopeAll, ActionView, nil)}, scoped...)
	assert.True(t, mixed.RestrictsApplicationView("p1"))

	// Wildcard patterns are still narrower than "*".
	pattern := PermissionSet{NewPermission(ScopeApplication, ActionView, Object{"application_namespace": "team-*"})}
	assert.True(t, pattern.RestrictsApplicationView("p1"))

	// No application view at all.
	none := PermissionSet{NewPermission(ScopeProjectLogs, ActionView, nil)}
	assert.True(t, none.RestrictsApplicationView("p1"))
	assert.True(t, PermissionSet{}.RestrictsApplicationView("p1"))

	// Project-wide view limited to another project.
	other := PermissionSet{NewPermission(ScopeProjectAll, ActionView, Object{"project_id": "p2"})}
	assert.True(t, other.RestrictsApplicationView("p1"))
	assert.False(t, other.RestrictsApplicationView("p2"))

	// Explicit wildcards are project-wide.
	wide := PermissionSet{NewPermission(ScopeApplication, ActionView, Object{
		"project_id":            "*",
		"application_category":  "*",
		"application_namespace": "*",
		"application_kind":      "*",
		"application_name":      "*",
	})}
	assert.False(t, wide.RestrictsApplicationView("p1"))
}
