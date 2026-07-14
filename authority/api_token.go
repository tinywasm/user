package authority

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/jwt"
)

// The JWT format lives in github.com/tinywasm/jwt: signing, verification, and every
// security invariant (HS256 only, `alg` never read from the token, empty secret and
// empty subject refused, no `exp` means invalid rather than eternal).
//
// This module does NOT re-export it. Signing reads the secret from m.config, never from
// an argument: a package-level GenerateJWT(secret, ...) would let a call site pass the
// wrong key — mint tokens nobody can validate, or validate tokens nobody minted. The
// secret has exactly one owner, and the type system now says so.

// errInvalidToken is what a forged token means inside this module. It stays deliberately
// vague: telling a caller WHY a token failed tells an attacker where they stand.
var errInvalidToken = fmt.Err("token", "invalid")

// ErrJWTSecretRequired is returned by any token operation attempted without a secret.
var ErrJWTSecretRequired = fmt.Err("JWTSecret", "is", "required")

// issueToken signs a session token for userID. The ONE signer in this module.
func (m *Module) issueToken(userID string, ttl int) (string, error) {
	if len(m.config.JWTSecret) == 0 {
		return "", ErrJWTSecretRequired
	}
	return jwt.Sign(m.config.JWTSecret, jwt.NewClaims(userID, ttl))
}

// GenerateAPIToken creates a signed JWT for API access (MCP clients, IDEs, LLMs).
// Requires Config.JWTSecret — independent of the configured AuthMode.
// ttl=0 → 50 years (effectively no expiry). Not 100: this module compiles for the
// edge, where int is 32-bit, and 100 years of seconds overflows int32.
// The returned token is used as a Bearer token in Authorization headers.
func (m *Module) GenerateAPIToken(userID string, ttl int) (string, error) {
	if ttl == 0 {
		ttl = 365 * 24 * 3600 * 50
	}
	return m.issueToken(userID, ttl)
}
