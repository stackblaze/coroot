package db

import (
	"testing"

	"github.com/coroot/coroot/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	database, err := NewSqlite(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, database.Migrate())
	return database
}

func TestEnsureUserOnlyManagesHandoffUsers(t *testing.T) {
	database := newTestDB(t)
	require.NoError(t, database.AddUser("ops@example.com", "password", "Ops", rbac.RoleAdmin))

	_, err := database.EnsureUser("ops@example.com", "Ops", "tenant-role")
	assert.ErrorIs(t, err, ErrConflict)
	assert.ErrorIs(t, database.CheckHandoffUser("ops@example.com"), ErrConflict)
	u, err := database.GetUserByEmail("ops@example.com")
	require.NoError(t, err)
	assert.Equal(t, []rbac.RoleName{rbac.RoleAdmin}, u.Roles)
	assert.False(t, u.Handoff)

	assert.NoError(t, database.CheckHandoffUser("tenant@example.com"))
	u, err = database.EnsureUser("tenant@example.com", "Tenant", "tenant-role")
	require.NoError(t, err)
	assert.True(t, u.Handoff)
	assert.Equal(t, []rbac.RoleName{"tenant-role"}, u.Roles)
	assert.NoError(t, database.CheckHandoffUser("tenant@example.com"))

	u2, err := database.EnsureUser("tenant@example.com", "", "other-role")
	require.NoError(t, err)
	assert.Equal(t, u.Id, u2.Id)
	assert.Equal(t, "Tenant", u2.Name)
	assert.Equal(t, []rbac.RoleName{"other-role"}, u2.Roles)

	// Setting a password turns a handoff user into a regular account.
	require.NoError(t, database.UpdateUser(u.Id, u.Email, "new-password", u.Name, "other-role"))
	_, err = database.EnsureUser("tenant@example.com", "", "tenant-role")
	assert.ErrorIs(t, err, ErrConflict)
}

func TestRoleHasNonHandoffUsers(t *testing.T) {
	database := newTestDB(t)
	_, err := database.EnsureUser("tenant@example.com", "", "tenant-role")
	require.NoError(t, err)

	inUse, err := database.RoleHasNonHandoffUsers("tenant-role")
	require.NoError(t, err)
	assert.False(t, inUse)

	require.NoError(t, database.AddUser("ops@example.com", "password", "Ops", "tenant-role"))
	inUse, err = database.RoleHasNonHandoffUsers("tenant-role")
	require.NoError(t, err)
	assert.True(t, inUse)
}

func TestUserHandoffColumnBackfill(t *testing.T) {
	dir := t.TempDir()
	database, err := NewSqlite(dir)
	require.NoError(t, err)
	// A users table created before the handoff column existed.
	_, err = database.Exec(`CREATE TABLE users (
		id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		password TEXT NOT NULL,
		roles TEXT NOT NULL
	)`)
	require.NoError(t, err)
	_, err = database.Exec(`INSERT INTO users(email, name, password, roles) VALUES
		('kubero+u1+ns@handoff.local', 'u1', 'x', '["tenant-role"]'),
		('ops@example.com', 'Ops', 'x', '["Admin"]')`)
	require.NoError(t, err)

	require.NoError(t, database.Migrate())
	u, err := database.GetUserByEmail("kubero+u1+ns@handoff.local")
	require.NoError(t, err)
	assert.True(t, u.Handoff)
	u, err = database.GetUserByEmail("ops@example.com")
	require.NoError(t, err)
	assert.False(t, u.Handoff)

	// The backfill runs only when the column is added.
	require.NoError(t, database.AddUser("new@handoff.local", "password", "New", rbac.RoleViewer))
	require.NoError(t, database.Migrate())
	u, err = database.GetUserByEmail("new@handoff.local")
	require.NoError(t, err)
	assert.False(t, u.Handoff)
}
