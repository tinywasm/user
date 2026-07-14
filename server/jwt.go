package userserver

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/jwt"
)

// ErrInvalidToken is what a forged token means to this library's consumers. It stays
// deliberately vague: telling a caller WHY a token failed tells an attacker where they
// stand.
var ErrInvalidToken = fmt.Err("token", "invalid")

// The JWT format lives in github.com/tinywasm/jwt: signing, verification, and every
// security invariant (HS256 only, `alg` never read from the token, empty secret and
// empty subject refused, no `exp` means invalid rather than eternal). This file is only
// the seam that maps that library's verdict onto this one's vocabulary.

// GenerateJWT issues a session token for userID.
func GenerateJWT(secret []byte, userID string, ttl int) (string, error) {
	return jwt.Sign(secret, jwt.NewClaims(userID, ttl))
}

// ValidateJWT authenticates a token and returns its subject alongside the verdict.
//
// The verdict is NOT collapsed into the error: an expired session and a forged token
// are different events, and a caller that cannot tell them apart raises a tampering
// alarm every time somebody's session merely runs out. Callers switch on jwt.Outcome;
// the error means only that this module is misconfigured (no JWTSecret).
func ValidateJWT(secret []byte, token string) (string, jwt.Outcome, error) {
	claims, outcome, err := jwt.Verify(secret, token)
	if err != nil {
		return "", jwt.Forged, err
	}
	return claims.Sub, outcome, nil
}
