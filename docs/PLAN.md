# PLAN — Production wiring surface: `MountAPI` (login/logout), `me` with permissions, `Bootstrap`

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

## Context (zero-context summary)

`tinywasm/user` is the ecosystem's single authority for **who** (identity) and
**may** (RBAC). The backend `userserver.Module` (created via `New(db,
user.Config)`) already provides: `Authenticate() router.Middleware`,
`Can(userID, resource, action string) bool`, sessions/JWT/bearer, OAuth, RBAC
primitives (`CreateRole`/`CreatePermission`/`AssignRole`/`AssignPermission`/
`GetRoleByCode`), CRUD handlers (`Add()`), SSR-capable auth UI flow handlers
(`UIModules()`: login/register/profile/lan/oauth with `RenderHTML`, `Create`,
`SetCookie`), and MCP tools (`Tools()` with `me`).

Three gaps block a consumer (`veltylabs/mjosefa-cms`) from going to production:

1. **The auth flows are not mounted anywhere.** `loginModule` can render its
   form and validate credentials, but nothing publishes `GET/POST /login` or
   `POST /logout` on a `router.Router`. Every consumer would have to hand-wire
   routes against private types — impossible (the types are unexported) and
   wrong (this module owns its flows).
2. **`me` returns no permissions.** The client shell (`platformd.Platform.CanView`)
   gates the nav cosmetically using the profile's permissions; today
   `user.ProfileDTO` carries only `ID/Name/Email/Roles`, so the client cannot
   know which resources the user may see.
3. **No first-run story.** A fresh database has zero users: nobody can ever log
   in. Consumers need an idempotent, safe bootstrap.

**Ecosystem conventions that apply:** no stdlib in shared code (`tinywasm/fmt`),
no `any`/`map` in public APIs, typed constants for every repeated string (route
paths!), errors always propagate, `gotest` only.

## Stage 1 — `Module` implements `router.APIModule`

New file `server/mount.go`:

```go
// MountAPI publishes the authentication flows on the host router. The module
// owns its routes; consumers just Mount it like any other APIModule.
func (m *Module) MountAPI(r router.Router) {
	// GET  PathLogin  → full SSR login page (loginModule.RenderHTML wrapped in
	//                   a minimal HTML document; no wasm required to log in)
	// POST PathLogin  → parse LoginData, loginModule.Create (Login), on success
	//                   SetCookie + redirect PathAfterLogin; on failure re-render
	//                   the login page with the error (and emit SecurityEvent)
	// POST PathLogout → destroy session + clear cookie + redirect PathLogin
}
```

Rules:

- Route paths are exported typed constants: `PathLogin = "/login"`,
  `PathLogout = "/logout"`, `PathAfterLogin = "/"` (overridable via
  `user.Config` field `AfterLoginPath string`, default `"/"`).
- The SSR page reuses the existing `loginModule.RenderHTML()` (form + OAuth
  links) — no duplicated markup. The HTML document wrapper lives in a
  `//go:build !wasm` file per the SSR-split convention.
- Failed logins go through the existing `OnSecurityEvent` notification path.
- These routes are **public by design** (they are the door); document that the
  host must not require a resource on them.
- OAuth callback routes: mount the existing `oauthModule` handlers under their
  current path convention only if providers are registered; zero routes when
  `cfg.OAuthProviders` is empty (pay-per-use).

## Stage 2 — `me` returns permissions

Extend `user.ProfileDTO` (in the root package `models.go`) with:

```go
Permissions []string // "resource:actions" pairs, e.g. "service_catalog:rc"
```

The `me` tool (`server/tools.go`) fills it from the user's roles' permissions
(data already reachable via `hydrateUser`/RBAC tables; add a private
`permissionsOf(u)` helper if needed). Purpose: the client shell filters its
nav cosmetically (`CanView`); the server keeps re-validating every call with
`Can` — state this in the field's doc comment.

## Stage 3 — `Bootstrap` (first-run admin seed)

New method in `server/`:

```go
// Bootstrap seeds the very first administrator. It is a no-op unless the users
// table is EMPTY. When empty it: creates the user (email), sets the password,
// creates (or reuses) the role with code RoleCodeAdmin, grants it the wildcard
// permission ResourceAll/ActionAll, and assigns it. Idempotent and safe to call
// on every startup.
func (m *Module) Bootstrap(email, password string) error
```

- `RoleCodeAdmin = "admin"`, plus wildcard resource/action constants — whatever
  wildcard convention `Can`/`HasPermission` already honors; if none exists,
  implement wildcard support in `HasPermission` (`resource == "*"` matches all)
  as part of this stage, with tests.
- Empty email/password with an empty table → explicit error (the consumer
  surfaces "set ADMIN_EMAIL/ADMIN_PASSWORD"); with a non-empty table → nil.
- Uses only existing primitives (`createUser`, `SetPassword`, `CreateRole`,
  `CreatePermission`, `AssignPermission`, `AssignRole`).

## Tests (`gotest ./...`, never `go test`; suite conventions already exist in `tests/`)

- Mount: `httptest` against a router hosting `MountAPI` — GET login renders the
  form; POST with good credentials sets the session cookie and redirects; POST
  with bad credentials re-renders with error + emits SecurityEvent; logout
  clears the session.
- `me`: profile includes the exact permission pairs of the user's roles.
- `Bootstrap`: empty table seeds admin who passes
  `Can(admin, "<any>", "<any>")`; second call is a no-op; non-empty table is a
  no-op; empty credentials + empty table errors.

## Documentation (mandatory)

- `README.md`: "Production wiring" section — `New` → `MountAPI` → inject
  `Authenticate`/`Can` into the host, `Bootstrap` from env at startup, `me` for
  client-side cosmetic gating.
- `docs/ARCHITECTURE.md`: add mount surface + bootstrap flow to the diagrams.
- `docs/SKILL.md`: update conventions (consumers never touch private flow
  handlers; they Mount the module).

## Harness checklist (mandatory)

- Route paths, role codes and wildcard markers are exported typed constants —
  zero string literals in logic.
- No `any`/`map` in new public API; no stdlib in shared/wasm-visible code.
- Errors propagate; every auth failure emits the existing `SecurityEvent`.
- Additive: no existing public signature changes (`ProfileDTO` gains a field —
  additive for `tinywasm/json` decoding).

## Acceptance criteria

1. `gotest ./...` green.
2. A host app reaches production login with exactly:
   `auth, _ := userserver.New(db, cfg)` + `Mount(auth)` +
   `Authn: auth.Authenticate(), Authorize: auth.Can` +
   `auth.Bootstrap(email, pass)` — no other user-package calls.
3. `me` powers `platformd.CanView` (permission pairs verified in test).
4. Fresh DB + env credentials → login works end-to-end in the mount test.

## Stages

| Stage | File(s) | Action |
|---|---|---|
| 1 | `server/mount.go` (+ SSR wrapper file `!wasm`) | `MountAPI`: GET/POST login, POST logout, OAuth passthrough; path constants |
| 2 | `models.go`, `server/tools.go` | `ProfileDTO.Permissions` + fill in `me` |
| 3 | `server/bootstrap.go` | `Bootstrap` + wildcard permission support if missing |
| 4 | `tests/` | mount round-trip, me permissions, bootstrap idempotency |
| 5 | `README.md`, `docs/ARCHITECTURE.md`, `docs/SKILL.md` | production wiring docs |
