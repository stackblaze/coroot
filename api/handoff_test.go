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
	require.NoError(t, database.AddUser("dev@example.com", "password", "Dev", "team-role"))
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
	w := create(`{"email":"ops@example.com","role":"tenant-role",` + perms + `}`)
	assert.Equal(t, http.StatusConflict, w.Code)
	u, err := database.GetUserByEmail("ops@example.com")
	require.NoError(t, err)
	assert.Equal(t, []rbac.RoleName{rbac.RoleAdmin}, u.Roles)
	roles, err := database.GetCustomRoles()
	require.NoError(t, err)
	assert.Empty(t, roles, "the role is not saved")

	// A role held by a regular user is not redefined.
	w = create(`{"email":"tenant@example.com","role":"team-role",` + perms + `}`)
	assert.Equal(t, http.StatusConflict, w.Code)
	_, err = database.GetUserByEmail("tenant@example.com")
	assert.ErrorIs(t, err, db.ErrNotFound)

	// New and existing handoff users work as before.
	for i := 0; i < 2; i++ {
		w = create(`{"email":"tenant@example.com","role":"tenant-role",` + perms + `}`)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
	u, err = database.GetUserByEmail("tenant@example.com")
	require.NoError(t, err)
	assert.True(t, u.Handoff)
	assert.Equal(t, []rbac.RoleName{"tenant-role"}, u.Roles)
}
