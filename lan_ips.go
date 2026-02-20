package user

import (
	"time"

	"github.com/tinywasm/unixid"
)

type LANIP struct {
	ID        string
	UserID    string
	IP        string
	Label     string
	CreatedAt int64
}

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

	if err := store.exec.Exec("DELETE FROM user_lan_ips WHERE user_id = ?", userID); err != nil {
		return err
	}
	return store.exec.Exec("DELETE FROM user_identities WHERE user_id = ? AND provider = 'lan'", userID)
}

func AssignLANIP(userID, ip, label string) error {
	var count int
	err := store.exec.QueryRow("SELECT COUNT(*) FROM user_lan_ips WHERE ip = ?", ip).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrIPTaken
	}

	u, err := unixid.NewUnixID()
	if err != nil {
		return err
	}
	id := u.GetNewID()
	now := time.Now().Unix()

	return store.exec.Exec(
		"INSERT INTO user_lan_ips (id, user_id, ip, label, created_at) VALUES (?, ?, ?, ?, ?)",
		id, userID, ip, label, now,
	)
}

func RevokeLANIP(userID, ip string) error {
	var count int
	err := store.exec.QueryRow("SELECT COUNT(*) FROM user_lan_ips WHERE user_id = ? AND ip = ?", userID, ip).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}

	return store.exec.Exec("DELETE FROM user_lan_ips WHERE user_id = ? AND ip = ?", userID, ip)
}

func GetLANIPs(userID string) ([]LANIP, error) {
	rows, err := store.exec.Query("SELECT id, user_id, ip, label, created_at FROM user_lan_ips WHERE user_id = ? ORDER BY created_at ASC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ips []LANIP
	for rows.Next() {
		var i LANIP
		if err := rows.Scan(&i.ID, &i.UserID, &i.IP, &i.Label, &i.CreatedAt); err != nil {
			return nil, err
		}
		ips = append(ips, i)
	}
	if ips == nil {
		ips = []LANIP{}
	}
	return ips, rows.Err()
}

func checkLANIP(userID, ip string) error {
	var count int
	err := store.exec.QueryRow("SELECT COUNT(*) FROM user_lan_ips WHERE user_id = ? AND ip = ?", userID, ip).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrInvalidCredentials
	}
	return nil
}
