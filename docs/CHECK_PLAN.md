# tinywasm/user — Bug Fix + Configurable Auth Mode (Cookie Session | JWT Cookie)

## Context

The `CHECK_PLAN.md` was **partially executed**. Steps 1–5 completed correctly (Module pattern, DI,
CRUDP wrappers, middleware). However two blockers prevent `gopush`:

1. **WASM build broken:** `module_login.go`, `module_register.go`, `module_profile.go`,
   `module_lan.go`, `module_oauth.go` son isomórficos (sin build tag) pero contienen `m *Module`.
   `Module` es `//go:build !wasm` → el compilador WASM falla con `undefined: Module`.

2. **New feature pending:** `AuthMode` configurable (Cookie con session en BD vs JWT en Cookie) no implementado.

**Decisiones finales:**
- **AuthMode**: Configurable — `AuthModeCookie` (default) | `AuthModeJWT`
- **Ambos modos usan HttpOnly Cookie** como transporte (seguro contra XSS, persiste en SPA/PWA)
- **Diferencia**: lo que viaja en la cookie cambia — session ID (con BD) vs JWT (sin BD)
- **JWT**: stateless, sin tabla `user_sessions`, validación criptográfica HMAC-SHA256
- **Cookie**: stateful, tabla `user_sessions`, revocación inmediata
- **WASM fix**: stub `type Module struct{}` en `//go:build wasm`

### Comparativa del diseño final

| | `AuthModeCookie` | `AuthModeJWT` |
|---|---|---|
| Cookie value | Session ID (opaco) | JWT firmado |
| DB lookup/request | Sí (`user_sessions`) | No (solo user cache) |
| Logout inmediato | ✅ borra fila en BD | ❌ espera expiración |
| SPA/PWA refresh | ✅ cookie persiste | ✅ cookie persiste |
| XSS protection | ✅ HttpOnly | ✅ HttpOnly |
| Escalabilidad | BD requerida | Stateless (multi-server) |

---

## Development Rules

- `go install github.com/tinywasm/devflow/cmd/gotest@latest`
- Build tags: backend `//go:build !wasm`, WASM `//go:build wasm`
- **No external libs** — JWT con `crypto/hmac`, `crypto/sha256`, `encoding/base64`, `encoding/json`
- **Breaking changes allowed** per CHECK_PLAN.md
- SRP: max 500 líneas por archivo

---

## Step 0 — Fix WASM Build (2 lines)

**New file:** `user_front.go`

```go
//go:build wasm

package user

// Module is a WASM stub. The real implementation lives in user_back.go.
type Module struct{}
```

**Why:** Los archivos isomórficos `module_*.go` referencian `*Module` en su struct.
WASM necesita que el tipo exista aunque nunca se llamen métodos sobre él desde WASM.
Fix mínimo, cero impacto arquitectónico.

**Verify:** `gotest` — la compilación WASM debe pasar tras este step.

---

## Step 1 — Extend Config with AuthMode

**Target file:** `user.go` (isomorphic, sin build tag)

Agregar antes del struct `Config`:

```go
// AuthMode selects the session strategy.
type AuthMode uint8

const (
    // AuthModeCookie stores a session ID in an HttpOnly cookie.
    // Stateful: requires user_sessions table. Supports immediate revocation.
    AuthModeCookie AuthMode = iota // default

    // AuthModeJWT stores a signed JWT in an HttpOnly cookie.
    // Stateless: no DB lookup per request. No immediate revocation.
    // Ideal for SPA/PWA and multi-server deployments.
    AuthModeJWT
)
```

Extender `Config` (reemplaza la definición existente):

```go
type Config struct {
    AuthMode AuthMode // default: AuthModeCookie

    // Shared by both modes
    CookieName string // default: "session"
    TokenTTL   int    // default: 86400 (seconds). Session TTL in cookie mode, JWT expiry in JWT mode.

    // JWT mode only — required when AuthMode == AuthModeJWT
    JWTSecret []byte

    TrustProxy     bool
    OAuthProviders []OAuthProvider
}
```

**Breaking change:** `SessionCookieName` → `CookieName`, `SessionTTL` → `TokenTTL`.
Todos los archivos que referencian `config.SessionCookieName` / `config.SessionTTL` deben actualizarse.

---

## Step 2 — JWT Primitives (sin dependencias externas)

**New file:** `jwt_back.go` (`//go:build !wasm`)

HMAC-SHA256 manual. Formato: `base64url(header).base64url(payload).base64url(sig)`.

```go
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
    header  := base64.RawURLEncoding.EncodeToString(h)
    payload := base64.RawURLEncoding.EncodeToString(p)
    sig     := jwtSign(secret, header+"."+payload)
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
```

---

## Step 3 — Update `New()` and `initSchema`

**Target files:** `user_back.go`, `migrate.go`

### `migrate.go` — Omitir tabla sessions en JWT mode

```go
func initSchema(db *orm.DB, mode AuthMode) error {
    models := []orm.Model{
        &User{}, &Role{}, &Permission{},
        &Identity{}, &LANIP{},
        &OAuthState{}, &UserRole{}, &RolePermission{},
    }
    if mode == AuthModeCookie {
        models = append(models, &Session{})
    }
    for _, m := range models {
        if err := db.CreateTable(m); err != nil {
            return err
        }
    }
    return nil
}
```

### `user_back.go` — Actualizar `New()`

```go
func New(db *orm.DB, cfg Config) (*Module, error) {
    if cfg.CookieName == "" {
        cfg.CookieName = "session"
    }
    if cfg.TokenTTL == 0 {
        cfg.TokenTTL = 86400
    }
    m := &Module{
        db:        db,
        cache:     newSessionCache(), // siempre init; warmUp solo en Cookie mode
        ucache:    newUserCache(),
        config:    cfg,
        providers: make(map[string]OAuthProvider),
    }
    if err := initSchema(db, cfg.AuthMode); err != nil {
        return nil, err
    }
    for _, p := range cfg.OAuthProviders {
        m.registerProvider(p)
    }
    if cfg.AuthMode == AuthModeCookie {
        if err := m.cache.warmUp(db); err != nil {
            return nil, err
        }
    }
    return m, nil
}
```

---

## Step 4 — Update `validateSession` in `middleware_back.go`

Ambos modos leen la misma cookie `CookieName`. La diferencia es cómo se interpreta el valor:

```go
func (m *Module) validateSession(r *http.Request) (*User, error) {
    cookie, err := r.Cookie(m.config.CookieName)
    if err != nil {
        return nil, ErrSessionExpired
    }

    if m.config.AuthMode == AuthModeJWT {
        userID, err := validateJWT(m.config.JWTSecret, cookie.Value)
        if err != nil {
            return nil, err
        }
        u, err := m.GetUser(userID)
        if err != nil {
            return nil, err
        }
        return &u, nil
    }

    // AuthModeCookie — session ID lookup (unchanged logic)
    sess, err := m.GetSession(cookie.Value)
    if err != nil {
        return nil, err
    }
    u, err := m.GetUser(sess.UserID)
    if err != nil {
        return nil, err
    }
    return &u, nil
}
```

---

## Step 5 — Update `SetCookie` in Login and Register Modules

**Target files:** `module_login_back.go`, `module_register_back.go`

Ambos modos setean una HttpOnly Cookie. La diferencia es el valor:
- Cookie mode: session ID (opaco, referencia a BD)
- JWT mode: JWT firmado (auto-contenido, sin BD)

```go
func (m *loginModule) SetCookie(userID string, w http.ResponseWriter, r *http.Request) error {
    var value string

    if m.m.config.AuthMode == AuthModeJWT {
        token, err := generateJWT(m.m.config.JWTSecret, userID, m.m.config.TokenTTL)
        if err != nil {
            return err
        }
        value = token
    } else {
        sess, err := m.m.CreateSession(userID, extractClientIP(r, m.m.config.TrustProxy), r.UserAgent())
        if err != nil {
            return err
        }
        value = sess.ID
    }

    http.SetCookie(w, &http.Cookie{
        Name:     m.m.config.CookieName,
        Value:    value,
        HttpOnly: true,
        Secure:   true,
        SameSite: http.SameSiteStrictMode,
        MaxAge:   m.m.config.TokenTTL,
        Path:     "/",
    })
    return nil
}
```

Aplicar **cambio idéntico** a `module_register_back.go`.

---

## Step 6 — Update Tests

**Target:** `tests/suite_back_test.go`

Tests existentes usan `user.New(db, user.Config{})` — actualizar campos renombrados:
`SessionCookieName` → `CookieName`, `SessionTTL` → `TokenTTL`.

**Agregar `TestJWTCookieMode` en `suite_back_test.go`:**

```go
func TestJWTCookieMode(t *testing.T) {
    db := newTestDB(t)
    secret := []byte("test-secret-32-bytes-minimum-len")
    m, err := user.New(db, user.Config{
        AuthMode:  user.AuthModeJWT,
        JWTSecret: secret,
    })
    if err != nil {
        t.Fatal(err)
    }

    u, _ := m.CreateUser("jwt@test.com", "JWT User", "")
    _ = m.SetPassword(u.ID, "password123")
    logged, err := m.Login("jwt@test.com", "password123")
    if err != nil {
        t.Fatal("login failed:", err)
    }

    // Generar JWT como lo haría SetCookie
    token, err := user.GenerateJWT(secret, logged.ID, 86400)
    if err != nil {
        t.Fatal(err)
    }

    // Request con JWT en cookie → debe autenticar
    req := httptest.NewRequest("GET", "/", nil)
    req.AddCookie(&http.Cookie{Name: "session", Value: token})
    rec := httptest.NewRecorder()
    var ctxUser *user.User
    m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctxUser, _ = m.FromContext(r.Context())
    })).ServeHTTP(rec, req)
    if ctxUser == nil || ctxUser.ID != logged.ID {
        t.Errorf("JWT middleware: expected user %s, got %v", logged.ID, ctxUser)
    }

    // Token inválido → 401
    req2 := httptest.NewRequest("GET", "/", nil)
    req2.AddCookie(&http.Cookie{Name: "session", Value: "invalid.jwt.token"})
    rec2 := httptest.NewRecorder()
    called := false
    m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        called = true
    })).ServeHTTP(rec2, req2)
    if called || rec2.Code != http.StatusUnauthorized {
        t.Errorf("want 401 for invalid JWT, got %d (called=%v)", rec2.Code, called)
    }
}
```

**Agregar `export_test.go`** en el root del package (no en `tests/`):

```go
// export_test.go — expone funciones internas para tests
package user

var (
    GenerateJWT = generateJWT
    ValidateJWT = validateJWT
)
```

Run `gotest` — todos los tests incluyendo WASM deben pasar.

---

## Step 7 — Update `SKILL.md` + Submit

**Target:** `docs/SKILL.md`

Reemplazar sección Configuration:

```
### Configuration
type Config struct {
    AuthMode   AuthMode // AuthModeCookie (default) | AuthModeJWT
    CookieName string   // default: "session" — used in both modes
    TokenTTL   int      // default: 86400s — session TTL or JWT expiry
    JWTSecret  []byte   // required for AuthModeJWT
    TrustProxy bool
    OAuthProviders []OAuthProvider
}

AuthModeCookie: session ID in HttpOnly cookie → DB lookup per request. Immediate revocation.
AuthModeJWT:    signed JWT in HttpOnly cookie → crypto validation only. Stateless, SPA/PWA-ready.
```

Luego ejecutar:

```bash
gopush 'fix: WASM stub for Module; feat: AuthModeCookie|AuthModeJWT via HttpOnly cookie'
```

---

## File Change Summary

| File | Action | Purpose |
|---|---|---|
| `user_front.go` | **CREATE** `//go:build wasm` | WASM stub para tipo Module |
| `user.go` | **MODIFY** | `AuthMode` type + `Config` con `CookieName`/`TokenTTL`/`JWTSecret` |
| `jwt_back.go` | **CREATE** `//go:build !wasm` | Primitivas JWT HMAC-SHA256 |
| `user_back.go` | **MODIFY** | `New()` branching en AuthMode |
| `migrate.go` | **MODIFY** | `initSchema` omite sessions en JWT mode |
| `middleware_back.go` | **MODIFY** | `validateSession` lee cookie, branching en mode |
| `module_login_back.go` | **MODIFY** | `SetCookie` — valor = sess.ID o JWT según mode |
| `module_register_back.go` | **MODIFY** | Ídem login |
| `tests/suite_back_test.go` | **MODIFY** | Renombrar campos + agregar TestJWTCookieMode |
| `export_test.go` | **CREATE** | Expone `generateJWT`/`validateJWT` para tests |
| `docs/SKILL.md` | **MODIFY** | Documenta nuevos modos de auth |
