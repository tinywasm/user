package userserver

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/time"

	"github.com/tinywasm/orm"
	"github.com/tinywasm/router"
	"github.com/tinywasm/unixid"
	"github.com/tinywasm/user"
)

func (m *Module) LoginLAN(rut string, ctx router.Context) (user.User, error) {
	normalized, err := validateRUT(rut)
	if err != nil {
		return user.User{}, user.ErrInvalidRUT
	}

	identity, err := getIdentityByProvider(m.db, "lan", normalized)
	if err != nil {
		return user.User{}, user.ErrInvalidCredentials
	}

	u, err := getUser(m.db, m.ucache, identity.UserId)
	if err != nil {
		return user.User{}, user.ErrInvalidCredentials
	}
	if u.Status == "suspended" {
		return user.User{}, user.ErrSuspended
	}

	clientIP := extractClientIP(ctx, m.config.TrustProxy)
	if err := checkLANIP(m.db, identity.UserId, clientIP); err != nil {
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
		if id.UserId != userID {
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

	qb := m.db.Query(&user.LANIP{}).Where(user.LANIP_.UserId).Eq(userID)
	ips, _ := user.ReadAllLANIP(qb)
	for _, ip := range ips {
		m.db.Delete(ip, orm.Eq(user.LANIP_.Id, ip.Id))
	}

	qbId := m.db.Query(&user.Identity{}).Where(user.Identity_.UserId).Eq(userID).Where(user.Identity_.Provider).Eq("lan")
	ids, _ := user.ReadAllIdentity(qbId)
	for _, id := range ids {
		m.db.Delete(id, orm.Eq(user.Identity_.Id, id.Id))
	}
	return nil
}

func (m *Module) AssignLANIP(userID, ip, label string) error {
	qb := m.db.Query(&user.LANIP{}).Where(user.LANIP_.Ip).Eq(ip)
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
	now := time.Now() / 1e9

	i := &user.LANIP{
		Id:        id,
		UserId:    userID,
		Ip:        ip,
		Label:     label,
		CreatedAt: now,
	}
	return m.db.Create(i)
}

func (m *Module) RevokeLANIP(userID, ip string) error {
	qb := m.db.Query(&user.LANIP{}).Where(user.LANIP_.UserId).Eq(userID).Where(user.LANIP_.Ip).Eq(ip)
	results, err := user.ReadAllLANIP(qb)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return user.ErrNotFound
	}

	return m.db.Delete(results[0], orm.Eq(user.LANIP_.Id, results[0].Id))
}

func (m *Module) GetLANIPs(userID string) ([]user.LANIP, error) {
	qb := m.db.Query(&user.LANIP{}).Where(user.LANIP_.UserId).Eq(userID).OrderBy(user.LANIP_.CreatedAt).Asc()
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
	qb := db.Query(&user.LANIP{}).Where(user.LANIP_.UserId).Eq(userID).Where(user.LANIP_.Ip).Eq(ip)
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
	rut = fmt.Convert(rut).TrimSpace().String()
	rut = fmt.Convert(rut).Replace(".", "").Replace("-", "").String()

	if len(rut) < 2 { // At least 1 digit + 1 DV
		return "", user.ErrInvalidRUT
	}

	bodyStr := rut[:len(rut)-1]
	dvStr := fmt.ToUpper(rut[len(rut)-1:])

	if _, err := fmt.Convert(bodyStr).Int(); err != nil {
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
		expectedDVStr = fmt.Convert(expectedDV).String()
	}

	if dvStr != expectedDVStr {
		return "", user.ErrInvalidRUT
	}

	return bodyStr + "-" + expectedDVStr, nil
}

func extractClientIP(ctx router.Context, trustProxy bool) string {
	if trustProxy {
		xff := ctx.GetHeader("X-Forwarded-For")
		if xff != "" {
			parts := fmt.Split(xff, ",")
			return fmt.Convert(parts[0]).TrimSpace().String()
		}
		xri := ctx.GetHeader("X-Real-IP")
		if xri != "" {
			return fmt.Convert(xri).TrimSpace().String()
		}
	}

	if addr, ok := ctx.Value("RemoteAddr").(string); ok {
		// addr is often "ip:port"
		parts := fmt.Split(addr, ":")
		if len(parts) > 0 {
			return parts[0]
		}
		return addr
	}

	return ""
}
