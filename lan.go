//go:build !wasm

package user

import (
	"net"
	"net/http"
	"strconv"
	"strings"
)

func LoginLAN(rut string, r *http.Request) (User, error) {
	normalized, err := validateRUT(rut)
	if err != nil {
		return User{}, ErrInvalidRUT
	}

	identity, err := GetIdentityByProvider("lan", normalized)
	if err != nil {
		return User{}, ErrInvalidCredentials
	}

	u, err := GetUser(identity.UserID)
	if err != nil {
		return User{}, ErrInvalidCredentials
	}
	if u.Status == "suspended" {
		return User{}, ErrSuspended
	}

	clientIP := extractClientIP(r, store.config.TrustProxy)
	if err := checkLANIP(identity.UserID, clientIP); err != nil {
		return User{}, ErrInvalidCredentials
	}

	return u, nil
}

func validateRUT(rut string) (string, error) {
	rut = strings.TrimSpace(rut)
	rut = strings.ReplaceAll(rut, ".", "")
	rut = strings.ReplaceAll(rut, "-", "")

	if len(rut) < 2 { // At least 1 digit + 1 DV
		return "", ErrInvalidRUT
	}

	bodyStr := rut[:len(rut)-1]
	dvStr := strings.ToUpper(rut[len(rut)-1:])

	_, err := strconv.Atoi(bodyStr)
	if err != nil {
		return "", ErrInvalidRUT
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
		return "", ErrInvalidRUT
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
