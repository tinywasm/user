# tinywasm/user — Enhancement Plan (Module Pattern + CRUDP + Middleware)

> **Goal:** Refactor the package to eliminate global state, expose `New()` returning `*Module`
> with explicit DI, implement typed CRUDP entity methods with `db *orm.DB`, and export a
> unified middleware contract that becomes the ecosystem-wide security standard.
>
> **Note:** `tinywasm/rbac` has been archived. Its functionality (`Register`, `HasPermission`,
> `CreateRole`, `AssignPermission`, etc.) is already present in `user_rbac_mutations.go`
> using the global `store`. This plan migrates those to the `*Module` pattern.
>
> **Status:** Pending execution

---

## Development Rules

- **Testing Runner:** `go install github.com/tinywasm/devflow/cmd/gotest@latest`
- **Build Tags:** Backend files must use `//go:build !wasm`. Isomorphic files have no tag.
- **SRP:** One domain per file. Max 500 lines — subdivide if exceeded.
- **DI — No Global State:** `New()` is the only entry point. The `*Module` holds all state.
- **Breaking Changes Allowed:** API fluidity for the ecosystem takes priority over compatibility.
- **Standard Library Only:** No external assertion libraries in tests.
- **Prerequisite:** `tinywasm/crudp` must expose `Register(db *orm.DB, handlers ...any)` that
  calls entity methods with the db it received. See `tinywasm/crudp/docs/PLAN.md`.

---

## Step 1 — Replace Global `store` with `New()` Returning `*Module`

**Target Files:** `user_back.go`, `user.go`

Remove `var store *Store` and `func Init(db *orm.DB, cfg Config) error`.
Replace with `New()` that returns an explicit `*Module` handle. This is the ONLY entry point
for the backend.

```go
//go:build !wasm

package user

import (
    "sync"
    "github.com/tinywasm/orm"
)

// Module is the user/auth/rbac handle. All backend operations are methods on this type.
// Created exclusively via New().
type Module struct {
    db     *orm.DB
    cache  *sessionCache
    ucache *userCache
    config Config
    log    func(...any)
    mu     sync.RWMutex
}

// New initializes the user/rbac schema, warms the cache, and returns a Module handle.
// This is the ONLY entry point for this package on the backend.
func New(db *orm.DB, cfg Config) (*Module, error) {
    if cfg.SessionCookieName == "" {
        cfg.SessionCookieName = "session"
    }
    if cfg.SessionTTL == 0 {
        cfg.SessionTTL = 86400
    }
    m := &Module{
        db:     db,
        cache:  newSessionCache(),
        ucache: newUserCache(),
        config: cfg,
    }
    if err := m.initSchema(); err != nil {
        return nil, err
    }
    for _, p := range cfg.OAuthProviders {
        m.registerProvider(p)
    }
    if err := m.cache.warmUp(db); err != nil {
        return nil, err
    }
    return m, nil
}

// SetLog configures optional logging. Call immediately after New().
// Default: no-op. Follows the tinywasm ecosystem SetLog convention (same as rbac).
//
// Example:
//   m.SetLog(func(msg ...any) { log.Println(msg...) })
func (m *Module) SetLog(fn func(...any)) {
    m.log = fn
}
```

**Breaking change checklist:**
- Delete `var store *Store`
- Delete `func Init(db *orm.DB, cfg Config) error`
- Delete package-level `var sessionCookieName` mutation
- All references to `store.db`, `store.cache`, `store.ucache` become `m.*`

---

## Step 2 — Refactor Internal Functions to Explicit `db *orm.DB`

**Target Files:** `user_back.go`, `crud.go`, `sessions.go`, `auth.go`, `lan.go`,
`lan_ips.go`, `identities.go`, `oauth.go`, `user_rbac_mutations.go`

All functions that accessed `store.db` must migrate to one of two patterns:

| Pattern | When to use | Signature |
|---|---|---|
| **Module method** | Public API called by consuming apps | `func (m *Module) GetUser(id string) (User, error)` |
| **Unexported helper** | Internal logic used by CRUDP entity methods | `func getUser(db *orm.DB, cache *userCache, id string) (User, error)` |

*Example:*
```go
// Public API — Module method
func (m *Module) GetUser(id string) (User, error)            { return getUser(m.db, m.ucache, id) }
func (m *Module) HasPermission(userID, resource string, action byte) (bool, error) { ... }
func (m *Module) Register(handlers ...RBACObject) error      { return registerRBAC(m.db, handlers...) }

// Internal helpers (unexported) — consumed by CRUDP entity methods in Step 3
func getUser(db *orm.DB, cache *userCache, id string) (User, error)           { ... }
func createUser(db *orm.DB, email, name, phone string) (User, error)          { ... }
func listUsers(db *orm.DB) ([]User, error)                                    { ... }
func updateUser(db *orm.DB, id, name, phone string) error                     { ... }
func deleteUser(db *orm.DB, id string) error                                  { ... }
```

Apply the same migration to all RBAC functions:
`CreateRole`, `GetRole`, `DeleteRole`, `CreatePermission`, `GetPermission`, `DeletePermission`,
`AssignRole`, `RevokeRole`, `AssignPermission`, `GetRoleByCode`, `GetUserRoles`.

---

## Step 3 — CRUDP Handler Wrappers (Private Types)

**Target File:** `models_crud_back.go` (create new, `//go:build !wasm`)

Create private handler wrapper types that capture `*orm.DB` in their constructor.
The domain model structs (`User`, `Role`, etc.) remain pure data types with no CRUDP
methods — separation of domain vs. infrastructure.

**Design decision: wrappers are private (`userCRUD`, not `UserCRUD`).**
The consumer only calls `crudp.RegisterHandlers(m.Add()...)` — it never needs to inspect
or use the concrete wrapper type. Exporting them would pollute the package's public API
with infrastructure details.

Each wrapper implements the `tinywasm/crudp` interfaces:
`Creator`, `Reader` (including `List()`), `Updater`, `Deleter`, `NamedHandler`,
`DataValidator`, `AccessLevel`.

```go
//go:build !wasm

package user

import "github.com/tinywasm/orm"

// --- userCRUD ---
type userCRUD struct{ db *orm.DB }

func (h *userCRUD) HandlerName() string                    { return "users" }
func (h *userCRUD) AllowedRoles(action byte) []byte        { return []byte{'a'} }
func (h *userCRUD) ValidateData(action byte, _ any) error  { return nil }

func (h *userCRUD) Create(payload any) (any, error) {
    u := payload.(User)
    return createUser(h.db, u.Email, u.Name, u.Phone)
}
func (h *userCRUD) Read(id string) (any, error)  { return getUser(h.db, nil, id) }
func (h *userCRUD) List() (any, error)           { return listUsers(h.db) }
func (h *userCRUD) Update(payload any) (any, error) {
    u := payload.(User)
    return u, updateUser(h.db, u.ID, u.Name, u.Phone)
}
func (h *userCRUD) Delete(id string) error       { return deleteUser(h.db, id) }

// --- roleCRUD ---
type roleCRUD struct{ db *orm.DB }

func (h *roleCRUD) HandlerName() string                    { return "roles" }
func (h *roleCRUD) AllowedRoles(action byte) []byte        { return []byte{'a'} }
func (h *roleCRUD) ValidateData(action byte, _ any) error  { return nil }

func (h *roleCRUD) Create(payload any) (any, error) { ... }
func (h *roleCRUD) Read(id string) (any, error)     { ... }
func (h *roleCRUD) List() (any, error)              { ... }
func (h *roleCRUD) Update(payload any) (any, error) { ... }
func (h *roleCRUD) Delete(id string) error          { ... }

// --- permissionCRUD ---
type permissionCRUD struct{ db *orm.DB }

func (h *permissionCRUD) HandlerName() string                    { return "permissions" }
func (h *permissionCRUD) AllowedRoles(action byte) []byte        { return []byte{'a'} }
func (h *permissionCRUD) ValidateData(action byte, _ any) error  { return nil }

func (h *permissionCRUD) Create(payload any) (any, error) { ... }
func (h *permissionCRUD) Read(id string) (any, error)     { ... }
func (h *permissionCRUD) List() (any, error)              { ... }
func (h *permissionCRUD) Update(payload any) (any, error) { ... }
func (h *permissionCRUD) Delete(id string) error          { ... }

// --- lanipCRUD ---
type lanipCRUD struct{ db *orm.DB }

func (h *lanipCRUD) HandlerName() string                    { return "lan_ips" }
func (h *lanipCRUD) AllowedRoles(action byte) []byte        { return []byte{'a'} }
func (h *lanipCRUD) ValidateData(action byte, _ any) error  { return nil }

func (h *lanipCRUD) Create(payload any) (any, error) { ... }
func (h *lanipCRUD) Read(id string) (any, error)     { ... }
func (h *lanipCRUD) List() (any, error)              { ... }
func (h *lanipCRUD) Update(payload any) (any, error) { ... }
func (h *lanipCRUD) Delete(id string) error          { ... }
```

---

## Step 4 — Export `Add()` and `UIModules()`

**Target Files:** `user_back.go` (`Add`) and `modules.go` (`UIModules`)

`Add()` is a `*Module` method returning the private CRUDP handler wrappers.
The concrete types are private (`userCRUD`, etc.) — the consumer treats them as opaque
`any` values and passes them directly to `crudp.RegisterHandlers`. This keeps the
package's public API clean: only domain types are exported, not infrastructure wrappers.

```go
//go:build !wasm

// Add returns all admin-managed CRUDP handlers for registration.
// The concrete types are private — pass directly to crudp.RegisterHandlers.
//
// Usage: cp.RegisterHandlers(m.Add()...)
func (m *Module) Add() []any {
    return []any{
        &userCRUD{db: m.db},
        &roleCRUD{db: m.db},
        &permissionCRUD{db: m.db},
        &lanipCRUD{db: m.db},
    }
}
```

`UIModules()` is a package-level function (isomorphic, no build tag). It replaces the
mutable exported globals (`LoginModule`, `RegisterModule`, etc.) in `modules.go`:

```go
// modules.go (no build tag — isomorphic)

var uiModules []any

func init() {
    form.RegisterInput(
        input.Password("", "current"),
        input.Password("", "new"),
        input.Password("", "confirm"),
    )
    uiModules = []any{
        &loginModule{form: mustForm("login", &LoginData{})},
        &registerModule{form: mustForm("register", &RegisterData{})},
        &profileModule{
            form:         mustForm("profile", &ProfileData{}),
            passwordForm: mustForm("password", &PasswordData{}),
        },
        &lanModule{},
        &oauthModule{},
    }
}

// UIModules returns all standard authentication UI flow handlers.
// Isomorphic: available in both WASM and non-WASM builds.
func UIModules() []any { return uiModules }
```

**Breaking change:** Remove exported `LoginModule`, `RegisterModule`, `ProfileModule`,
`LANModule`, `OAuthCallback` vars.

---

## Step 5 — Middleware & Ecosystem Integration Methods

**Target File:** `middleware_back.go` (create new, `//go:build !wasm`)

The unified tinywasm ecosystem middleware contract. Every security library in the ecosystem
exposes these methods on its module handle. This is the convention.

```go
//go:build !wasm

package user

import (
    "context"
    "net/http"
    mcplib "github.com/tinywasm/mcp"
)

type contextKey int
const userKey contextKey = iota

// RegisterMCP registers user authentication context funcs on the MCP server.
// Covers both SSE and HTTP transports in one call.
// The module knows which MCP hooks to register — the caller does not need to
// know about mcp.WithHTTPContextFunc or mcp.WithSSEContextFunc.
//
// Call after mcp.NewMCPServer() and before serving.
//
// Example:
//   srv := mcp.NewMCPServer("app", "1.0.0")
//   m.RegisterMCP(srv)
func (m *Module) RegisterMCP(srv *mcplib.MCPServer) {
    fn := m.mcpContextFunc()
    srv.SetHTTPContextFunc(fn)
    srv.SetSSEContextFunc(fn)
}

// Middleware protects HTTP routes. Validates the session cookie and injects
// the authenticated *User into the request context.
// Returns HTTP 401 if the session is missing or expired.
//
// Example:
//   mux.Handle("/admin", m.Middleware(adminHandler))
func (m *Module) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        u, err := m.validateSession(r)
        if err != nil {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        ctx := context.WithValue(r.Context(), userKey, u)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// FromContext extracts the authenticated *User injected by Middleware or RegisterMCP.
// Returns (nil, false) if the context carries no authenticated user.
func (m *Module) FromContext(ctx context.Context) (*User, bool) {
    u, ok := ctx.Value(userKey).(*User)
    return u, ok
}

// AccessCheck is the bridge function for tinywasm/crudp and tinywasm/site.
// Reads the *http.Request from data, validates the session, and checks RBAC permissions.
// Satisfies the site.SetAccessCheck(fn) signature directly.
//
// Usage: site.SetAccessCheck(m.AccessCheck)
func (m *Module) AccessCheck(resource string, action byte, data ...any) bool {
    for _, d := range data {
        if r, ok := d.(*http.Request); ok {
            u, err := m.validateSession(r)
            if err != nil {
                return false
            }
            ok, _ := m.HasPermission(u.ID, resource, action)
            return ok
        }
    }
    return false
}

// mcpContextFunc returns a func compatible with mcp.SetHTTPContextFunc / SetSSEContextFunc.
func (m *Module) mcpContextFunc() func(context.Context, *http.Request) context.Context {
    return func(ctx context.Context, r *http.Request) context.Context {
        u, err := m.validateSession(r)
        if err != nil {
            return ctx // unauthenticated — tools check with FromContext
        }
        return context.WithValue(ctx, userKey, u)
    }
}
```

---

## Step 6 — Update Tests

**Target:** `tests/setup_test.go`, `tests/user_back_test.go`, `tests/suite_back_test.go`,
`tests/rbac_integration_test.go`

- Replace `user.Init(testDB, cfg)` with `m, err := user.New(testDB, cfg)` in `TestMain`
- Propagate `m *Module` to tests that need Module methods
- Replace `user.LoginModule` globals in `testModules`:
  ```go
  modules := user.UIModules()
  expected := []string{"login", "register", "profile", "lan", "oauth/callback"}
  for _, name := range expected {
      found := false
      for _, mod := range modules {
          if h, ok := mod.(interface{ HandlerName() string }); ok && h.HandlerName() == name {
              found = true; break
          }
      }
      if !found {
          t.Errorf("UIModules: missing handler %q", name)
      }
  }
  ```
- Add tests for `m.Middleware` (inject session cookie, assert user in context)
- Add tests for `m.AccessCheck` (mock request with session cookie, assert RBAC check)
- Add tests for each typed CRUDP entity method using the test DB
- Run `gotest` — 100% pass required before proceeding

---

## Step 7 — Verify & Submit

1. Run `gotest` from project root. All tests must pass.
2. Update `SKILL.md`:
   - Replace old `Init()` + global functions API with `New() *Module` API
   - Document `m.RegisterMCP`, `m.Middleware`, `m.FromContext`, `m.AccessCheck`
   - Document `m.Add()` and package-level `UIModules()`
   - Document the entity CRUDP method signatures (typed, db explicit)
3. Run `gopush 'feat: Module pattern, typed CRUDP entities, unified middleware contract'`
