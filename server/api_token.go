package userserver

import (
	"errors"
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

