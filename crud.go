//go:build !wasm

package user

import (
	"strings"
	"time"

	"github.com/tinywasm/unixid"
)

func CreateUser(email, name, phone string) (User, error) {
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

	if err := store.db.Create(&newUser); err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrEmailTaken
		}
		return User{}, err
	}
	return newUser, nil
}

func hydrateUser(u *User) error {
	// 1. Fetch UserRoles to get Role IDs
	qbUserRoles := store.db.Query(&UserRole{}).Where(UserRoleMeta.UserID).Eq(u.ID)
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
		qbRoles := store.db.Query(&Role{}).Where(RoleMeta.ID).In(roleIDs)
		roles, err := ReadAllRole(qbRoles)
		if err != nil {
			return err
		}
		u.Roles = make([]Role, len(roles))
		for i, r := range roles {
			u.Roles[i] = *r
		}

		// 3. Fetch RolePermissions to get Permission IDs
		qbRolePerms := store.db.Query(&RolePermission{}).Where(RolePermissionMeta.RoleID).In(roleIDs)
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
			qbPerms := store.db.Query(&Permission{}).Where(PermissionMeta.ID).In(permIDs)
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

func GetUser(id string) (User, error) {
	if cached, ok := store.userCache.Get(id); ok {
		return *cached, nil
	}

	qb := store.db.Query(&User{}).Where(UserMeta.ID).Eq(id)
	results, err := ReadAllUser(qb)
	if err != nil {
		return User{}, err
	}
	if len(results) == 0 {
		return User{}, ErrNotFound
	}
	u := results[0]

	if err := hydrateUser(u); err != nil {
		return User{}, err
	}

	store.userCache.Set(u.ID, u)
	return *u, nil
}

func GetUserByEmail(email string) (User, error) {
	qb := store.db.Query(&User{}).Where(UserMeta.Email).Eq(email)
	results, err := ReadAllUser(qb)
	if err != nil {
		return User{}, err
	}
	if len(results) == 0 {
		return User{}, ErrNotFound
	}
	u := results[0]

	if cached, ok := store.userCache.Get(u.ID); ok {
		return *cached, nil
	}

	if err := hydrateUser(u); err != nil {
		return User{}, err
	}

	store.userCache.Set(u.ID, u)
	return *u, nil
}

func UpdateUser(id, name, phone string) error {
	store.userCache.Delete(id)
	qb := store.db.Query(&User{}).Where(UserMeta.ID).Eq(id)
	results, err := ReadAllUser(qb)
	if err != nil || len(results) == 0 {
		return ErrNotFound
	}
	u := results[0]
	u.Name = name
	u.Phone = phone
	return store.db.Update(u)
}

func SuspendUser(id string) error {
	store.userCache.Delete(id)
	qb := store.db.Query(&User{}).Where(UserMeta.ID).Eq(id)
	results, err := ReadAllUser(qb)
	if err != nil || len(results) == 0 {
		return ErrNotFound
	}
	u := results[0]
	u.Status = "suspended"
	return store.db.Update(u)
}

func ReactivateUser(id string) error {
	store.userCache.Delete(id)
	qb := store.db.Query(&User{}).Where(UserMeta.ID).Eq(id)
	results, err := ReadAllUser(qb)
	if err != nil || len(results) == 0 {
		return ErrNotFound
	}
	u := results[0]
	u.Status = "active"
	return store.db.Update(u)
}

func isUniqueViolation(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed") ||
		strings.Contains(err.Error(), "constraint: unique") ||
		strings.Contains(err.Error(), "duplicate key")
}
