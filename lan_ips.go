//go:build !wasm

package user

import (
	"time"

	"github.com/tinywasm/unixid"
)



func RegisterLAN(userID, rut string) error {
	normalized, err := validateRUT(rut)
	if err != nil {
		return ErrInvalidRUT
	}

	id, err := GetIdentityByProvider("lan", normalized)
	if err == nil {
		if id.UserID != userID {
			return ErrRUTTaken
		}
		return nil
	} else if err != ErrNotFound {
		return err
	}

	return CreateIdentity(userID, "lan", normalized, "")
}

func UnregisterLAN(userID string) error {
	_, err := getIdentityByUserAndProvider(userID, "lan")
	if err == ErrNotFound {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	qb := store.db.Query(&LANIP{}).Where(LANIPMeta.UserID).Eq(userID)
	ips, _ := ReadAllLANIP(qb)
	for _, ip := range ips {
		store.db.Delete(ip)
	}

	qbId := store.db.Query(&Identity{}).Where(IdentityMeta.UserID).Eq(userID).Where(IdentityMeta.Provider).Eq("lan")
	ids, _ := ReadAllIdentity(qbId)
	for _, id := range ids {
		store.db.Delete(id)
	}
	return nil
}

func AssignLANIP(userID, ip, label string) error {
	qb := store.db.Query(&LANIP{}).Where(LANIPMeta.IP).Eq(ip)
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
	return store.db.Create(i)
}

func RevokeLANIP(userID, ip string) error {
	qb := store.db.Query(&LANIP{}).Where(LANIPMeta.UserID).Eq(userID).Where(LANIPMeta.IP).Eq(ip)
	results, err := ReadAllLANIP(qb)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return ErrNotFound
	}

	return store.db.Delete(results[0])
}

func GetLANIPs(userID string) ([]LANIP, error) {
	qb := store.db.Query(&LANIP{}).Where(LANIPMeta.UserID).Eq(userID).OrderBy(LANIPMeta.CreatedAt).Asc()
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

func checkLANIP(userID, ip string) error {
	qb := store.db.Query(&LANIP{}).Where(LANIPMeta.UserID).Eq(userID).Where(LANIPMeta.IP).Eq(ip)
	results, err := ReadAllLANIP(qb)
	if err != nil {
		return ErrInvalidCredentials
	}
	if len(results) == 0 {
		return ErrInvalidCredentials
	}
	return nil
}
