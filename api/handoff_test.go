package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coroot/coroot/config"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeHandoffRedirect(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"/p/abc/app/ns:Deployment:app", "/p/abc/app/ns:Deployment:app"},
		{"p/abc", "/p/abc"},
		{"//evil.com", ""},
		{"https://evil.com/x", ""},
		{"http://evil.com", ""},
	}
	for _, c := range cases {
		if got := sanitizeHandoffRedirect(c.in); got != c.want {
			t.Errorf("sanitizeHandoffRedirect(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestJoinBasePathPreserveQuery(t *testing.T) {
	cases := []struct {
		base, target, want string
	}{
		{"/metrics", "/p/proj/app/x?theme=dark", "/metrics/p/proj/app/x?theme=dark"},
		{"/metrics/", "/p/proj/app/foo:bar?theme=dark", "/metrics/p/proj/app/foo:bar?theme=dark"},
		{"/metrics", "/p/proj/app/x", "/metrics/p/proj/app/x"},
	}
	for _, c := range cases {
		if got := joinBasePathPreserveQuery(c.base, c.target); got != c.want {
			t.Errorf("joinBasePathPreserveQuery(%q, %q) = %q, want %q", c.base, c.target, got, c.want)
		}
	}
}

func TestAppendQueryParam(t *testing.T) {
	cases := []struct {
		raw, want string
	}{
		{"/p/x", "/p/x?theme=dark"},
		{"/p/x?foo=1", "/p/x?foo=1&theme=dark"},
		{"/p/x?theme=light", "/p/x?theme=dark"},
	}
	for _, c := range cases {
		if got := appendQueryParam(c.raw, "theme", "dark"); got != c.want {
			t.Errorf("appendQueryParam(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestCreateHandoffRefusesExistingNonHandoffUser(t *testing.T) {
	database := newTestDB(t)
	require.NoError(t, database.AddUser("ops@example.com", "password", "Ops", rbac.RoleAdmin))
	require.NoError(t, database.AddUser("dev@example.com", "password", "Dev", "kubero-team-prod"))
	cfg := config.NewConfig()
	cfg.Auth.HandoffSecret = "secret"
	api := &Api{db: database, roles: rbac.NewDBRoleManager(database), cfg: cfg}

	create := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/handoff", strings.NewReader(body))
		r.Header.Set(HandoffSecretHeader, "secret")
		w := httptest.NewRecorder()
		api.CreateHandoff(w, r)
		return w
	}
	perms := `"permissions":[{"scope":"project.application","action":"view","object":{"project_id":"p1","application_namespace":"team-a"}}]`

	// An existing regular user is neither logged in nor re-roled.
	w := create(`{"email":"ops@example.com","role":"kubero-tenant-prod",` + perms + `}`)
	assert.Equal(t, http.StatusConflict, w.Code)
	u, err := database.GetUserByEmail("ops@example.com")
	require.NoError(t, err)
	assert.Equal(t, []rbac.RoleName{rbac.RoleAdmin}, u.Roles)
	roles, err := database.GetCustomRoles()
	require.NoError(t, err)
	assert.Empty(t, roles, "the role is not saved")

	// A role held by a regular user is not redefined.
	w = create(`{"email":"tenant@example.com","role":"kubero-team-prod",` + perms + `}`)
	assert.Equal(t, http.StatusConflict, w.Code)
	_, err = database.GetUserByEmail("tenant@example.com")
	assert.ErrorIs(t, err, db.ErrNotFound)

	// New and existing handoff users work as before.
	for i := 0; i < 2; i++ {
		w = create(`{"email":"tenant@example.com","role":"kubero-tenant-prod",` + perms + `}`)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
	u, err = database.GetUserByEmail("tenant@example.com")
	require.NoError(t, err)
	assert.True(t, u.Handoff)
	assert.Equal(t, []rbac.RoleName{"kubero-tenant-prod"}, u.Roles)
}

func TestHandoffRoleAllowed(t *testing.T) {
	for _, role := range []rbac.RoleName{
		"kubero-shop-production",
		"kubero-shop-prod-0123456789ab",
		"kubero-my_app-dev",
		"kubero-0123456789ab",
		"kubero-truncated-",
		rbac.RoleName("kubero-" + strings.Repeat("a", 57)),
	} {
		assert.True(t, handoffRoleAllowed(role), role)
	}
	for _, role := range []rbac.RoleName{
		"", rbac.RoleAdmin, rbac.RoleEditor, rbac.RoleViewer,
		"admin", "kubero", "kubero-", "Kubero-shop-prod", "kubero-Shop-prod",
		"kubero--shop", "kubero-shop prod", "kubero-shop.prod", "team-kubero-x",
		" kubero-shop-prod", "kubero-shop-prod\n",
		rbac.RoleName("kubero-" + strings.Repeat("a", 58)),
	} {
		assert.False(t, handoffRoleAllowed(role), role)
	}
}

func TestCreateHandoffRefusesRolesItDoesNotManage(t *testing.T) {
	database := newTestDB(t)
	teamPerms := rbac.PermissionSet{rbac.NewPermission(rbac.ScopeProjectRisks, rbac.ActionView, rbac.Object{"project_id": "p1"})}
	require.NoError(t, database.UpsertCustomRole("team-role", teamPerms))
	cfg := config.NewConfig()
	cfg.Auth.HandoffSecret = "secret"
	api := &Api{db: database, roles: rbac.NewDBRoleManager(database), cfg: cfg}

	create := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/handoff", strings.NewReader(body))
		r.Header.Set(HandoffSecretHeader, "secret")
		w := httptest.NewRecorder()
		api.CreateHandoff(w, r)
		return w
	}
	perms := `"permissions":[{"scope":"project.application","action":"view","object":{"project_id":"p1","application_namespace":"team-a"}}]`

	for _, body := range []string{
		`{"email":"tenant@example.com"}`,
		`{"email":"tenant@example.com","role":"Admin"}`,
		`{"email":"tenant@example.com","role":"Editor"}`,
		`{"email":"tenant@example.com","role":"Viewer"}`,
		`{"email":"tenant@example.com","role":"team-role"}`,
		`{"email":"tenant@example.com","role":"team-role",` + perms + `}`,
		`{"email":"tenant@example.com","role":"Kubero-Team",` + perms + `}`,
	} {
		w := create(body)
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
		_, err := database.GetUserByEmail("tenant@example.com")
		assert.ErrorIs(t, err, db.ErrNotFound, body)
	}
	roles, err := database.GetCustomRoles()
	require.NoError(t, err)
	require.Len(t, roles, 1)
	assert.Equal(t, rbac.RoleName("team-role"), roles[0].Name)
	assert.Equal(t, teamPerms, roles[0].Permissions, "an unmanaged role is not redefined")

	w := create(`{"email":"tenant@example.com","role":"kubero-team-prod",` + perms + `}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}
