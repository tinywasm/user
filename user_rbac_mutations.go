//go:build !wasm

package user

import (
	"database/sql"
)

func CreateRole(id string, code string, name, description string) error {
	r := &Role{
		ID:          id,
		Code:        code,
		Name:        name,
		Description: description,
	}
	err := store.db.Create(r)
	if err != nil && isUniqueViolation(err) {
		qb := store.db.Query(&Role{}).Where(RoleMeta.ID).Eq(id)
		existingR, readErr := ReadOneRole(qb, &Role{})
		if readErr != nil {
			return readErr
		}
		existingR.Code = code
		existingR.Name = name
		existingR.Description = description
		return store.db.Update(existingR)
	}
	return err
}

func GetRole(id string) (*Role, error) {
	qb := store.db.Query(&Role{}).Where(RoleMeta.ID).Eq(id)
	return ReadOneRole(qb, &Role{})
}

func DeleteRole(id string) error {
	qb := store.db.Query(&Role{}).Where(RoleMeta.ID).Eq(id)
	roles, err := ReadAllRole(qb)
	if err != nil {
		return err
	}
	if len(roles) == 0 {
		return nil // Or ErrNotFound
	}
	r := roles[0]

	// Delete from link tables first to simulate cascade, since tinywasm/orm doesn't cascade automatically like PRAGMA foreign_keys = ON does unless DB level handles it
	urQb := store.db.Query(&UserRole{}).Where(UserRoleMeta.RoleID).Eq(id)
	urs, _ := ReadAllUserRole(urQb)
	for _, ur := range urs {
		store.db.Delete(ur)
	}

	rpQb := store.db.Query(&RolePermission{}).Where(RolePermissionMeta.RoleID).Eq(id)
	rps, _ := ReadAllRolePermission(rpQb)
	for _, rp := range rps {
		store.db.Delete(rp)
	}

	err = store.db.Delete(r)
	if err == nil {
		store.userCache.InvalidateByRole(id)
	}
	return err
}

func CreatePermission(id, name, resource string, action string) error {
	p := &Permission{
		ID:       id,
		Name:     name,
		Resource: resource,
		Action:   action,
	}
	err := store.db.Create(p)
	if err != nil && isUniqueViolation(err) {
		qb := store.db.Query(&Permission{}).Where(PermissionMeta.ID).Eq(id)
		existingP, readErr := ReadOnePermission(qb, &Permission{})
		if readErr != nil {
			return readErr
		}
		existingP.Name = name
		existingP.Resource = resource
		existingP.Action = action
		return store.db.Update(existingP)
	}
	return err
}

func GetPermission(id string) (*Permission, error) {
	qb := store.db.Query(&Permission{}).Where(PermissionMeta.ID).Eq(id)
	return ReadOnePermission(qb, &Permission{})
}

func DeletePermission(id string) error {
	qb := store.db.Query(&Permission{}).Where(PermissionMeta.ID).Eq(id)
	p, err := ReadOnePermission(qb, &Permission{})
	if err != nil {
		return err
	}

	err = store.db.Delete(p)
	if err == nil {
		store.userCache.InvalidateByPermission(id)
	}
	return err
}

func AssignRole(userID, roleID string) error {
	ur := &UserRole{
		UserID: userID,
		RoleID: roleID,
	}
	err := store.db.Create(ur)
	if err != nil && isUniqueViolation(err) {
		return nil // Ignore duplicates
	}
	if err == nil {
		store.userCache.Delete(userID) // Invalidate user to reload roles
	}
	return err
}

func RevokeRole(userID, roleID string) error {
	qb := store.db.Query(&UserRole{}).Where(UserRoleMeta.UserID).Eq(userID).Where(UserRoleMeta.RoleID).Eq(roleID)
	ur, err := ReadOneUserRole(qb, &UserRole{})
	if err != nil {
		return err
	}
	err = store.db.Delete(ur)
	if err == nil {
		store.userCache.Delete(userID)
	}
	return err
}

func GetUserRoles(userID string) ([]Role, error) {
	qbUserRoles := store.db.Query(&UserRole{}).Where(UserRoleMeta.UserID).Eq(userID)
	userRoles, err := ReadAllUserRole(qbUserRoles)
	if err != nil {
		return nil, err
	}

	var roleIDs []any
	for _, ur := range userRoles {
		roleIDs = append(roleIDs, ur.RoleID)
	}

	if len(roleIDs) == 0 {
		return []Role{}, nil
	}

	qbRoles := store.db.Query(&Role{}).Where(RoleMeta.ID).In(roleIDs)
	rolesPtrs, err := ReadAllRole(qbRoles)
	if err != nil {
		return nil, err
	}

	roles := make([]Role, len(rolesPtrs))
	for i, r := range rolesPtrs {
		roles[i] = *r
	}
	return roles, nil
}

func AssignPermission(roleID, permissionID string) error {
	rp := &RolePermission{
		RoleID:       roleID,
		PermissionID: permissionID,
	}
	err := store.db.Create(rp)
	if err != nil && isUniqueViolation(err) {
		return nil // Ignore duplicates
	}
	if err == nil {
		store.userCache.InvalidateByRole(roleID) // Invalidate users with this role
	}
	return err
}

type RBACObject interface {
	HandlerName() string
	AllowedRoles(action byte) []byte
}

func GetRoleByCode(code string) (*Role, error) {
	qb := store.db.Query(&Role{}).Where(RoleMeta.Code).Eq(code)
	roles, err := ReadAllRole(qb)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return nil, sql.ErrNoRows
	}
	return roles[0], nil
}

func Register(handlers ...RBACObject) error {
	actions := []byte{'c', 'r', 'u', 'd'}
	for _, h := range handlers {
		resource := h.HandlerName()
		for _, action := range actions {
			roles := h.AllowedRoles(action)
			if len(roles) == 0 {
				continue
			}

			permID := resource + ":" + string(action)
			if err := CreatePermission(permID, permID, resource, string(action)); err != nil {
				return err
			}

			for _, code := range roles {
				r, err := GetRoleByCode(string(code))
				if err != nil {
					continue // Role not found, skip assignment
				}
				if err := AssignPermission(r.ID, permID); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func HasPermission(userID, resource string, action byte) (bool, error) {
	u, err := GetUser(userID)
	if err != nil {
		if err == ErrNotFound || err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	for _, p := range u.Permissions {
		if p.Resource == resource && p.Action == string(action) {
			return true, nil
		}
	}
	return false, nil
}
