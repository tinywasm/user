package userserver

import (
	"github.com/tinywasm/user"
)

// Bootstrap seeds the very first administrator. It is a no-op unless the users
// table is EMPTY. When empty it: creates the user (email), sets the password,
// creates (or reuses) the role with code RoleCodeAdmin, grants it the wildcard
// permission ResourceAll/ActionAll, and assigns it. Idempotent and safe to call
// on every startup.
func (m *Module) Bootstrap(email, password string) error {
	// Check if any users exist
	qb := m.db.Query(&user.User{})
	users, err := user.ReadAllUser(qb)
	if err != nil {
		return err
	}

	if len(users) > 0 {
		return nil
	}

	if email == "" || password == "" {
		return user.ErrInvalidCredentials
	}

	// 1. Create User
	u, err := createUser(m.db, email, "Administrator", "")
	if err != nil {
		return err
	}

	// 2. Set Password
	if err := m.SetPassword(u.ID, password); err != nil {
		return err
	}

	// 3. Create Role
	roleID := "role_" + user.RoleCodeAdmin
	if err := m.CreateRole(roleID, user.RoleCodeAdmin, "Administrator", "Full access role"); err != nil {
		return err
	}

	// 4. Create Permission
	permID := "perm_all"
	if err := m.CreatePermission(permID, "All Access", user.ResourceAll, user.ActionAll); err != nil {
		return err
	}

	// 5. Assign Permission to Role
	if err := m.AssignPermission(roleID, permID); err != nil {
		return err
	}

	// 6. Assign Role to User
	if err := m.AssignRole(u.ID, roleID); err != nil {
		return err
	}

	return nil
}
