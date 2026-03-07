//go:build !wasm

package user

import (
	"errors"
	"net"
	"net/http"
	"strings"
)

// GenerateAPIToken creates a signed JWT for API access (MCP clients, IDEs, LLMs).
// Requires Config.JWTSecret — independent of the configured AuthMode.
// ttl=0 → 100 years (effectively no expiry).
// The returned token is used as a Bearer token in Authorization headers.
func (m *Module) GenerateAPIToken(userID string, ttl int) (string, error) {
	if len(m.config.JWTSecret) == 0 {
		return "", errors.New("JWTSecret is required for API token generation")
	}
	if ttl == 0 {
		ttl = 365 * 24 * 3600 * 100
	}
	return GenerateJWT(m.config.JWTSecret, userID, ttl)
}

// clientIP extracts the real client IP from the request.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return strings.SplitN(ip, ",", 2)[0]
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}
