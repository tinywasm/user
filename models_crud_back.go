//go:build !wasm

package user

import "github.com/tinywasm/orm"

// --- userCRUD ---
type userCRUD struct {
	db    *orm.DB
	cache *userCache
}

func (h *userCRUD) HandlerName() string                   { return "users" }
func (h *userCRUD) AllowedRoles(action byte) []byte       { return []byte{'a'} }
func (h *userCRUD) ValidateData(action byte, _ any) error { return nil }

func (h *userCRUD) Create(payload any) (any, error) {
	u := payload.(User)
	return createUser(h.db, u.Email, u.Name, u.Phone)
}
func (h *userCRUD) Read(id string) (any, error) { return getUser(h.db, h.cache, id) }
func (h *userCRUD) List() (any, error)          { return listUsers(h.db) }
func (h *userCRUD) Update(payload any) (any, error) {
	u := payload.(User)
	return u, updateUser(h.db, h.cache, u.ID, u.Name, u.Phone)
}
func (h *userCRUD) Delete(id string) error { return deleteUser(h.db, h.cache, id) }

// --- roleCRUD ---
type roleCRUD struct{ m *Module }

func (h *roleCRUD) HandlerName() string                   { return "roles" }
func (h *roleCRUD) AllowedRoles(action byte) []byte       { return []byte{'a'} }
func (h *roleCRUD) ValidateData(action byte, _ any) error { return nil }

func (h *roleCRUD) Create(payload any) (any, error) {
	r := payload.(Role)
	err := h.m.CreateRole(r.ID, r.Code, r.Name, r.Description)
	return r, err
}
func (h *roleCRUD) Read(id string) (any, error) { return h.m.GetRole(id) }
func (h *roleCRUD) List() (any, error) {
	qb := h.m.db.Query(&Role{})
	roles, err := ReadAllRole(qb)
	if err != nil {
		return nil, err
	}
	res := make([]Role, len(roles))
	for i, r := range roles {
		res[i] = *r
	}
	return res, nil
}
func (h *roleCRUD) Update(payload any) (any, error) {
	r := payload.(Role)
	err := h.m.CreateRole(r.ID, r.Code, r.Name, r.Description) // CreateRole does an upsert
	return r, err
}
func (h *roleCRUD) Delete(id string) error { return h.m.DeleteRole(id) }

// --- permissionCRUD ---
type permissionCRUD struct{ m *Module }

func (h *permissionCRUD) HandlerName() string                   { return "permissions" }
func (h *permissionCRUD) AllowedRoles(action byte) []byte       { return []byte{'a'} }
func (h *permissionCRUD) ValidateData(action byte, _ any) error { return nil }

func (h *permissionCRUD) Create(payload any) (any, error) {
	p := payload.(Permission)
	err := h.m.CreatePermission(p.ID, p.Name, p.Resource, p.Action)
	return p, err
}
func (h *permissionCRUD) Read(id string) (any, error) { return h.m.GetPermission(id) }
func (h *permissionCRUD) List() (any, error) {
	qb := h.m.db.Query(&Permission{})
	perms, err := ReadAllPermission(qb)
	if err != nil {
		return nil, err
	}
	res := make([]Permission, len(perms))
	for i, p := range perms {
		res[i] = *p
	}
	return res, nil
}
func (h *permissionCRUD) Update(payload any) (any, error) {
	p := payload.(Permission)
	err := h.m.CreatePermission(p.ID, p.Name, p.Resource, p.Action) // CreatePermission does an upsert
	return p, err
}
func (h *permissionCRUD) Delete(id string) error { return h.m.DeletePermission(id) }

// --- lanipCRUD ---
type lanipCRUD struct{ m *Module }

func (h *lanipCRUD) HandlerName() string                   { return "lan_ips" }
func (h *lanipCRUD) AllowedRoles(action byte) []byte       { return []byte{'a'} }
func (h *lanipCRUD) ValidateData(action byte, _ any) error { return nil }

func (h *lanipCRUD) Create(payload any) (any, error) {
	ip := payload.(LANIP)
	err := h.m.AssignLANIP(ip.UserID, ip.IP, ip.Label)
	return ip, err
}
func (h *lanipCRUD) Read(id string) (any, error) {
	qb := h.m.db.Query(&LANIP{}).Where(LANIPMeta.ID).Eq(id)
	results, err := ReadAllLANIP(qb)
	if err != nil || len(results) == 0 {
		return LANIP{}, err
	}
	return *results[0], nil
}
func (h *lanipCRUD) List() (any, error) {
	qb := h.m.db.Query(&LANIP{})
	ips, err := ReadAllLANIP(qb)
	if err != nil {
		return nil, err
	}
	res := make([]LANIP, len(ips))
	for i, ip := range ips {
		res[i] = *ip
	}
	return res, nil
}
func (h *lanipCRUD) Update(payload any) (any, error) {
	return payload, nil // update isn't strictly defined for LANIPs beyond assigning
}
func (h *lanipCRUD) Delete(id string) error {
	qb := h.m.db.Query(&LANIP{}).Where(LANIPMeta.ID).Eq(id)
	results, err := ReadAllLANIP(qb)
	if err != nil || len(results) == 0 {
		return err
	}
	return h.m.RevokeLANIP(results[0].UserID, results[0].IP)
}
