package userserver

import (

	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/user"
)

func (m *Module) CreateRole(id string, code model.RoleCode, name, description string) error {
	r := &user.Role{
		Id:          id,
		Code:        string(code),
		Name:        name,
		Description: description,
	}
	err := m.db.Create(r)
	if err != nil && isUniqueViolation(err) {
		qb := m.db.Query(&user.Role{}).Where(user.Role_.Id).Eq(id)
		existingR, readErr := user.ReadOneRole(qb, &user.Role{})
		if readErr != nil {
			return readErr
		}
		existingR.Code = string(code)
		existingR.Name = name
		existingR.Description = description
		return m.db.Update(existingR, orm.Eq(user.Role_.Id, existingR.Id))
	}
	return err
}

func (m *Module) GetRole(id string) (*user.Role, error) {
	qb := m.db.Query(&user.Role{}).Where(user.Role_.Id).Eq(id)
	return user.ReadOneRole(qb, &user.Role{})
}

func (m *Module) DeleteRole(id string) error {
	qb := m.db.Query(&user.Role{}).Where(user.Role_.Id).Eq(id)
	roles, err := user.ReadAllRole(qb)
	if err != nil {
		return err
	}
	if len(roles) == 0 {
		return nil // Or ErrNotFound
	}
	r := roles[0]

	// Delete from link tables first to simulate cascade, since tinywasm/orm doesn't cascade automatically like PRAGMA foreign_keys = ON does unless DB level handles it
	urQb := m.db.Query(&user.UserRole{}).Where(user.UserRole_.RoleId).Eq(id)
	urs, _ := user.ReadAllUserRole(urQb)
	for _, ur := range urs {
		m.db.Delete(ur, orm.Eq(user.UserRole_.UserId, ur.UserId), orm.Eq(user.UserRole_.RoleId, ur.RoleId))
	}

	rpQb := m.db.Query(&user.RolePermission{}).Where(user.RolePermission_.RoleId).Eq(id)
	rps, _ := user.ReadAllRolePermission(rpQb)
	for _, rp := range rps {
		m.db.Delete(rp, orm.Eq(user.RolePermission_.RoleId, rp.RoleId), orm.Eq(user.RolePermission_.PermissionId, rp.PermissionId))
	}

	err = m.db.Delete(r, orm.Eq(user.Role_.Id, r.Id))
	if err == nil {
		m.ucache.InvalidateByRole(id)
	}
	return err
}

func (m *Module) CreatePermission(id, name string, resource model.Resource, action model.Action) error {
	p := &user.Permission{
		Id:       id,
		Name:     name,
		Resource: string(resource),
		Action:   action.String(),
	}
	err := m.db.Create(p)
	if err != nil && isUniqueViolation(err) {
		qb := m.db.Query(&user.Permission{}).Where(user.Permission_.Id).Eq(id)
		existingP, readErr := user.ReadOnePermission(qb, &user.Permission{})
		if readErr != nil {
			return readErr
		}
		existingP.Name = name
		existingP.Resource = string(resource)
		existingP.Action = action.String()
		return m.db.Update(existingP, orm.Eq(user.Permission_.Id, existingP.Id))
	}
	return err
}

func (m *Module) GetPermission(id string) (*user.Permission, error) {
	qb := m.db.Query(&user.Permission{}).Where(user.Permission_.Id).Eq(id)
	return user.ReadOnePermission(qb, &user.Permission{})
}

func (m *Module) DeletePermission(id string) error {
	qb := m.db.Query(&user.Permission{}).Where(user.Permission_.Id).Eq(id)
	p, err := user.ReadOnePermission(qb, &user.Permission{})
	if err != nil {
		return err
	}

	err = m.db.Delete(p, orm.Eq(user.Permission_.Id, p.Id))
	if err == nil {
		m.ucache.InvalidateByPermission(id)
	}
	return err
}

func (m *Module) AssignRole(userID, roleID string) error {
	ur := &user.UserRole{
		UserId: userID,
		RoleId: roleID,
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
	qb := m.db.Query(&user.UserRole{}).Where(user.UserRole_.UserId).Eq(userID).Where(user.UserRole_.RoleId).Eq(roleID)
	ur, err := user.ReadOneUserRole(qb, &user.UserRole{})
	if err != nil {
		return err
	}
	err = m.db.Delete(ur, orm.Eq(user.UserRole_.UserId, ur.UserId), orm.Eq(user.UserRole_.RoleId, ur.RoleId))
	if err == nil {
		m.ucache.Delete(userID)
	}
	return err
}

func (m *Module) GetUserRoles(userID string) ([]user.Role, error) {
	qbUserRoles := m.db.Query(&user.UserRole{}).Where(user.UserRole_.UserId).Eq(userID)
	userRoles, err := user.ReadAllUserRole(qbUserRoles)
	if err != nil {
		return nil, err
	}

	var roleIDs []any
	for _, ur := range userRoles {
		roleIDs = append(roleIDs, ur.RoleId)
	}

	if len(roleIDs) == 0 {
		return []user.Role{}, nil
	}

	qbRoles := m.db.Query(&user.Role{}).Where(user.Role_.Id).In(roleIDs)
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
		RoleId:       roleID,
		PermissionId: permissionID,
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
	AllowedRoles(action model.Action) []model.RoleCode
}

func (m *Module) GetRoleByCode(code model.RoleCode) (*user.Role, error) {
	qb := m.db.Query(&user.Role{}).Where(user.Role_.Code).Eq(string(code))
	roles, err := user.ReadAllRole(qb)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return nil, orm.ErrNoRows
	}
	return roles[0], nil
}

func (m *Module) Register(handlers ...RBACObject) error {
	return registerRBAC(m, handlers...)
}

func registerRBAC(m *Module, handlers ...RBACObject) error {
	actions := []model.Action{model.Create, model.Read, model.Update, model.Delete}
	for _, h := range handlers {
		resource := h.HandlerName()
		for _, action := range actions {
			roles := h.AllowedRoles(action)
			if len(roles) == 0 {
				continue
			}

			permID := resource + ":" + action.String()
			if err := m.CreatePermission(permID, permID, model.Resource(resource), action); err != nil {
				return err
			}

			for _, code := range roles {
				r, err := m.GetRoleByCode(code)
				if err != nil {
					continue // Role not found, skip assignment
				}
				if err := m.AssignPermission(r.Id, permID); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (m *Module) HasPermission(userID string, resource model.Resource, action model.Action) (bool, error) {
	if userID == "" {
		return false, nil
	}
	u, err := m.GetUser(userID)
	if err != nil {
		if err == user.ErrNotFound || err == orm.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	for _, p := range u.Permissions {
		// Una acción ilegible NO se salta: saltarla borra el permiso real en silencio y deja
		// la fila corrupta invisible para siempre. Denegar sí; callar no.
		pAction, err := model.ParseAction(p.Action)
		if err != nil {
			return false, err
		}
		grant := model.Grant{
			Resource: model.Resource(p.Resource),
			Actions:  pAction,
		}
		if grant.Matches(resource, action) {
			return true, nil
		}
	}
	return false, nil
}
