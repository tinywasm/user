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

| File | Responsibility | Key functions / exports |
|------|---------------|------------------------|
| `user.go` | Types, store singleton, `Init`, config | `Init(exec, cfg)`, `Config`, types `User`, `Session`, `Identity`, `Store` |
| `sql.go` | DB interfaces | `Executor`, `Scanner`, `Rows` |
| `migrate.go` | Schema creation | `runMigrations(exec)` — M001: 4 tables (no `password_hash`), M002: email nullable + `user_lan_ips` |
| `cache.go` | In-memory session cache | `sessionCache` (RWMutex map) |
| `crud.go` | User CRUD | `CreateUser`, `GetUser`, `GetUserByEmail`, `UpdateUser`, `SuspendUser`, `ReactivateUser` |
| `auth.go` | Local credential validation (identity-based) | `Login(email, password)`, `SetPassword`, `VerifyPassword` |
| `sessions.go` | Session lifecycle | `CreateSession`, `GetSession`, `DeleteSession`, `PurgeExpiredSessions` |
| `identities.go` | Identity management | `CreateIdentity`, `GetIdentityByProvider`, `GetUserIdentities`, `UnlinkIdentity` |
| `oauth.go` | OAuth flow + state management | `BeginOAuth`, `CompleteOAuth`, `consumeState`, `PurgeExpiredOAuthStates` |
| `google.go` | Google OAuth adapter | `GoogleProvider` (implements `OAuthProvider`) |
| `microsoft.go` | Microsoft OAuth adapter | `MicrosoftProvider` (implements `OAuthProvider`) |
| `lan.go` | LAN auth: RUT validation, IP extraction, `LoginLAN` | `LoginLAN`, `validateRUT`, `extractClientIP` |
| `lan_ips.go` | LAN identity & IP allowlist management | `RegisterLAN`, `UnregisterLAN` (explicit IP deletion + identity removal), `AssignLANIP`, `RevokeLANIP`, `GetLANIPs`, `LANIP` type |

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
| `module_oauth.go` | `!wasm` | `oauthModule` — `HandlerName()`, `RenderHTML()`, HTTP GET callback handler |

---

## Reference Code

### `user.go` — Store, Config, and Init

```go
package user

import "sync"

type Config struct {
    SessionCookieName string          // default: "session"
    SessionTTL        int             // default: 86400 (24h)
    TrustProxy        bool            // default: false
    OAuthProviders    []OAuthProvider
}

type Store struct {
    exec   Executor
    cache  *sessionCache
    config Config
    mu     sync.RWMutex
}

var store *Store

func Init(exec Executor, cfg Config) error {
    if cfg.SessionCookieName == "" { cfg.SessionCookieName = "session" }
    if cfg.SessionTTL == 0 { cfg.SessionTTL = 86400 }
    if err := runMigrations(exec); err != nil {
        return err
    }
    store = &Store{
        exec:   exec,
        cache:  newSessionCache(),
        config: cfg,
    }
    for _, p := range cfg.OAuthProviders {
        registerProvider(p)
    }
    return store.cache.warmUp(exec)
}

func SessionCookieName() string { return store.config.SessionCookieName }
```

### `auth.go` — Login (identity-based)

```go
// Login validates email+password via the user_identities table (provider='local').
// Returns ErrInvalidCredentials for: user not found, no local identity, wrong password.
// Returns ErrSuspended if user is suspended.
func Login(email, password string) (User, error) {
    u, err := GetUserByEmail(email)
    if err != nil {
        return User{}, ErrInvalidCredentials
    }
    if u.Status == "suspended" {
        return User{}, ErrSuspended
    }
    // Must have a local identity to authenticate via password
    identity, err := getLocalIdentity(u.ID)
    if err != nil {
        return User{}, ErrInvalidCredentials  // no local identity — OAuth/LAN-only user
    }
    if err := bcrypt.CompareHashAndPassword([]byte(identity.ProviderID), []byte(password)); err != nil {
        return User{}, ErrInvalidCredentials
    }
    return u, nil
}

// getLocalIdentity returns the 'local' identity for a user.
// provider_id stores the bcrypt hash of the password.
func getLocalIdentity(userID string) (Identity, error) {
    return getIdentityByUserAndProvider(userID, "local")
}

// SetPassword creates or updates the 'local' identity with a bcrypt hash.
// Returns ErrWeakPassword if password is less than 8 characters.
func SetPassword(userID, password string) error {
    if len(password) < 8 {
        return ErrWeakPassword
    }
    hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
    if err != nil { return err }
    // Upsert: if local identity exists, update provider_id; otherwise insert.
    return upsertIdentity(userID, "local", string(hash), "")
}

// VerifyPassword checks the current password without changing it.
func VerifyPassword(userID, password string) error {
    identity, err := getLocalIdentity(userID)
    if err != nil { return ErrInvalidCredentials }
    if err := bcrypt.CompareHashAndPassword([]byte(identity.ProviderID), []byte(password)); err != nil {
        return ErrInvalidCredentials
    }
    return nil
}
```

Note: `Login` deliberately returns `ErrInvalidCredentials` for "user not found",
"no local identity", and "wrong password" — prevents email enumeration attacks.
OAuth-only and LAN-only users have no local identity, so `Login(email, "")` correctly
fails with `ErrInvalidCredentials`.

### `sessions.go` — CreateSession

```go
const defaultTTL = 24 * 60 * 60  // 24 hours in seconds

func CreateSession(userID, ip, userAgent string) (Session, error) {
    u, err := unixid.NewUnixID()
    if err != nil { return Session{}, err }

    now := time.Now().Unix()
    sess := Session{
        ID:        u.GetNewID(),
        UserID:    userID,
        ExpiresAt: now + defaultTTL,
    }
    if err := store.exec.Exec(
        `INSERT INTO user_sessions (id, user_id, expires_at, ip, user_agent, created_at)
         VALUES (?, ?, ?, ?, ?, ?)`,
        sess.ID, sess.UserID, sess.ExpiresAt, ip, userAgent, now,
    ); err != nil {
        return Session{}, err
    }
    store.cache.set(sess.ID, sess)
    return sess, nil
}
```

### `crud.go` — CreateUser

```go
// nullableStr converts "" to nil so SQLite stores NULL instead of an empty string.
// Required because users.email is now nullable (M002).
func nullableStr(s string) any {
    if s == "" { return nil }
    return s
}

// CreateUser creates a user without any credentials.
// To enable local login, call SetPassword(userID, password) separately.
func CreateUser(email, name, phone string) (User, error) {
    u, err := unixid.NewUnixID()
    if err != nil { return User{}, err }

    id := u.GetNewID()
    now := time.Now().Unix()

    if err := store.exec.Exec(
        `INSERT INTO users (id, email, name, phone, created_at)
         VALUES (?, ?, ?, ?, ?)`,
        id, nullableStr(email), name, phone, now,
    ); err != nil {
        if isUniqueViolation(err) {
            return User{}, ErrEmailTaken
        }
        return User{}, err
    }
    return User{ID: id, Email: email, Name: name, Phone: phone, Status: "active", CreatedAt: now}, nil
}
```

### `oauth.go` — CompleteOAuth reference logic

```go
func CompleteOAuth(providerName string, r *http.Request, ip, ua string) (User, bool, error) {
    // 1. Validate CSRF state (single-use, 10 min TTL)
    state := r.URL.Query().Get("state")
    if err := consumeState(state, providerName); err != nil {
        return User{}, false, ErrInvalidOAuthState
    }
    // 2. Exchange authorization code for token
    p := getProvider(providerName)
    token, err := p.ExchangeCode(r.Context(), r.URL.Query().Get("code"))
    if err != nil { return User{}, false, err }
    // 3. Get user info from provider
    info, err := p.GetUserInfo(r.Context(), token)
    if err != nil { return User{}, false, err }
    // 4. Returning user? (known provider + provider_id)
    identity, err := GetIdentityByProvider(providerName, info.ID)
    if err == nil {
        u, err := GetUser(identity.UserID)
        return u, false, err
    }
    // 5. Local account with same email? Auto-link.
    u, err := GetUserByEmail(info.Email)
    if err == nil {
        _ = CreateIdentity(u.ID, providerName, info.ID, info.Email)
        return u, false, nil
    }
    // 6. New user — auto-register (no password, no local identity)
    u, err = CreateUser(info.Email, info.Name, "")
    if err != nil { return User{}, false, err }
    _ = CreateIdentity(u.ID, providerName, info.ID, info.Email)
    return u, true /* isNewUser */, nil
}
```

### Error sentinels

```go
var (
    ErrInvalidCredentials = fmt.Err("access", "denied")              // EN: Access Denied                    / ES: Acceso Denegado
    ErrSuspended          = fmt.Err("user", "suspended")             // EN: User Suspended                   / ES: Usuario Suspendido
    ErrEmailTaken         = fmt.Err("email", "registered")           // EN: Email Registered                 / ES: Correo electrónico Registrado
    ErrWeakPassword       = fmt.Err("password", "weak")              // EN: Password Weak                    / ES: Contraseña Débil
    ErrSessionExpired     = fmt.Err("token", "expired")              // EN: Token Expired                    / ES: Token Expirado
    ErrNotFound           = fmt.Err("user", "not", "found")          // EN: User Not Found                   / ES: Usuario No Encontrado
    ErrProviderNotFound   = fmt.Err("provider", "not", "found")      // EN: Provider Not Found               / ES: Proveedor No Encontrado
    ErrInvalidOAuthState  = fmt.Err("state", "invalid")              // EN: State Invalid                    / ES: Estado Inválido
    ErrCannotUnlink       = fmt.Err("identity", "cannot", "unlink")  // EN: Identity Cannot Unlink           / ES: Identidad No puede Desvincular
    ErrInvalidRUT         = fmt.Err("rut", "invalid")                // EN: Rut Invalid                      / ES: Rut Inválido
    ErrRUTTaken           = fmt.Err("rut", "registered")             // EN: Rut Registered                   / ES: Rut Registrado
    ErrIPTaken            = fmt.Err("ip", "registered")              // EN: Ip Registered                    / ES: Ip Registrado
    // Note: unauthorized IP on LoginLAN → ErrInvalidCredentials (intentional, no info leak)
)
```

### `lan.go` — LAN authentication

```go
// extractClientIP extracts the client IP, stripping the port.
// Uses store.config.TrustProxy to decide whether to trust proxy headers.
// trustProxy=false: uses r.RemoteAddr only.
// trustProxy=true:  checks X-Forwarded-For (first IP), then X-Real-IP, fallback RemoteAddr.
// WARNING: only enable TrustProxy if the server is behind a trusted reverse proxy.
// Without a trusted proxy, a client can spoof X-Forwarded-For to bypass IP checks.

// LoginLAN authenticates a LAN user by RUT + source IP.
// Returns ErrInvalidRUT if RUT format or check digit is invalid.
// Returns ErrInvalidCredentials if RUT not registered OR IP not in allowlist (no info leak).
// Returns ErrSuspended if the user account is suspended.
// Does NOT create a session — call CreateSession explicitly (same contract as Login).
func LoginLAN(rut string, r *http.Request) (User, error) {
    normalized, err := validateRUT(rut)
    if err != nil {
        return User{}, ErrInvalidRUT
    }
    identity, err := GetIdentityByProvider("lan", normalized)
    if err != nil {
        return User{}, ErrInvalidCredentials  // RUT not registered — same error as wrong IP
    }
    u, err := GetUser(identity.UserID)
    if err != nil {
        return User{}, ErrInvalidCredentials
    }
    if u.Status == "suspended" {
        return User{}, ErrSuspended
    }
    clientIP := extractClientIP(r, lanTrustProxy)
    if err := checkLANIP(identity.UserID, clientIP); err != nil {
        return User{}, ErrInvalidCredentials  // IP not allowed — no info leak
    }
    return u, nil
}

// validateRUT validates a Chilean RUT using the modulo-11 algorithm.
// Accepts: "12345678-9", "12.345.678-9", "12345678-K" (case-insensitive K).
// Returns normalized form: "12345678-9" (no dots, dash before check digit).
func validateRUT(rut string) (string, error) { ... }

// extractClientIP extracts the client IP, stripping the port.
// trustProxy=false: uses r.RemoteAddr only.
// trustProxy=true:  checks X-Forwarded-For (first IP), then X-Real-IP, fallback RemoteAddr.
func extractClientIP(r *http.Request, trustProxy bool) string { ... }
```

### `lan_ips.go` — LAN identity & IP management

```go
// LANIP represents a single authorized IP entry for a LAN user.
type LANIP struct {
    ID        string
    UserID    string
    IP        string
    Label     string
    CreatedAt int64
}

// RegisterLAN creates a 'lan' identity for userID with the given RUT.
// Validates and normalizes the RUT before storing.
// Returns ErrInvalidRUT or ErrRUTTaken.
func RegisterLAN(userID, rut string) error { ... }

// UnregisterLAN removes the 'lan' identity for userID and ALL assigned IPs.
// Explicitly deletes user_lan_ips rows first (FK references users.id, not identities).
// Returns ErrNotFound if no LAN identity exists for userID.
func UnregisterLAN(userID string) error { ... }

// AssignLANIP adds an IP to userID's LAN allowlist with an optional label.
// Returns ErrIPTaken if the IP is already assigned to any user.
func AssignLANIP(userID, ip, label string) error { ... }

// RevokeLANIP removes an IP from userID's allowlist.
// Returns ErrNotFound if the IP is not in userID's list.
// Scoped by user_id to prevent revoking another user's IP.
func RevokeLANIP(userID, ip string) error { ... }

// GetLANIPs returns all IPs assigned to userID, ordered by created_at ASC.
// Returns an empty slice (not an error) when no IPs are assigned.
func GetLANIPs(userID string) ([]LANIP, error) { ... }

// checkLANIP is an internal helper used by LoginLAN.
// Returns nil if ip is in userID's allowlist, error otherwise.
func checkLANIP(userID, ip string) error { ... }
```

---

## Module System — Reference Code

### `forms.go` — Form data structs

Zero struct tags — field names (`Email`, `Password`, `Name`, `Phone`, `Current`, `New`,
`Confirm`) are already in `tinywasm/fmt/dictionary` and auto-translated by `tinywasm/form`.

```go
// forms.go — shared (no build tag)
package user

// LoginData is validated by LoginModule on both frontend and backend.
type LoginData struct {
    Email    string
    Password string
}

// RegisterData is validated by RegisterModule.
type RegisterData struct {
    Name     string
    Email    string
    Password string
    Phone    string
}

// ProfileData is validated by ProfileModule (name/phone update).
type ProfileData struct {
    Name  string
    Phone string
}

// PasswordData is validated by ProfileModule (password change sub-form).
type PasswordData struct {
    Current string
    New     string
    Confirm string
}
```

### `modules.go` — Package-level module vars

```go
// modules.go — shared (no build tag)
package user

import "github.com/tinywasm/form"

// mustForm creates a form for the given parent ID and struct.
// Panics at startup if the struct has unmatchable fields — fails early.
func mustForm(parentID string, s any) *form.Form {
    f, err := form.New(parentID, s)
    if err != nil {
        panic("user: mustForm: " + err.Error())
    }
    return f
}

// Module vars — configured once at package init, registered by the application.
// All implement site.Module via duck typing (no import of tinywasm/site).
var (
    LoginModule    = &loginModule{form: mustForm("login", &LoginData{})}
    RegisterModule = &registerModule{form: mustForm("register", &RegisterData{})}
    ProfileModule  = &profileModule{
        form:         mustForm("profile", &ProfileData{}),
        passwordForm: mustForm("password", &PasswordData{}),
    }
    LANModule     = &lanModule{}
    OAuthCallback = &oauthModule{}
)
```

### `module_login.go` — Shared module definition

```go
// module_login.go — shared (no build tag)
package user

import "github.com/tinywasm/form"

// loginModule implements site.Module via duck typing.
// ValidateData satisfies crudp.DataValidator — used by crudp handlers on the backend.
// Thread-safe: form.Form.ValidateData reads only pre-computed indices, never writes.
type loginModule struct {
    form *form.Form
}

func (m *loginModule) HandlerName() string { return "login" }
func (m *loginModule) ModuleTitle() string { return "Login" }

// ValidateData delegates to form.Form.ValidateData — same rules as frontend live validation.
// action byte is ignored for auth forms (same rules for create 'c' and other actions).
func (m *loginModule) ValidateData(action byte, data ...any) error {
    return m.form.ValidateData(action, data...)
}
```

### `module_login_back.go` — SSR render + HTTP handler

```go
// module_login_back.go
//go:build !wasm

package user

import "net/http"

// RenderHTML renders the login form for SSR (initial page load).
// OAuth buttons are appended if providers are registered.
func (m *loginModule) RenderHTML() string {
    m.form.SetSSR(true)
    out := m.form.RenderHTML()
    // Append OAuth provider buttons if any are registered.
    for _, p := range registeredProviders() {
        out += `<a href="/oauth/` + p.Name() + `">Login with ` + p.Name() + `</a>`
    }
    return out
}

// Create handles POST /login: validate → login → create session → set cookie.
// Called by the crudp handler after ValidateData passes.
func (m *loginModule) Create(data ...any) (any, error) {
    if len(data) == 0 {
        return nil, ErrInvalidCredentials
    }
    d, ok := data[0].(*LoginData)
    if !ok {
        return nil, ErrInvalidCredentials
    }
    u, err := Login(d.Email, d.Password)
    if err != nil {
        return nil, err
    }
    return u, nil
}

// SetCookie writes the session cookie. Called by the crudp handler after Create.
func (m *loginModule) SetCookie(userID string, w http.ResponseWriter, r *http.Request) error {
    sess, err := CreateSession(userID, extractClientIP(r, lanTrustProxy), r.UserAgent())
    if err != nil {
        return err
    }
    http.SetCookie(w, &http.Cookie{
        Name:     sessionCookieName,
        Value:    sess.ID,
        HttpOnly: true,
        Secure:   true,
        SameSite: http.SameSiteStrictMode,
        MaxAge:   sessionTTL,
        Path:     "/",
    })
    return nil
}
```

### `module_login_front.go` — WASM interactivity

```go
// module_login_front.go
//go:build wasm

package user

// OnMount wires DOM events for live validation and form submission.
// Called by tinywasm/dom after RenderHTML is injected into the page.
func (m *loginModule) OnMount() {
    m.form.OnMount()
}
```

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

## Test Strategy (DDT)

Test files follow the build-tag split pattern:

| File | Tag | Purpose |
|------|-----|---------|
| `setup_test.go` | — | Shared: in-memory DB adapter, `RunUserTests(t)` |
| `user_back_test.go` | `!wasm` | Runs `RunUserTests(t)` + module SSR tests |
| `user_front_test.go` | `wasm` | Runs `RunUserTests(t)` + module OnMount tests |

### MODULE coverage

```go
// LoginModule
func TestLoginModule_HandlerName(t *testing.T)            // returns "login"
func TestLoginModule_ValidateData_Valid(t *testing.T)     // valid LoginData → nil
func TestLoginModule_ValidateData_BadEmail(t *testing.T)  // bad email → error
func TestLoginModule_ValidateData_NoData(t *testing.T)    // no args → nil
func TestLoginModule_RenderHTML_ContainsForm(t *testing.T) // SSR HTML contains form + inputs

// RegisterModule
func TestRegisterModule_ValidateData_Valid(t *testing.T)   // valid RegisterData → nil
func TestRegisterModule_ValidateData_ShortName(t *testing.T) // name < 2 chars → error
func TestRegisterModule_RenderHTML_ContainsForm(t *testing.T)

// ProfileModule
func TestProfileModule_ValidateData_Valid(t *testing.T)
func TestProfileModule_ValidateData_InvalidPhone(t *testing.T) // non-digits in phone → error
func TestProfileModule_RenderHTML_ContainsForm(t *testing.T)

// LANModule
func TestLANModule_HandlerName(t *testing.T)               // returns "lan"
func TestLANModule_RenderHTML_ContainsTable(t *testing.T)  // SSR HTML contains IP table

// OAuthCallback
func TestOAuthCallback_HandlerName(t *testing.T)           // returns "oauth/callback"
```

### AUTH_FLOW coverage

```go
func TestLogin_ValidCredentials(t *testing.T)        // user with local identity → returns User
func TestLogin_InvalidPassword(t *testing.T)         // returns ErrInvalidCredentials
func TestLogin_UserNotFound(t *testing.T)            // returns ErrInvalidCredentials (no enum)
func TestLogin_NoLocalIdentity(t *testing.T)         // OAuth-only user → ErrInvalidCredentials
func TestLogin_SuspendedUser(t *testing.T)           // returns ErrSuspended
func TestSetPassword_Success(t *testing.T)           // creates local identity → Login works
func TestSetPassword_WeakPassword(t *testing.T)      // < 8 chars → ErrWeakPassword
func TestSetPassword_Update(t *testing.T)            // updates existing hash → new password works
func TestVerifyPassword_Correct(t *testing.T)        // correct password → nil
func TestVerifyPassword_Wrong(t *testing.T)          // wrong password → ErrInvalidCredentials
```

### SESSION_FLOW coverage

```go
func TestSession_CreateAndGet(t *testing.T)        // create → get → valid
func TestSession_Expired(t *testing.T)             // expired → ErrSessionExpired
func TestSession_Delete(t *testing.T)              // delete → subsequent get → error
func TestSession_CacheHit(t *testing.T)            // second get served from cache
func TestPurgeExpiredSessions(t *testing.T)        // purge → expired sessions removed from DB + cache
```

### USER_CRUD_FLOW coverage

```go
func TestCreateUser_Success(t *testing.T)          // creates user (no password)
func TestCreateUser_DuplicateEmail(t *testing.T)   // returns ErrEmailTaken
func TestGetUserByEmail_NotFound(t *testing.T)     // returns ErrNotFound
func TestSuspendUser(t *testing.T)                 // status becomes "suspended"
func TestReactivateUser(t *testing.T)              // status becomes "active"
func TestSuspendedUser_CannotLogin(t *testing.T)   // Login returns ErrSuspended
func TestUpdateUser(t *testing.T)                  // name and phone updated
func TestCreateUser_ThenSetPassword(t *testing.T)  // CreateUser + SetPassword → Login works
```

### OAUTH_FLOW coverage

```go
func TestOAuth_NewUser_AutoRegister(t *testing.T)        // unknown email → create user + isNewUser=true
func TestOAuth_ExistingOAuthUser(t *testing.T)           // known (provider, id) → user, isNewUser=false
func TestOAuth_LinkToLocalAccount(t *testing.T)          // OAuth email matches local → link, isNewUser=false
func TestOAuth_InvalidState(t *testing.T)                // wrong state → ErrInvalidOAuthState
func TestOAuth_ExpiredState(t *testing.T)                // state > 10 min old → ErrInvalidOAuthState
func TestOAuth_StateConsumedOnce(t *testing.T)           // replay attack: second use → error
func TestUnlinkIdentity_LastIdentity(t *testing.T)       // → ErrCannotUnlink
func TestUnlinkIdentity_MultipleIdentities(t *testing.T) // → nil
```

### LAN_AUTH_FLOW coverage

```go
func TestLoginLAN_ValidRUT_ValidIP(t *testing.T)              // valid RUT + authorized IP → User
func TestLoginLAN_InvalidRUTFormat(t *testing.T)              // malformed string → ErrInvalidRUT
func TestLoginLAN_InvalidCheckDigit(t *testing.T)             // correct format, wrong check digit → ErrInvalidRUT
func TestLoginLAN_RUTNotFound(t *testing.T)                   // valid RUT, not registered → ErrInvalidCredentials (no enum)
func TestLoginLAN_IPNotAssigned(t *testing.T)                 // valid RUT, wrong IP → ErrInvalidCredentials (no enum)
func TestLoginLAN_SuspendedUser(t *testing.T)                 // valid RUT + IP, suspended → ErrSuspended
func TestLoginLAN_TrustProxy_ValidIP(t *testing.T)            // Config.TrustProxy=true, IP from X-Forwarded-For → User
func TestLoginLAN_TrustProxy_False_IgnoresHeader(t *testing.T) // Config.TrustProxy=false, X-Forwarded-For set → uses RemoteAddr
```

### LAN_IP_FLOW coverage

```go
func TestRegisterLAN_Success(t *testing.T)         // valid RUT, no prior identity → nil + identity in DB
func TestRegisterLAN_InvalidRUT(t *testing.T)      // bad check digit → ErrInvalidRUT
func TestRegisterLAN_RUTTaken(t *testing.T)        // RUT already linked to another user → ErrRUTTaken
func TestAssignLANIP_Success(t *testing.T)         // new IP → nil + row in user_lan_ips
func TestAssignLANIP_IPTaken(t *testing.T)         // IP already assigned to another user → ErrIPTaken
func TestRevokeLANIP_Success(t *testing.T)         // existing IP → nil + row removed
func TestRevokeLANIP_NotFound(t *testing.T)        // IP not in user's list → ErrNotFound
func TestGetLANIPs_MultipleIPs(t *testing.T)       // 3 IPs assigned → []LANIP len=3, ordered by created_at
func TestGetLANIPs_Empty(t *testing.T)             // no IPs → []LANIP len=0 (not an error)
func TestUnregisterLAN_RemovesAll(t *testing.T)    // identity + IPs deleted → GetLANIPs returns empty slice
```

---

## Publishing

Before publishing:
1. All tests pass: `gotest ./...`
2. Documentation updated (this file + diagrams reflect actual implementation)
3. Publish: `gopush` (`github.com/tinywasm/devflow/cmd/gopush`)
4. Update `site/go.mod` to use new version (remove `replace` directive)
