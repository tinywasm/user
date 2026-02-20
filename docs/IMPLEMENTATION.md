# Implementation

> **Status:** Design — February 2026

---

## Development Rules

> Copied from project CLAUDE.md — relevant rules for this isomorphic library.

- **Flat Hierarchy:** No subdirectories. All files in `tinywasm/user/` root.
- **Max 500 lines per file.** Split by domain if exceeded.
- **No external assertion libraries:** Testing with `testing` + `net/http/httptest` only. No testify.
- **Mandatory DI:** No global state beyond the singleton store (same pattern as rbac).
  All external dependencies injected via `Executor` interface.
- **DDT (Diagram-Driven Testing):** Every flow in `docs/diagrams/` MUST have an integration
  test covering every branch (diamonds) and failure mode.
- **Standard Library only** (exceptions: `golang.org/x/crypto/bcrypt` for passwords,
  `golang.org/x/oauth2` for OAuth token exchange in built-in providers).
- **Frontend Go Compatibility:** Module files compiled to WASM must use `tinywasm/fmt`
  instead of `fmt`/`strings`/`strconv`, `tinywasm/time` instead of `time`.
  No maps in WASM code — use slices or structs instead.
- **No maps in WASM module files.** Build-tag the map-heavy backend code with `!wasm`.
- **Testing:** Use `gotest` (`github.com/tinywasm/devflow/cmd/gotest`).

---

## File Layout

### Backend (build: `!wasm` or untagged)

| File | Build tag | Responsibility | Key functions / exports |
|------|-----------|----------------|-------------------------|
| `user.go` | — (shared) | Types, errors, configuration | `User`, `Session`, `Identity`, `Config`, `SessionCookieName()` |
| `user_back.go` | `!wasm` | Backend state & singleton | `Init(exec, cfg)`, `Store` |
| `sql.go` | `!wasm` | DB interfaces | `Executor`, `Scanner`, `Rows` |
| `migrate.go` | `!wasm` | Schema creation | `runMigrations(exec)` |
| `cache.go` | `!wasm` | In-memory session cache | `sessionCache` |
| `crud.go` | `!wasm` | User CRUD | `CreateUser`, `GetUser`, `UpdateUser`, etc. |
| `auth.go` | `!wasm` | Credential validation | `Login`, `SetPassword`, `VerifyPassword` |
| `sessions.go` | `!wasm` | Session lifecycle | `CreateSession`, `GetSession`, `DeleteSession` |
| `identities.go` | `!wasm` | Identity management | `CreateIdentity`, `UnlinkIdentity` |
| `oauth.go` | `!wasm` | OAuth management | `BeginOAuth`, `CompleteOAuth`, `consumeState` |
| `google.go` | `!wasm` | Google adapter | `GoogleProvider` |
| `microsoft.go` | `!wasm` | Microsoft adapter | `MicrosoftProvider` |
| `lan.go` | `!wasm` | LAN auth logic | `LoginLAN`, `validateRUT` |
| `lan_ips.go` | `!wasm` | LAN IP management | `RegisterLAN`, `AssignLANIP`, `RevokeLANIP` |

### Isomorphic module system (shared + build-tagged)

| File | Build tag | Responsibility |
|------|-----------|----------------|
| `forms.go` | — (shared) | Form data structs: `LoginData`, `RegisterData`, `ProfileData`, `PasswordData` |
| `modules.go` | — (shared) | Package-level vars: `LoginModule`, `RegisterModule`, `ProfileModule`, `LANModule`, `OAuthCallback` |
| `module_login.go` | — (shared) | `loginModule` struct, `HandlerName()`, `ModuleTitle()`, `ValidateData()` |
| `module_login_back.go` | `!wasm` | `RenderHTML()` (SSR), HTTP POST handler (login → session → cookie) |
| `module_login_front.go` | `wasm` | `OnMount()` — form interactivity, live validation |
| `module_register.go` | — (shared) | `registerModule` struct, `HandlerName()`, `ModuleTitle()`, `ValidateData()` |
| `module_register_back.go` | `!wasm` | `RenderHTML()` (SSR), HTTP POST handler (create user → session → cookie) |
| `module_register_front.go` | `wasm` | `OnMount()` — form interactivity, live validation |
| `module_profile.go` | — (shared) | `profileModule` struct, `HandlerName()`, `ModuleTitle()`, `ValidateData()` |
| `module_profile_back.go` | `!wasm` | `RenderHTML()` (SSR), HTTP POST handler (update user + optional password change) |
| `module_profile_front.go` | `wasm` | `OnMount()` — form interactivity |
| `module_lan.go` | — (shared) | `lanModule` struct, `HandlerName()`, `ModuleTitle()` |
| `module_lan_back.go` | `!wasm` | `RenderHTML()` (SSR, IP list table), HTTP POST/DELETE handlers |
| `module_lan_front.go` | `wasm` | `OnMount()` — add/remove IP rows |
| `module_oauth.go` | — (shared) | `oauthModule` struct, `HandlerName()`, `ModuleTitle()` |
| `module_oauth_back.go` | `!wasm` | `RenderHTML()`, HTTP GET callback handler |

---

## Reference Code

### `user.go` — Store, Config, and Init

[Implemented in user.go](../user.go)

### `auth.go` — Login (identity-based)

[Implemented in auth.go](../auth.go)

### `sessions.go` — CreateSession

[Implemented in sessions.go](../sessions.go)

### `crud.go` — CreateUser

[Implemented in crud.go](../crud.go)

### `oauth.go` — CompleteOAuth reference logic

[Implemented in oauth.go](../oauth.go)

### Error sentinels

[Implemented in user.go](../user.go)

### `lan.go` — LAN authentication

[Implemented in lan.go](../lan.go)

### `lan_ips.go` — LAN identity & IP management

[Implemented in lan_ips.go](../lan_ips.go)

---

## Module System — Reference Code

### `forms.go` — Form data structs

[Implemented in forms.go](../forms.go)

### `modules.go` — Package-level module vars

[Implemented in modules.go](../modules.go)

### `module_login.go` — Shared module definition

[Implemented in module_login.go](../module_login.go)

### `module_login_back.go` — SSR render + HTTP handler

[Implemented in module_login_back.go](../module_login_back.go)

### `module_login_front.go` — WASM interactivity

[Implemented in module_login_front.go](../module_login_front.go)

---

## Site Integration — Application Setup

Modules self-handle their HTTP routes and session cookie. The application registers them
alongside app modules — no site proxy layer needed.

```go
// main.go — application setup

// 1. Configure site (DB shared with user via applyUser internally)
site.SetDB(db)
site.SetUserID(extractUserID)   // reads session cookie → calls user.GetSession
site.CreateRole('a', "Admin", "full access")
site.CreateRole('v', "Visitor", "read only")

// 2. Configure user via Config struct (all optional, zero values = defaults)
site.SetUserConfig(user.Config{
    SessionCookieName: "s",
    SessionTTL:        86400,
    TrustProxy:        true,
    OAuthProviders: []user.OAuthProvider{
        &user.GoogleProvider{
            ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
            ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
            RedirectURL:  "https://example.com/oauth/callback",
        },
    },
})

// 3. Register user modules alongside app modules
site.RegisterHandlers(
    user.LoginModule,    // handles /login
    user.RegisterModule, // handles /register
    user.ProfileModule,  // handles /profile
    user.LANModule,      // handles /lan
    user.OAuthCallback,  // handles /oauth/callback
    &myapp.Dashboard{},
    &myapp.Contact{},
)

site.Serve(":8080")
// site.Serve internally calls:
//   applyUser() → user.Init(dbExecutor, cfg)   (runs migrations, warms cache)
//   applyRBAC() → rbac.Init(dbExecutor)

// extractUserID reads the session cookie and returns the user ID.
// Used by site to populate the request context on every request.
func extractUserID(args ...any) string {
    r, ok := args[0].(*http.Request)
    if !ok {
        return ""
    }
    c, err := r.Cookie(user.SessionCookieName())
    if err != nil {
        return ""
    }
    sess, err := user.GetSession(c.Value)
    if err != nil {
        return ""
    }
    return sess.UserID
}
```

**After OAuth registration, assign default role:**
```go
// In your app's OAuth post-processing (triggered by OAuthCallback.isNewUser flag):
if isNewUser {
    site.AssignRole(u.ID, 'v')  // rbac — completely independent of user lib
}
```

---

## Testing Strategy

The test suite follows the **WASM/Stlib Dual Testing Pattern** to ensure isomorphic compatibility.

### File Structure

| File | Build tag | Purpose |
|------|-----------|---------|
| `tests/setup_test.go` | — (shared) | Shared: module checks, `RunSharedTests(t)` |
| `tests/suite_back_test.go` | `!wasm` | Backend-only: DB helpers, CRUD/Auth logic, `RunUserTests(t)` |
| `tests/user_back_test.go` | `!wasm` | Backend entry point: calls shared + backend suites |
| `tests/user_front_test.go` | `wasm` | Frontend entry point: calls shared suite |

### Coverage References

- **MODULE**: `tests/setup_test.go`
- **AUTH_FLOW**, **SESSION_FLOW**, **CRUD_FLOW**, **OAUTH_FLOW**, **LAN_FLOW**: `tests/suite_back_test.go`

---

## Publishing

Before publishing:
1. All tests pass: `gotest ./...`
2. Documentation updated (this file + diagrams reflect actual implementation)
3. Publish: `gopush` (`github.com/tinywasm/devflow/cmd/gopush`)
4. Update `site/go.mod` to use new version (remove `replace` directive)
