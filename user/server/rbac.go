package userserver

import (
	"database/sql"

	"github.com/tinywasm/orm"
	"github.com/tinywasm/user"
)

func (m *Module) CreateRole(id string, code string, name, description string) error {
	r := &user.Role{
		ID:          id,
		Code:        code,
		Name:        name,
		Description: description,
	}
	err := m.db.Create(r)
	if err != nil && isUniqueViolation(err) {
		qb := m.db.Query(&user.Role{}).Where(user.Role_.ID).Eq(id)
		existingR, readErr := user.ReadOneRole(qb, &user.Role{})
		if readErr != nil {
			return readErr
		}
		existingR.Code = code
		existingR.Name = name
		existingR.Description = description
		return m.db.Update(existingR, orm.Eq(user.Role_.ID, existingR.ID))
	}
	return err
}

func (m *Module) GetRole(id string) (*user.Role, error) {
	qb := m.db.Query(&user.Role{}).Where(user.Role_.ID).Eq(id)
	return user.ReadOneRole(qb, &user.Role{})
}

func (m *Module) DeleteRole(id string) error {
	qb := m.db.Query(&user.Role{}).Where(user.Role_.ID).Eq(id)
	roles, err := user.ReadAllRole(qb)
	if err != nil {
		return err
	}
	if len(roles) == 0 {
		return nil // Or ErrNotFound
	}
	r := roles[0]

	// Delete from link tables first to simulate cascade, since tinywasm/orm doesn't cascade automatically like PRAGMA foreign_keys = ON does unless DB level handles it
	urQb := m.db.Query(&user.UserRole{}).Where(user.UserRole_.RoleID).Eq(id)
	urs, _ := user.ReadAllUserRole(urQb)
	for _, ur := range urs {
		m.db.Delete(ur, orm.Eq(user.UserRole_.UserID, ur.UserID), orm.Eq(user.UserRole_.RoleID, ur.RoleID))
	}

	rpQb := m.db.Query(&user.RolePermission{}).Where(user.RolePermission_.RoleID).Eq(id)
	rps, _ := user.ReadAllRolePermission(rpQb)
	for _, rp := range rps {
		m.db.Delete(rp, orm.Eq(user.RolePermission_.RoleID, rp.RoleID), orm.Eq(user.RolePermission_.PermissionID, rp.PermissionID))
	}

	err = m.db.Delete(r, orm.Eq(user.Role_.ID, r.ID))
	if err == nil {
		m.ucache.InvalidateByRole(id)
	}
	return err
}

func (m *Module) CreatePermission(id, name, resource string, action string) error {
	p := &user.Permission{
		ID:       id,
		Name:     name,
		Resource: resource,
		Action:   action,
	}
	err := m.db.Create(p)
	if err != nil && isUniqueViolation(err) {
		qb := m.db.Query(&user.Permission{}).Where(user.Permission_.ID).Eq(id)
		existingP, readErr := user.ReadOnePermission(qb, &user.Permission{})
		if readErr != nil {
			return readErr
		}
		existingP.Name = name
		existingP.Resource = resource
		existingP.Action = action
		return m.db.Update(existingP, orm.Eq(user.Permission_.ID, existingP.ID))
	}
	return err
}

func (m *Module) GetPermission(id string) (*user.Permission, error) {
	qb := m.db.Query(&user.Permission{}).Where(user.Permission_.ID).Eq(id)
	return user.ReadOnePermission(qb, &user.Permission{})
}

func (m *Module) DeletePermission(id string) error {
	qb := m.db.Query(&user.Permission{}).Where(user.Permission_.ID).Eq(id)
	p, err := user.ReadOnePermission(qb, &user.Permission{})
	if err != nil {
		return err
	}

	err = m.db.Delete(p, orm.Eq(user.Permission_.ID, p.ID))
	if err == nil {
		m.ucache.InvalidateByPermission(id)
	}
	return err
}

func (m *Module) AssignRole(userID, roleID string) error {
	ur := &user.UserRole{
		UserID: userID,
		RoleID: roleID,
	}
	err := m.db.Create(ur)
	if err != nil && isUniqueViolation(err) {
		return nil // Ignore duplicates
	}
	if err == nil {
		m.ucache.Delete(userID) // Invalidate user to reload roles
	}
	return err
}

func (m *Module) RevokeRole(userID, roleID string) error {
	qb := m.db.Query(&user.UserRole{}).Where(user.UserRole_.UserID).Eq(userID).Where(user.UserRole_.RoleID).Eq(roleID)
	ur, err := user.ReadOneUserRole(qb, &user.UserRole{})
	if err != nil {
		return err
	}
	err = m.db.Delete(ur, orm.Eq(user.UserRole_.UserID, ur.UserID), orm.Eq(user.UserRole_.RoleID, ur.RoleID))
	if err == nil {
		m.ucache.Delete(userID)
	}
	return err
}

func (m *Module) GetUserRoles(userID string) ([]user.Role, error) {
	qbUserRoles := m.db.Query(&user.UserRole{}).Where(user.UserRole_.UserID).Eq(userID)
	userRoles, err := user.ReadAllUserRole(qbUserRoles)
	if err != nil {
		return nil, err
	}

	var roleIDs []any
	for _, ur := range userRoles {
		roleIDs = append(roleIDs, ur.RoleID)
	}

	if len(roleIDs) == 0 {
		return []user.Role{}, nil
	}

	qbRoles := m.db.Query(&user.Role{}).Where(user.Role_.ID).In(roleIDs)
	rolesPtrs, err := user.ReadAllRole(qbRoles)
	if err != nil {
		return nil, err
	}

	roles := make([]user.Role, len(rolesPtrs))
	for i, r := range rolesPtrs {
		roles[i] = *r
	}
	return roles, nil
}

func (m *Module) AssignPermission(roleID, permissionID string) error {
	rp := &user.RolePermission{
		RoleID:       roleID,
		PermissionID: permissionID,
	}
	err := m.db.Create(rp)
	if err != nil && isUniqueViolation(err) {
		return nil // Ignore duplicates
	}
	if err == nil {
		m.ucache.InvalidateByRole(roleID) // Invalidate users with this role
	}
	return err
}

type RBACObject interface {
	HandlerName() string
	AllowedRoles(action byte) []byte
}

func (m *Module) GetRoleByCode(code string) (*user.Role, error) {
	qb := m.db.Query(&user.Role{}).Where(user.Role_.Code).Eq(code)
	roles, err := user.ReadAllRole(qb)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return nil, sql.ErrNoRows
	}
	return roles[0], nil
}

func (m *Module) Register(handlers ...RBACObject) error {
	return registerRBAC(m, handlers...)
}

func registerRBAC(m *Module, handlers ...RBACObject) error {
	actions := []byte{'c', 'r', 'u', 'd'}
	for _, h := range handlers {
		resource := h.HandlerName()
		for _, action := range actions {
			roles := h.AllowedRoles(action)
			if len(roles) == 0 {
				continue
			}

			permID := resource + ":" + string(action)
			if err := m.CreatePermission(permID, permID, resource, string(action)); err != nil {
				return err
			}

			for _, code := range roles {
				r, err := m.GetRoleByCode(string(code))
				if err != nil {
					continue // Role not found, skip assignment
				}
				if err := m.AssignPermission(r.ID, permID); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (m *Module) HasPermission(userID, resource string, action byte) (bool, error) {
	u, err := m.GetUser(userID)
	if err != nil {
		if err == user.ErrNotFound || err == sql.ErrNoRows {
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
