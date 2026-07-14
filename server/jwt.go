package userserver

import (
	"github.com/tinywasm/base64"
	"github.com/tinywasm/crypto"
	"github.com/tinywasm/json"
	"github.com/tinywasm/model"
	"github.com/tinywasm/time"

	"github.com/tinywasm/fmt"
	"github.com/tinywasm/user"
)

var ErrInvalidToken = fmt.Err("token", "invalid")

type jwtHeader struct {
	Alg string
	Typ string
}

func (h jwtHeader) EncodeFields(w model.FieldWriter) {
	w.String("alg", h.Alg)
	w.String("typ", h.Typ)
}
func (h jwtHeader) IsNil() bool { return false }

type jwtPayload struct {
	Sub string
	Exp int64
	Iat int64
}

func (p jwtPayload) EncodeFields(w model.FieldWriter) {
	w.String("sub", p.Sub)
	w.Int("exp", p.Exp)
	w.Int("iat", p.Iat)
}
func (p jwtPayload) IsNil() bool { return false }
func (p *jwtPayload) DecodeFields(r model.FieldReader) {
	p.Sub, _ = r.String("sub")
	p.Exp, _ = r.Int("exp")
	p.Iat, _ = r.Int("iat")
}

func GenerateJWT(secret []byte, userID string, ttl int) (string, error) {
	if ttl == 0 {
		ttl = 86400
	}
	now := time.Now() / 1e9

	headerObj := jwtHeader{Alg: "HS256", Typ: "JWT"}
	var h string
	json.Encode(headerObj, &h)

	payloadObj := jwtPayload{Sub: userID, Exp: now + int64(ttl), Iat: now}
	var p string
	json.Encode(payloadObj, &p)

	header := base64.URLEncode([]byte(h))
	payload := base64.URLEncode([]byte(p))
	sig := jwtSign(secret, header+"."+payload)
	return header + "." + payload + "." + sig, nil
}

func ValidateJWT(secret []byte, token string) (string, error) {
	parts := fmt.Split(token, ".")
	if len(parts) != 3 {
		return "", ErrInvalidToken
	}
	expected := jwtSign(secret, parts[0]+"."+parts[1])
	if !crypto.HMACEqual([]byte(parts[2]), []byte(expected)) {
		return "", ErrInvalidToken
	}
	raw, err := base64.URLDecode(parts[1])
	if err != nil {
		return "", ErrInvalidToken
	}
	var p jwtPayload
	if err := json.Decode(string(raw), &p); err != nil {
		return "", ErrInvalidToken
	}
	if time.Now() / 1e9 > p.Exp {
		return "", user.ErrSessionExpired
	}
	return p.Sub, nil
}

func jwtSign(secret []byte, data string) string {
	sum := crypto.HMACSHA256(secret, []byte(data))
	return base64.URLEncode(sum)
}
