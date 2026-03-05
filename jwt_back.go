//go:build !wasm

package user

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/tinywasm/fmt"
)

var ErrInvalidToken = fmt.Err("token", "invalid")

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type jwtPayload struct {
	Sub string `json:"sub"` // userID
	Exp int64  `json:"exp"`
	Iat int64  `json:"iat"`
}

func generateJWT(secret []byte, userID string, ttl int) (string, error) {
	if ttl == 0 {
		ttl = 86400
	}
	now := time.Now().Unix()
	h, _ := json.Marshal(jwtHeader{Alg: "HS256", Typ: "JWT"})
	p, _ := json.Marshal(jwtPayload{Sub: userID, Exp: now + int64(ttl), Iat: now})
	header := base64.RawURLEncoding.EncodeToString(h)
	payload := base64.RawURLEncoding.EncodeToString(p)
	sig := jwtSign(secret, header+"."+payload)
	return header + "." + payload + "." + sig, nil
}

func validateJWT(secret []byte, token string) (string, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return "", ErrInvalidToken
	}
	expected := jwtSign(secret, parts[0]+"."+parts[1])
	if !hmac.Equal([]byte(parts[2]), []byte(expected)) {
		return "", ErrInvalidToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", ErrInvalidToken
	}
	var p jwtPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", ErrInvalidToken
	}
	if time.Now().Unix() > p.Exp {
		return "", ErrSessionExpired
	}
	return p.Sub, nil
}

func jwtSign(secret []byte, data string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
