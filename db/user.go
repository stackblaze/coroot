package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"slices"

	"github.com/coroot/coroot/rbac"
	"github.com/coroot/coroot/utils"
	"golang.org/x/crypto/bcrypt"
)

const (
	AdminUserLogin           = "admin"
	AdminUserName            = "Admin"
	AdminUserDefaultPassword = ""
	AnonymousUserName        = "Anonymous"

	// legacyHandoffEmailSuffix marks users created by trusted handoff before
	// the users table had a handoff column. It is used only once, when the
	// column is added.
	legacyHandoffEmailSuffix = "@handoff.local"
)

type User struct {
	Id        int
	Email     string
	Name      string
	Roles     []rbac.RoleName
	Anonymous bool
	// Handoff is true for users created by the trusted handoff endpoint.
	Handoff bool
}

func (u *User) Migrate(m *Migrator) error {
	err := m.Exec(`
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		password TEXT NOT NULL,
		roles TEXT NOT NULL
	)`)
	if err != nil {
		return err
	}
	exists, err := m.ColumnExists("users", "handoff")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if err = m.AddColumnIfNotExists("users", "handoff", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	return m.Exec("UPDATE users SET handoff = 1 WHERE email LIKE $1", "%"+legacyHandoffEmailSuffix)
}

func (u *User) IsDefaultAdmin() bool {
	return u.Email == AdminUserLogin
}

func AnonymousUser(role rbac.RoleName) *User {
	return &User{Name: AnonymousUserName, Roles: []rbac.RoleName{role}, Anonymous: true}
}

func (db *DB) CreateAdminIfNotExists(password string) error {
	var i int
	err := db.db.QueryRow("SELECT 1 FROM users WHERE email = $1", AdminUserLogin).Scan(&i)
	if err == nil {
		// Stackblaze: the bootstrap password is AUTHORITATIVE, not first-boot
		// only. The admin credential lives in a Kubernetes Secret; the API
		// deliberately refuses to edit the admin user and the CLI reset needs
		// a TTY, so an existing admin whose password drifted from the Secret
		// was unrecoverable without database surgery. Secret = truth.
		if password != "" {
			return db.SetAdminPassword(password)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return db.AddUser(AdminUserLogin, password, AdminUserName, rbac.RoleAdmin)
}

func (db *DB) SetAdminPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.db.Exec("UPDATE users SET password = $1 WHERE email = $2", string(hash), AdminUserLogin)
	return err
}

func (db *DB) DefaultAdminUserIsTheOnlyUser() (*User, error) {
	rows, err := db.db.Query("SELECT id, email, name, roles, password FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var u User
	var hash, roles string
	for rows.Next() {
		if u.Email != "" { // second iteration
			return nil, nil
		}
		err = rows.Scan(&u.Id, &u.Email, &u.Name, &roles, &hash)
		if err != nil {
			return nil, err
		}
	}
	if !u.IsDefaultAdmin() {
		return nil, nil
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(AdminUserDefaultPassword)) != nil {
		return nil, nil
	}
	err = json.Unmarshal([]byte(roles), &u.Roles)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) GetUsers() ([]*User, error) {
	rows, err := db.db.Query("SELECT id, email, name, roles, handoff FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []*User
	for rows.Next() {
		var u User
		var roles string
		var handoff int
		err = rows.Scan(&u.Id, &u.Email, &u.Name, &roles, &handoff)
		if err != nil {
			return nil, err
		}
		u.Handoff = handoff != 0
		err = json.Unmarshal([]byte(roles), &u.Roles)
		if err != nil {
			return nil, err
		}
		res = append(res, &u)
	}
	return res, nil
}

func (db *DB) AuthUser(email, password string) (int, error) {
	var id int
	var hash string
	err := db.db.QueryRow("SELECT id, password FROM users WHERE email = $1", email).Scan(&id, &hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return 0, ErrNotFound
	}
	return id, nil
}

func (db *DB) GetUser(id int) (*User, error) {
	u := User{Id: id}
	var roles string
	var handoff int
	err := db.db.QueryRow("SELECT email, name, roles, handoff FROM users WHERE id = $1", id).Scan(&u.Email, &u.Name, &roles, &handoff)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.Handoff = handoff != 0
	err = json.Unmarshal([]byte(roles), &u.Roles)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) GetUserByEmail(email string) (*User, error) {
	u := User{Email: email}
	var roles string
	var handoff int
	err := db.db.QueryRow("SELECT id, name, roles, handoff FROM users WHERE email = $1", email).Scan(&u.Id, &u.Name, &roles, &handoff)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.Handoff = handoff != 0
	err = json.Unmarshal([]byte(roles), &u.Roles)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// EnsureUser returns an existing handoff user by email or creates one with a
// random unusable password. When the user already exists, name and role are
// updated when provided. Users that were not created by handoff are never
// returned or modified: ErrConflict is returned instead.
func (db *DB) EnsureUser(email, name string, role rbac.RoleName) (*User, error) {
	u, err := db.GetUserByEmail(email)
	if err == nil {
		if !u.Handoff {
			return nil, ErrConflict
		}
		if name == "" {
			name = u.Name
		}
		if role == "" && len(u.Roles) > 0 {
			role = u.Roles[0]
		}
		if err = db.UpdateUser(u.Id, email, "", name, role); err != nil {
			return nil, err
		}
		return db.GetUser(u.Id)
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if name == "" {
		name = email
	}
	if role == "" {
		role = rbac.RoleViewer
	}
	// Random unusable password — handoff users authenticate via session cookie only.
	if err = db.addUser(email, utils.RandomString(48), name, role, true); err != nil {
		return nil, err
	}
	return db.GetUserByEmail(email)
}

// CheckHandoffUser returns ErrConflict when a user with the email exists and
// was not created by handoff.
func (db *DB) CheckHandoffUser(email string) error {
	u, err := db.GetUserByEmail(email)
	switch {
	case errors.Is(err, ErrNotFound):
		return nil
	case err != nil:
		return err
	case !u.Handoff:
		return ErrConflict
	}
	return nil
}

// RoleHasNonHandoffUsers reports whether any user not created by handoff has
// the role.
func (db *DB) RoleHasNonHandoffUsers(role rbac.RoleName) (bool, error) {
	users, err := db.GetUsers()
	if err != nil {
		return false, err
	}
	for _, u := range users {
		if !u.Handoff && slices.Contains(u.Roles, role) {
			return true, nil
		}
	}
	return false, nil
}

func (db *DB) AddUser(email, password, name string, role rbac.RoleName) error {
	return db.addUser(email, password, name, role, false)
}

func (db *DB) addUser(email, password, name string, role rbac.RoleName, handoff bool) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	roles, err := json.Marshal([]rbac.RoleName{role})
	if err != nil {
		return err
	}
	h := 0
	if handoff {
		h = 1
	}
	_, err = db.db.Exec("INSERT INTO users(email, name, password, roles, handoff) VALUES($1, $2, $3, $4, $5)", email, name, string(hash), string(roles), h)
	if db.IsUniqueViolationError(err) {
		return ErrConflict
	}
	return err
}

func (db *DB) UpdateUser(id int, email, password, name string, role rbac.RoleName) error {
	roles, err := json.Marshal([]rbac.RoleName{role})
	if err != nil {
		return err
	}
	if password != "" {
		var hash []byte
		hash, err = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		// A user given a password is a regular account from now on.
		_, err = db.db.Exec("UPDATE users SET email=$1, name=$2, password=$3, roles = $4, handoff = 0 WHERE id = $5", email, name, string(hash), string(roles), id)
	} else {
		_, err = db.db.Exec("UPDATE users SET email=$1, name=$2, roles = $3 WHERE id = $4", email, name, string(roles), id)
	}
	return err
}

func (db *DB) ChangeUserPassword(id int, oldPassword, newPassword string) error {
	var hash []byte
	err := db.db.QueryRow("SELECT password FROM users WHERE id = $1", id).Scan(&hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	if bcrypt.CompareHashAndPassword(hash, []byte(oldPassword)) != nil {
		return ErrInvalid
	}

	if oldPassword == newPassword {
		return ErrConflict
	}

	hash, err = bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.db.Exec("UPDATE users SET password=$1 WHERE id = $2", string(hash), id)
	return err
}

func (db *DB) DeleteUser(id int) error {
	_, err := db.db.Exec("DELETE FROM users WHERE id = $1", id)
	return err
}
