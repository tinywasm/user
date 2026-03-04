//go:build !wasm

package user

import (
	"strings"
	"time"

	"github.com/tinywasm/orm"
	"github.com/tinywasm/unixid"
)

func createUser(db *orm.DB, email, name, phone string) (User, error) {
	u, err := unixid.NewUnixID()
	if err != nil {
		return User{}, err
	}

	id := u.GetNewID()
	now := time.Now().Unix()

	newUser := User{
		ID:        id,
		Email:     email,
		Name:      name,
		Phone:     phone,
		Status:    "active",
		CreatedAt: now,
	}

	if err := db.Create(&newUser); err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrEmailTaken
		}
		return User{}, err
	}
	return newUser, nil
}

func hydrateUser(db *orm.DB, u *User) error {
	// 1. Fetch UserRoles to get Role IDs
	qbUserRoles := db.Query(&UserRole{}).Where(UserRoleMeta.UserID).Eq(u.ID)
	userRoles, err := ReadAllUserRole(qbUserRoles)
	if err != nil {
		return err
	}

	var roleIDs []any
	for _, ur := range userRoles {
		roleIDs = append(roleIDs, ur.RoleID)
	}

	if len(roleIDs) > 0 {
		// 2. Fetch Roles
		qbRoles := db.Query(&Role{}).Where(RoleMeta.ID).In(roleIDs)
		roles, err := ReadAllRole(qbRoles)
		if err != nil {
			return err
		}
		u.Roles = make([]Role, len(roles))
		for i, r := range roles {
			u.Roles[i] = *r
		}

		// 3. Fetch RolePermissions to get Permission IDs
		qbRolePerms := db.Query(&RolePermission{}).Where(RolePermissionMeta.RoleID).In(roleIDs)
		rolePerms, err := ReadAllRolePermission(qbRolePerms)
		if err != nil {
			return err
		}

		var permIDs []any
		for _, rp := range rolePerms {
			permIDs = append(permIDs, rp.PermissionID)
		}

		if len(permIDs) > 0 {
			// 4. Fetch Permissions
			qbPerms := db.Query(&Permission{}).Where(PermissionMeta.ID).In(permIDs)
			perms, err := ReadAllPermission(qbPerms)
			if err != nil {
				return err
			}

			// Deduplicate permissions
			permMap := make(map[string]Permission)
			for _, p := range perms {
				permMap[p.ID] = *p
			}

			u.Permissions = make([]Permission, 0, len(permMap))
			for _, p := range permMap {
				u.Permissions = append(u.Permissions, p)
			}
		} else {
			u.Permissions = []Permission{}
		}
	} else {
		u.Roles = []Role{}
		u.Permissions = []Permission{}
	}

	return nil
}

func (m *Module) GetUser(id string) (User, error) {
	return getUser(m.db, m.ucache, id)
}

func getUser(db *orm.DB, cache *userCache, id string) (User, error) {
	if cache != nil {
		if cached, ok := cache.Get(id); ok {
			return *cached, nil
		}
	}

	qb := db.Query(&User{}).Where(UserMeta.ID).Eq(id)
	results, err := ReadAllUser(qb)
	if err != nil {
		return User{}, err
	}
	if len(results) == 0 {
		return User{}, ErrNotFound
	}
	u := results[0]

	if err := hydrateUser(db, u); err != nil {
		return User{}, err
	}

	if cache != nil {
		cache.Set(u.ID, u)
	}
	return *u, nil
}

func getUserByEmail(db *orm.DB, cache *userCache, email string) (User, error) {
	qb := db.Query(&User{}).Where(UserMeta.Email).Eq(email)
	results, err := ReadAllUser(qb)
	if err != nil {
		return User{}, err
	}
	if len(results) == 0 {
		return User{}, ErrNotFound
	}
	u := results[0]

	if cache != nil {
		if cached, ok := cache.Get(u.ID); ok {
			return *cached, nil
		}
	}

	if err := hydrateUser(db, u); err != nil {
		return User{}, err
	}

	if cache != nil {
		cache.Set(u.ID, u)
	}
	return *u, nil
}

func updateUser(db *orm.DB, cache *userCache, id, name, phone string) error {
	if cache != nil {
		cache.Delete(id)
	}
	qb := db.Query(&User{}).Where(UserMeta.ID).Eq(id)
	results, err := ReadAllUser(qb)
	if err != nil || len(results) == 0 {
		return ErrNotFound
	}
	u := results[0]
	u.Name = name
	u.Phone = phone
	return db.Update(u)
}

func suspendUser(db *orm.DB, cache *userCache, id string) error {
	if cache != nil {
		cache.Delete(id)
	}
	qb := db.Query(&User{}).Where(UserMeta.ID).Eq(id)
	results, err := ReadAllUser(qb)
	if err != nil || len(results) == 0 {
		return ErrNotFound
	}
	u := results[0]
	u.Status = "suspended"
	return db.Update(u)
}

func reactivateUser(db *orm.DB, cache *userCache, id string) error {
	if cache != nil {
		cache.Delete(id)
	}
	qb := db.Query(&User{}).Where(UserMeta.ID).Eq(id)
	results, err := ReadAllUser(qb)
	if err != nil || len(results) == 0 {
		return ErrNotFound
	}
	u := results[0]
	u.Status = "active"
	return db.Update(u)
}

func listUsers(db *orm.DB) ([]User, error) {
	qb := db.Query(&User{})
	users, err := ReadAllUser(qb)
	if err != nil {
		return nil, err
	}
	var res []User
	for _, u := range users {
		hydrateUser(db, u)
		res = append(res, *u)
	}
	return res, nil
}

func deleteUser(db *orm.DB, cache *userCache, id string) error {
	if cache != nil {
		cache.Delete(id)
	}
	qb := db.Query(&User{}).Where(UserMeta.ID).Eq(id)
	results, err := ReadAllUser(qb)
	if err != nil || len(results) == 0 {
		return ErrNotFound
	}
	return db.Delete(results[0])
}

func isUniqueViolation(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed") ||
		strings.Contains(err.Error(), "constraint: unique") ||
		strings.Contains(err.Error(), "duplicate key")
}
