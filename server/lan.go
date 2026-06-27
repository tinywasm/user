package userserver

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tinywasm/orm"
	"github.com/tinywasm/unixid"
	"github.com/tinywasm/user"
)

func (m *Module) LoginLAN(rut string, r *http.Request) (user.User, error) {
	normalized, err := validateRUT(rut)
	if err != nil {
		return user.User{}, user.ErrInvalidRUT
	}

	identity, err := getIdentityByProvider(m.db, "lan", normalized)
	if err != nil {
		return user.User{}, user.ErrInvalidCredentials
	}

	u, err := getUser(m.db, m.ucache, identity.UserID)
	if err != nil {
		return user.User{}, user.ErrInvalidCredentials
	}
	if u.Status == "suspended" {
		return user.User{}, user.ErrSuspended
	}

	clientIP := extractClientIP(r, m.config.TrustProxy)
	if err := checkLANIP(m.db, identity.UserID, clientIP); err != nil {
		return user.User{}, user.ErrInvalidCredentials
	}

	return u, nil
}

func (m *Module) RegisterLAN(userID, rut string) error {
	normalized, err := validateRUT(rut)
	if err != nil {
		return user.ErrInvalidRUT
	}

	id, err := getIdentityByProvider(m.db, "lan", normalized)
	if err == nil {
		if id.UserID != userID {
			return user.ErrRUTTaken
		}
		return nil
	} else if err != user.ErrNotFound {
		return err
	}

	return createIdentity(m.db, userID, "lan", normalized, "")
}

func (m *Module) UnregisterLAN(userID string) error {
	_, err := getIdentityByUserAndProvider(m.db, userID, "lan")
	if err == user.ErrNotFound {
		return user.ErrNotFound
	}
	if err != nil {
		return err
	}

	qb := m.db.Query(&user.LANIP{}).Where(user.LANIP_.UserID).Eq(userID)
	ips, _ := user.ReadAllLANIP(qb)
	for _, ip := range ips {
		m.db.Delete(ip, orm.Eq(user.LANIP_.ID, ip.ID))
	}

	qbId := m.db.Query(&user.Identity{}).Where(user.Identity_.UserID).Eq(userID).Where(user.Identity_.Provider).Eq("lan")
	ids, _ := user.ReadAllIdentity(qbId)
	for _, id := range ids {
		m.db.Delete(id, orm.Eq(user.Identity_.ID, id.ID))
	}
	return nil
}

func (m *Module) AssignLANIP(userID, ip, label string) error {
	qb := m.db.Query(&user.LANIP{}).Where(user.LANIP_.IP).Eq(ip)
	results, err := user.ReadAllLANIP(qb)
	if err != nil {
		return err
	}
	if len(results) > 0 {
		return user.ErrIPTaken
	}

	u, err := unixid.NewUnixID()
	if err != nil {
		return err
	}
	id := u.GetNewID()
	now := time.Now().Unix()

	i := &user.LANIP{
		ID:        id,
		UserID:    userID,
		IP:        ip,
		Label:     label,
		CreatedAt: now,
	}
	return m.db.Create(i)
}

func (m *Module) RevokeLANIP(userID, ip string) error {
	qb := m.db.Query(&user.LANIP{}).Where(user.LANIP_.UserID).Eq(userID).Where(user.LANIP_.IP).Eq(ip)
	results, err := user.ReadAllLANIP(qb)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return user.ErrNotFound
	}

	return m.db.Delete(results[0], orm.Eq(user.LANIP_.ID, results[0].ID))
}

func (m *Module) GetLANIPs(userID string) ([]user.LANIP, error) {
	qb := m.db.Query(&user.LANIP{}).Where(user.LANIP_.UserID).Eq(userID).OrderBy(user.LANIP_.CreatedAt).Asc()
	results, err := user.ReadAllLANIP(qb)
	if err != nil {
		return nil, err
	}

	ips := make([]user.LANIP, 0, len(results))
	for _, r := range results {
		ips = append(ips, *r)
	}
	return ips, nil
}

func checkLANIP(db *orm.DB, userID, ip string) error {
	qb := db.Query(&user.LANIP{}).Where(user.LANIP_.UserID).Eq(userID).Where(user.LANIP_.IP).Eq(ip)
	results, err := user.ReadAllLANIP(qb)
	if err != nil {
		return user.ErrInvalidCredentials
	}
	if len(results) == 0 {
		return user.ErrInvalidCredentials
	}
	return nil
}

func validateRUT(rut string) (string, error) {
	rut = strings.TrimSpace(rut)
	rut = strings.ReplaceAll(rut, ".", "")
	rut = strings.ReplaceAll(rut, "-", "")

	if len(rut) < 2 { // At least 1 digit + 1 DV
		return "", user.ErrInvalidRUT
	}

	bodyStr := rut[:len(rut)-1]
	dvStr := strings.ToUpper(rut[len(rut)-1:])

	_, err := strconv.Atoi(bodyStr)
	if err != nil {
		return "", user.ErrInvalidRUT
	}

	sum := 0
	multiplier := 2
	for i := len(bodyStr) - 1; i >= 0; i-- {
		digit := int(bodyStr[i] - '0')
		sum += digit * multiplier
		multiplier++
		if multiplier > 7 {
			multiplier = 2
		}
	}

	expectedDV := 11 - (sum % 11)
	var expectedDVStr string
	if expectedDV == 11 {
		expectedDVStr = "0"
	} else if expectedDV == 10 {
		expectedDVStr = "K"
	} else {
		expectedDVStr = strconv.Itoa(expectedDV)
	}

	if dvStr != expectedDVStr {
		return "", user.ErrInvalidRUT
	}

	return bodyStr + "-" + dvStr, nil
}

func extractClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[0])
		}
		xri := r.Header.Get("X-Real-IP")
		if xri != "" {
			return strings.TrimSpace(xri)
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
