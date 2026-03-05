# Stage 5: Security Events & User Status

← [Stage 4](PLAN_STAGE_4_CACHE_MODULES.md) | [Master Plan](PLAN.md)

## Objective
Implement and test the `OnSecurityEvent` hook mechanism. This stage introduces the `SecurityEvent`
type, a `notify()` internal dispatcher, two public status-transition methods (`SuspendUser`,
`ReactivateUser`), `PurgeSessionsByUser`, and fixes the status guard in `Login`/`LoginLAN` to use
`!= "active"` instead of `== "suspended"`. All 8 existing security detection points are instrumented.

## New API surface (code to write before tests)

### 1. Types — add to `user.go`

```go
type SecurityEventType uint8

const (
    EventJWTTampered        SecurityEventType = iota // ValidateJWT: HMAC mismatch
    EventOAuthReplay                                  // consumeState: state already consumed (2nd use)
    EventOAuthExpiredState                            // consumeState: state found but past ExpiresAt
    EventOAuthCrossProvider                           // consumeState: provider mismatch (state preserved)
    EventIPMismatch                                   // LoginLAN: IP not registered
    EventNonActiveAccess                              // Login/LoginLAN: status != "active"
    EventUnauthorizedAccess                           // validateSession: cookie present but session invalid
    EventAccessDenied                                 // AccessCheck: RBAC denied with valid session
)

type SecurityEvent struct {
    Type      SecurityEventType
    IP        string // client IP, empty if not available
    UserID    string // empty if user not yet identified
    Provider  string // OAuth provider name, for OAuth events
    Resource  string // RBAC resource, for EventAccessDenied
    Timestamp int64  // time.Now().Unix()
}
```

Add `OnSecurityEvent func(SecurityEvent)` field to `Config`.

### 2. `notify()` — add to `user_back.go`

```go
func (m *Module) notify(e SecurityEvent) {
    if e.Timestamp == 0 {
        e.Timestamp = time.Now().Unix()
    }
    if m.config.OnSecurityEvent != nil {
        m.config.OnSecurityEvent(e)
        return
    }
    if m.log != nil {
        m.log("security_event", e.Type, e.IP, e.UserID)
    }
}
```

### 3. Status methods — add to `user_back.go`

```go
// SuspendUser sets Status = "suspended". Evicts user from cache.
func (m *Module) SuspendUser(id string) error

// ReactivateUser sets Status = "active". Evicts user from cache.
func (m *Module) ReactivateUser(id string) error

// PurgeSessionsByUser deletes all sessions belonging to userID from cache and DB.
func (m *Module) PurgeSessionsByUser(userID string) error
```

Both `SuspendUser` and `ReactivateUser` wrap the already-existing internal functions
`suspendUser` and `reactivateUser` in `crud.go` — just expose them as public methods on `Module`.

### 4. Status guard fix — `auth.go` and `lan.go`

Change the existing check from:
```go
if u.Status == "suspended" {
```
to:
```go
if u.Status != "active" {
```

This makes the library open to any custom status value the application may set (`"banned"`,
`"locked"`, `"pending"`, etc.) — all are blocked unless the status is explicitly `"active"`.
The returned error stays `ErrSuspended` to avoid leaking status details to the client.

### 5. Instrumentation — call `m.notify(...)` at each existing detection point

> **Architectural constraint for OAuth events:** `consumeState` is a package-level function with
> no access to `m.notify`. It currently returns `ErrInvalidOAuthState` for ALL failure paths,
> making them indistinguishable at the call site. Before instrumenting, refactor `consumeState`
> to return distinct internal sentinel errors:
> - `errOAuthStateNotFound` (len == 0: already consumed → true replay)
> - `errOAuthCrossProvider` (provider mismatch)
> - `errOAuthExpired` (past ExpiresAt)
>
> `CompleteOAuth` maps each sentinel to the correct `m.notify(...)` call and then
> returns `ErrInvalidOAuthState` to the caller (no internal details leak).
>
> **Critical: preserve the current deletion order in `consumeState` during refactoring:**
> - Cross-provider mismatch must return `errOAuthCrossProvider` **before** the `db.Delete` call,
>   so the state is preserved in DB (the legitimate provider A can still complete its flow).
> - Expiry check must happen **after** the `db.Delete` call, so an expired state is always deleted
>   even when it fails validation (prevents accumulation of expired tokens).
> - The current code already follows this order; the refactoring must not change it.
>
> **Import note:** `notify()` calls `time.Now().Unix()`. Adding it to `user_back.go` requires
> adding `"time"` to that file's import block.

| File | Location | Event |
|---|---|---|
| `middleware_back.go` | `validateSession` JWT path — `ValidateJWT` returns `ErrInvalidToken` | `EventJWTTampered` |
| `oauth.go` (via `CompleteOAuth`) | `consumeState` → `errOAuthStateNotFound` (state gone, already used) | `EventOAuthReplay` |
| `oauth.go` (via `CompleteOAuth`) | `consumeState` → `errOAuthExpired` | `EventOAuthExpiredState` |
| `oauth.go` (via `CompleteOAuth`) | `consumeState` → `errOAuthCrossProvider` | `EventOAuthCrossProvider` |
| `lan.go` | `checkLANIP` returns error | `EventIPMismatch` |
| `auth.go` / `lan.go` | `u.Status != "active"` | `EventNonActiveAccess` |
| `middleware_back.go` | cookie found but `validateSession` returns error (cookie mode non-expiry failures) | `EventUnauthorizedAccess` |
| `middleware_back.go` | `AccessCheck` — `HasPermission` returns false | `EventAccessDenied` |

> **Why `middleware_back.go` for JWT tampering, not `jwt_back.go`:**
> `ValidateJWT` is a package-level function with no access to `m.notify`. The call site
> `validateSession` in `middleware_back.go` is the only place with both the error and `m`.
> Distinguish tamper (`ErrInvalidToken`) from expiry (`ErrSessionExpired`) to emit the right event.
>
> **Why `EventUnauthorizedAccess` is NOT at line 84:**
> Line 84 in `middleware_back.go` is the early return when no cookie is found at all — that path
> correctly returns HTTP 401 silently (no cookie = no attack to report). The event should fire
> when a cookie IS present but the session lookup fails (invalid/deleted session ID).

---

## 1. Status Transition Tests (`tests/user_state_test.go`)
- **Target:** `crud.go`, `user_back.go`, `auth.go`, `lan.go`
- **Diagram:** `docs/diagrams/user_state_machine.md`
- **Test Cases:**
  - `SuspendUser(id)` → status is `"suspended"` in DB; user evicted from ucache.
  - `ReactivateUser(id)` → status is `"active"` in DB; user evicted from ucache.
  - `SuspendUser` for non-existent user → must return `ErrNotFound`.
  - `ReactivateUser` for non-existent user → must return `ErrNotFound`.
  - `Login` with status `"suspended"` → must return `ErrSuspended`.
  - `Login` with any custom non-active status (e.g., `"locked"`, `"banned"`) → must return `ErrSuspended`.
    This validates the `!= "active"` guard is truly open.
  - `LoginLAN` with status `"suspended"` → must return `ErrSuspended`.
  - `PurgeSessionsByUser`: create 3 sessions, call `PurgeSessionsByUser`, assert all 3 absent
    from cache and DB. Non-related sessions must remain intact.

## 2. Security Event Emission Tests (`tests/security_events_test.go`)
- **Target:** `user_back.go` (`notify`), all instrumented files
- **Diagram:** `docs/diagrams/security_events.md`
- **Test Cases:**
  - `OnSecurityEvent` configured: assert it fires (not `m.log`) for every event type.
  - `OnSecurityEvent = nil` + `m.log` set: assert `m.log` fires as fallback for every event type.
  - `OnSecurityEvent = nil` + `m.log = nil`: assert no panic (silent no-op).
  - **`EventJWTTampered`:** tampered JWT via middleware; assert `Type == EventJWTTampered`, `IP` populated.
  - **`EventOAuthReplay`:** consume a valid state successfully, then submit the same state again;
    assert hook fires with `Type == EventOAuthReplay` and correct `Provider` (second-use replay).
  - **`EventOAuthExpiredState`:** submit an expired state (ExpiresAt in the past); assert hook fires
    with `Type == EventOAuthExpiredState` and correct `Provider`.
  - **`EventOAuthCrossProvider`:** state submitted to wrong provider; assert `Provider` field correct
    and `Type == EventOAuthCrossProvider`.
  - **`EventIPMismatch`:** `LoginLAN` from unregistered IP; assert `IP` and `UserID` populated.
  - **`EventNonActiveAccess`:** `Login` with suspended user; assert `UserID` populated.
  - **`EventUnauthorizedAccess`:** middleware with valid-format cookie but non-existent session.
  - **`EventAccessDenied`:** `AccessCheck` — valid session, missing RBAC permission; assert `Resource` populated.
  - **Auto-suspend integration:** hook calls `m.SuspendUser(e.UserID)` on `EventJWTTampered`.
    Trigger tampered JWT. Assert user status becomes `"suspended"`. Validates closure pattern without deadlock.
