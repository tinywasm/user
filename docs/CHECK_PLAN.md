# Plan: tinywasm/user — AuthModeBearer + mcp.Authorizer implementation

← Prerequisite for: [mcp PLAN](../../mcp/docs/PLAN.md) (implements mcp.Authorizer)

> **Scope note:** This plan is for projects that use **both** `tinywasm/user` AND
> `tinywasm/mcp` together (e.g., multi-user platforms with full RBAC).
> `tinywasm/app` does NOT use `tinywasm/user` — it uses `mcp.NewTokenAuthorizer`.

## References
- [ARCHITECTURE.md](ARCHITECTURE.md)
- `user/user.go` — `Config`, `AuthMode`, `SecurityEvent`, `SecurityEventType`
- `user/middleware_back.go` — `validateSession`, `Middleware`, `AccessCheck`
- `user/jwt_back.go` — `GenerateJWT`, `ValidateJWT`
- `user/cache_users.go` — user cache (FIFO, 1000 entries)
- `user/cache.go` — session cache

---

## Development Rules
- **SRP:** Every file must have a single, well-defined purpose.
- **Max 500 lines per file.**
- **No global state.** Use DI via interfaces.
- **Standard library only** in test assertions.
- **Test runner:** `gotest`. **Publish:** `gopush`.
- **Language:** Plans in English, chat in Spanish.
- **No code changes** until the user says "ejecuta" or "ok".

---

## Problem Summary

`tinywasm/mcp` defines an `Authorizer` interface:
```go
type Authorizer interface {
    InjectIdentity(ctx context.Context, r *http.Request) context.Context
    CanExecute(ctx context.Context, resource string, action byte) bool
}
```

`user.Module` must implement it **without importing tinywasm/mcp** (Go implicit
interface satisfaction — method signatures match exactly, no import needed).

MCP/IDE clients send `Authorization: Bearer <JWT>` header — not a cookie.
Currently `validateSession` only reads cookies, so `user.Module` cannot serve
API clients as-is.

---

## Design Decision: `AuthModeBearer` (NOT auto-detect)

**Why NOT auto-detect (previous plan's approach):**
Silently checking the Bearer header on every request before the cookie creates:
- Invisible behavior for existing cookie-mode users (unexpected code path)
- Mixed semantics: a browser endpoint that also silently accepts JWT headers
- Violation of the library's explicit config principle

**Why `AuthModeBearer` is correct:**
`AuthMode` is already the mechanism for selecting session strategy — it maps
directly to the architecture's *"Config struct: all configuration grouped logically"*.
`AuthModeBearer` follows the exact same pattern as the existing `AuthModeJWT`:

| Mode | Transport | Validation |
|------|-----------|------------|
| `AuthModeCookie` | Cookie (session ID) | DB session lookup |
| `AuthModeJWT` | Cookie (JWT value) | `ValidateJWT` |
| `AuthModeBearer` *(NEW)* | `Authorization: Bearer` header (JWT) | `ValidateJWT` |

The JWT validation logic is **identical** in both JWT modes — only the transport
(cookie vs. header) differs. A private `validateJWT(token)` helper is extracted
from the existing `AuthModeJWT` branch and reused.

**Performance:** `AuthModeBearer` has zero overhead for `AuthModeCookie` and
`AuthModeJWT` users — the switch never reaches the header read.
Auto-detect would call `r.Header.Get("Authorization")` on every single request.

**Intuitiveness:** one config field, one clear decision:
```go
user.Config{AuthMode: user.AuthModeBearer, JWTSecret: secret}
```

---

## MODIFY: `user/user.go`

Add `AuthModeBearer` constant and update `JWTSecret` comment:

```go
const (
    // AuthModeCookie stores a session ID in an HttpOnly cookie.
    // Stateful: requires user_sessions table. Supports immediate revocation.
    AuthModeCookie AuthMode = iota // default

    // AuthModeJWT stores a signed JWT in an HttpOnly cookie.
    // Stateless: no DB lookup per request. No immediate revocation.
    // Ideal for SPA/PWA and multi-server deployments.
    AuthModeJWT

    // AuthModeBearer reads a signed JWT from the "Authorization: Bearer <token>" header.
    // Stateless: for API clients (MCP servers, IDEs, LLMs) that cannot use cookies.
    // Structurally implements mcp.Authorizer via InjectIdentity + CanExecute methods.
    // Requires JWTSecret.
    AuthModeBearer
)

type Config struct {
    AuthMode AuthMode

    CookieName string
    TokenTTL   int

    // Required when AuthMode == AuthModeJWT or AuthMode == AuthModeBearer.
    // Also required to call GenerateAPIToken regardless of AuthMode.
    JWTSecret []byte

    TrustProxy         bool
    OAuthProviders     []OAuthProvider
    OnSecurityEvent    func(SecurityEvent)
    OnPasswordValidate func(password string) error
}
```

---

## MODIFY: `user/middleware_back.go`

### Extract private `validateJWT` helper

Shared by `AuthModeJWT` (cookie) and `AuthModeBearer` (header) — same logic,
different token source:

```go
// validateJWT validates a raw JWT string and returns the active user.
func (m *Module) validateJWT(token string) (*User, error) {
    userID, err := ValidateJWT(m.config.JWTSecret, token)
    if err != nil {
        m.notify(SecurityEvent{Type: EventJWTTampered, Timestamp: time.Now().Unix()})
        return nil, err
    }
    u, err := m.GetUser(userID)
    if err != nil {
        return nil, err
    }
    if u.Status != "active" {
        m.notify(SecurityEvent{Type: EventNonActiveAccess, UserID: u.ID, Timestamp: time.Now().Unix()})
        return nil, ErrSuspended
    }
    return &u, nil
}
```

### Update `validateSession` — add `AuthModeBearer` branch

New branch is explicit and isolated. Existing branches are **unchanged** except
`AuthModeJWT` now calls the shared `validateJWT` helper:

```go
func (m *Module) validateSession(r *http.Request) (*User, error) {
    // AuthModeBearer: API/MCP clients — JWT in Authorization header, no cookie.
    if m.config.AuthMode == AuthModeBearer {
        const prefix = "Bearer "
        auth := r.Header.Get("Authorization")
        if !strings.HasPrefix(auth, prefix) {
            return nil, ErrSessionExpired
        }
        return m.validateJWT(auth[len(prefix):])
    }

    // Cookie modes: browser clients.
    cookie, err := r.Cookie(m.config.CookieName)
    if err != nil {
        return nil, ErrSessionExpired
    }
    if m.config.AuthMode == AuthModeJWT {
        return m.validateJWT(cookie.Value) // reuses shared helper
    }

    // AuthModeCookie (default): stateful session ID in cookie.
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

### Add `InjectIdentity` and `CanExecute`

These satisfy `mcp.Authorizer` structurally. They delegate entirely to existing
methods — no new auth logic:

```go
// InjectIdentity implements mcp.Authorizer.
// Delegates to validateSession (respects configured AuthMode).
// On failure: returns ctx unchanged — CanExecute will deny.
func (m *Module) InjectIdentity(ctx context.Context, r *http.Request) context.Context {
    u, err := m.validateSession(r)
    if err != nil {
        m.notify(SecurityEvent{
            Type:      EventUnauthorizedAccess,
            IP:        clientIP(r),
            Timestamp: time.Now().Unix(),
        })
        return ctx
    }
    return context.WithValue(ctx, userKey, u)
}

// CanExecute implements mcp.Authorizer.
// Reads identity injected by InjectIdentity and checks RBAC.
func (m *Module) CanExecute(ctx context.Context, resource string, action byte) bool {
    u, ok := ctx.Value(userKey).(*User)
    if !ok || u == nil {
        return false
    }
    ok2, _ := m.HasPermission(u.ID, resource, action)
    if !ok2 {
        m.notify(SecurityEvent{
            Type:      EventAccessDenied,
            UserID:    u.ID,
            Resource:  resource,
            Timestamp: time.Now().Unix(),
        })
    }
    return ok2
}
```

---

## CREATE: `user/api_token_back.go`

SRP: owns API token generation and IP resolution. Separate from session middleware.

```go
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
```

---

## Usage example (for consuming application — NOT tinywasm/app)

```go
// One config field enables MCP compatibility:
userModule := user.New(db, user.Config{
    AuthMode:  user.AuthModeBearer,
    JWTSecret: myApp.LoadOrGenerateJWTSecret(),
})

// Create system user in DB (first run only)
// userModule.CreateUser("system-user-id", ...)

// Generate long-lived API token for IDE clients
apiKey, _ := userModule.GenerateAPIToken("system-user-id", 0)

// Wire into mcp.Handler — structural typing satisfies mcp.Authorizer
mcpHandler.SetAuth(userModule)
mcpHandler.SetAPIKey(apiKey) // written to IDE configs by ConfigureIDEs()
```

---

## Files to Modify / Create

| File | Action | Description |
|------|--------|-------------|
| `user/user.go` | **MODIFY** | Add `AuthModeBearer`; update `JWTSecret` comment |
| `user/middleware_back.go` | **MODIFY** | Extract `validateJWT` helper; add `AuthModeBearer` branch; add `InjectIdentity`, `CanExecute` |
| `user/api_token_back.go` | **CREATE** | `GenerateAPIToken` + `clientIP` |

---

## Execution Steps

### Step 1 — Modify `user/user.go`
Add `AuthModeBearer`. Update `JWTSecret` comment to cover both JWT modes.

### Step 2 — Modify `user/middleware_back.go`
- Extract `validateJWT(token string) (*User, error)` from existing `AuthModeJWT` branch
- Add `AuthModeBearer` branch to `validateSession`
- Update `AuthModeJWT` branch to call shared `validateJWT`
- Add `InjectIdentity` and `CanExecute`

### Step 3 — Create `user/api_token_back.go`

### Step 4 — Run tests and publish
```bash
gotest
gopush 'feat: AuthModeBearer for MCP/API clients, mcp.Authorizer implementation, GenerateAPIToken'
```

---

## Test Strategy

| Test | Validates |
|------|-----------|
| `TestValidateSession_Bearer_ValidJWT` | `AuthModeBearer` + valid JWT in header → returns user |
| `TestValidateSession_Bearer_InvalidJWT` | Tampered JWT → `ErrSessionExpired` + `EventJWTTampered` |
| `TestValidateSession_Bearer_SuspendedUser` | Valid JWT, suspended user → `ErrSuspended` |
| `TestValidateSession_Bearer_MissingHeader` | No Authorization header → `ErrSessionExpired` |
| `TestValidateSession_Cookie_Unchanged` | `AuthModeCookie` → existing logic unchanged |
| `TestValidateSession_JWT_Unchanged` | `AuthModeJWT` → cookie JWT unchanged, reuses validateJWT |
| `TestValidateSession_Bearer_NoCookieNeeded` | `AuthModeBearer` + no cookie → still valid |
| `TestInjectIdentity_ValidToken_InjectsUser` | Valid token → user in returned ctx |
| `TestInjectIdentity_InvalidToken_CtxUnchanged` | Invalid token → ctx unchanged + security event |
| `TestCanExecute_WithPermission_ReturnsTrue` | User has permission → true |
| `TestCanExecute_WithoutPermission_ReturnsFalse` | No permission → false + EventAccessDenied |
| `TestCanExecute_NoIdentity_ReturnsFalse` | Empty ctx → false (no panic) |
| `TestGenerateAPIToken_ValidToken_ValidatesCorrectly` | Generated token passes `ValidateJWT` |
| `TestGenerateAPIToken_NoSecret_ReturnsError` | Empty JWTSecret → error |
| `TestGenerateAPIToken_ZeroTTL_LongLived` | ttl=0 → very long expiry |
| `TestGenerateAPIToken_WorksWithCookieMode` | `AuthModeCookie` + JWTSecret → token generated (modes independent) |
