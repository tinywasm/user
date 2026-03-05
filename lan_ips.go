//go:build !wasm

package user

import (
	"time"

	"github.com/tinywasm/orm"
	"github.com/tinywasm/unixid"
)

func (m *Module) RegisterLAN(userID, rut string) error {
	normalized, err := validateRUT(rut)
	if err != nil {
		return ErrInvalidRUT
	}

	id, err := getIdentityByProvider(m.db, "lan", normalized)
	if err == nil {
		if id.UserID != userID {
			return ErrRUTTaken
		}
		return nil
	} else if err != ErrNotFound {
		return err
	}

	return createIdentity(m.db, userID, "lan", normalized, "")
}

func (m *Module) UnregisterLAN(userID string) error {
	_, err := getIdentityByUserAndProvider(m.db, userID, "lan")
	if err == ErrNotFound {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	qb := m.db.Query(&LANIP{}).Where(LANIPMeta.UserID).Eq(userID)
	ips, _ := ReadAllLANIP(qb)
	for _, ip := range ips {
		m.db.Delete(ip, orm.Eq(LANIPMeta.ID, ip.ID))
	}

	qbId := m.db.Query(&Identity{}).Where(IdentityMeta.UserID).Eq(userID).Where(IdentityMeta.Provider).Eq("lan")
	ids, _ := ReadAllIdentity(qbId)
	for _, id := range ids {
		m.db.Delete(id, orm.Eq(IdentityMeta.ID, id.ID))
	}
	return nil
}

func (m *Module) AssignLANIP(userID, ip, label string) error {
	qb := m.db.Query(&LANIP{}).Where(LANIPMeta.IP).Eq(ip)
	results, err := ReadAllLANIP(qb)
	if err != nil {
		return err
	}
	if len(results) > 0 {
		return ErrIPTaken
	}

	u, err := unixid.NewUnixID()
	if err != nil {
		return err
	}
	id := u.GetNewID()
	now := time.Now().Unix()

	i := &LANIP{
		ID:        id,
		UserID:    userID,
		IP:        ip,
		Label:     label,
		CreatedAt: now,
	}
	return m.db.Create(i)
}

func (m *Module) RevokeLANIP(userID, ip string) error {
	qb := m.db.Query(&LANIP{}).Where(LANIPMeta.UserID).Eq(userID).Where(LANIPMeta.IP).Eq(ip)
	results, err := ReadAllLANIP(qb)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return ErrNotFound
	}

	return m.db.Delete(results[0], orm.Eq(LANIPMeta.ID, results[0].ID))
}

func (m *Module) GetLANIPs(userID string) ([]LANIP, error) {
	qb := m.db.Query(&LANIP{}).Where(LANIPMeta.UserID).Eq(userID).OrderBy(LANIPMeta.CreatedAt).Asc()
	results, err := ReadAllLANIP(qb)
	if err != nil {
		return nil, err
	}

	ips := make([]LANIP, 0, len(results))
	for _, r := range results {
		ips = append(ips, *r)
	}
	return ips, nil
}

func checkLANIP(db *orm.DB, userID, ip string) error {
	qb := db.Query(&LANIP{}).Where(LANIPMeta.UserID).Eq(userID).Where(LANIPMeta.IP).Eq(ip)
	results, err := ReadAllLANIP(qb)
	if err != nil {
		return ErrInvalidCredentials
	}
	if len(results) == 0 {
		return ErrInvalidCredentials
	}
	return nil
}
