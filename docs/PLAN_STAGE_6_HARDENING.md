# Stage 6: Production Hardening

← [Stage 5](PLAN_STAGE_5_SECURITY_EVENTS.md) | [Master Plan](PLAN.md)

## Objective
Close the remaining gaps between "functionally correct" and "production-ready":
timing oracle mitigation, cookie security assertions, session rotation, password
strength hooks, and SQL injection boundary validation.

## Scope boundary — what this library owns vs what the caller owns

| Concern | Owner | Rationale |
|---|---|---|
| Timing-safe Login | **Library** | `Login`/`LoginLAN` control the bcrypt path; dummy bcrypt must be here |
| Cookie flags | **Library** | `SetCookie` in `module_login_back.go` sets them; tests must assert them |
| Session rotation | **Library** | Requires atomic delete+create on `user_sessions`; only the library has access |
| Password strength hook | **Library** | `SetPassword` is the single entry point for hashing; hook must run before bcrypt |
| SQL injection boundary | **Library** | The library passes user input to `tinywasm/orm`; tests validate parameterization |
| Rate limiting | **Caller / Infra** | Belongs in reverse proxy (nginx `limit_req`) or app middleware. Library can't own global state across instances |
| HTTPS enforcement | **Caller / Infra** | Server-level TLS config; `Secure` cookie flag already set by library |
| CSRF tokens | **Caller / Framework** | `tinywasm/site` form handling; library sets `SameSite=Strict` as mitigation |

---

## New API surface (code to write before tests)

### 1. Timing-safe Login — modify `auth.go`

Current code returns immediately on `getUserByEmail` miss (no bcrypt), creating a
measurable timing difference (~200ms) that enables email enumeration.

```go
func (m *Module) Login(email, password string) (User, error) {
    u, err := getUserByEmail(m.db, m.ucache, email)
    if err != nil {
        // Dummy bcrypt: constant-time regardless of user existence.
        // Uses the same cost as real hashing to match timing.
        bcrypt.CompareHashAndPassword([]byte("$2a$10$dummy.hash.for.timing.safety.only"), []byte(password))
        return User{}, ErrInvalidCredentials
    }
    if u.Status != "active" { // already updated by Stage 5
        bcrypt.CompareHashAndPassword([]byte("$2a$10$dummy.hash.for.timing.safety.only"), []byte(password))
        return User{}, ErrSuspended
    }
    // ... rest unchanged
}
```

Apply the same pattern to the `getLocalIdentity` miss path (OAuth/LAN-only user
with no `local` identity).

> **Why not `LoginLAN`?** LAN login uses RUT + IP, no password/bcrypt involved.
> There is no timing oracle to exploit — all paths are constant-time DB lookups.

### 2. Password validation hook — add to `Config` in `user.go`

```go
type Config struct {
    // ... existing fields ...

    // OnPasswordValidate is called by SetPassword before hashing.
    // Return a non-nil error to reject the password.
    // If nil, only the built-in len >= 8 check applies.
    OnPasswordValidate func(password string) error
}
```

Modify `SetPassword` in `auth.go`:

```go
func (m *Module) SetPassword(userID, password string) error {
    if len(password) < 8 {
        return ErrWeakPassword
    }
    if m.config.OnPasswordValidate != nil {
        if err := m.config.OnPasswordValidate(password); err != nil {
            return err
        }
    }
    hash, err := bcrypt.GenerateFromPassword([]byte(password), PasswordHashCost)
    // ...
}
```

### 3. Session rotation — add to `sessions.go`

```go
// RotateSession atomically deletes the old session and creates a new one
// with the same userID, updated IP/UserAgent, and a fresh TTL.
// Prevents session fixation attacks when called post-login.
func (m *Module) RotateSession(oldID, ip, userAgent string) (Session, error)
```

Implementation:
1. `GetSession(oldID)` → extract `UserID`
2. `DeleteSession(oldID)` → remove from cache + DB
3. `CreateSession(userID, ip, userAgent)` → new ID + fresh expiry
4. Return new session (caller updates the cookie)

### 4. Cookie flag assertion helper — no new production code

Tests call `SetCookie` and inspect the `http.ResponseRecorder` headers to assert:
- `HttpOnly` = true
- `Secure` = true
- `SameSite` = Strict
- `Path` = "/"
- `Max-Age` = `Config.TokenTTL`

No new production code needed — the flags are already set correctly in
`module_login_back.go:50-58`. This stage only adds the missing tests.

---

## Already Covered — do NOT duplicate

None of the test cases below are covered by existing tests.

---

## 1. Timing-Safe Authentication (`tests/timing_safe_test.go`)
- **Target:** `auth.go`
- **Diagram:** `docs/diagrams/test_hardening.md`
- **Test Cases:**
  - `Login` with non-existent email: measure execution time. Compare against `Login` with
    correct email + wrong password. Assert the difference is < 50ms (both paths execute bcrypt).
  - `Login` with suspended user: same timing assertion (dummy bcrypt on `ErrSuspended` path).
  - `Login` for OAuth-only user (no local identity): same timing assertion.
  - All three paths must return the correct error (`ErrInvalidCredentials` / `ErrSuspended`),
    unchanged from Stage 1 expected behavior.

## 2. Cookie Security (`tests/cookie_security_test.go`)
- **Target:** `module_login_back.go` (`SetCookie`)
- **Diagram:** `docs/diagrams/test_hardening.md`
- **Test Cases:**
  - `SetCookie` in cookie mode: assert response `Set-Cookie` header contains `HttpOnly`,
    `Secure`, `SameSite=Strict`, `Path=/`, and `Max-Age` matching `Config.TokenTTL`.
  - `SetCookie` in JWT mode: same flag assertions; additionally assert the cookie value
    is a valid 3-segment JWT (not a session ID).
  - `SetCookie` with custom `CookieName`: assert the cookie name in the header matches.

## 3. Session Rotation (`tests/session_rotation_test.go`)
- **Target:** `sessions.go` (`RotateSession`)
- **Diagram:** `docs/diagrams/test_hardening.md`
- **Test Cases:**
  - `RotateSession(oldID, ip, ua)` → returns new `Session` with different ID, same `UserID`,
    fresh `ExpiresAt`. Old session ID must return `ErrNotFound` from `GetSession`.
  - `RotateSession` with expired old session → must return `ErrSessionExpired` (cannot
    rotate an already-expired session).
  - `RotateSession` with non-existent old session → must return `ErrNotFound`.
  - Middleware continuity: after rotation, a request with the NEW session cookie succeeds
    (HTTP 200). A request with the OLD session cookie fails (HTTP 401).

## 4. Password Validation Hook (`tests/password_hook_test.go`)
- **Target:** `auth.go` (`SetPassword`), `user.go` (`Config`)
- **Diagram:** `docs/diagrams/test_hardening.md`
- **Test Cases:**
  - `SetPassword` with `OnPasswordValidate` returning error → `SetPassword` must return
    that error, password NOT updated in DB. Verify via `VerifyPassword` (old password still works).
  - `SetPassword` with `OnPasswordValidate` returning nil → password updated normally.
  - `SetPassword` with `OnPasswordValidate = nil` → only built-in `len >= 8` check applies.
  - `SetPassword` with password that passes length check but fails hook → hook error returned,
    NOT `ErrWeakPassword` (length check runs first, hook second).

## 5. SQL Injection Boundary (`tests/sql_boundary_test.go`)
- **Target:** `crud.go`, `identities.go`, `lan_ips.go`, `auth.go`
- **Diagram:** `docs/diagrams/test_hardening.md`
- **Test Cases:**
  - `Login` with email `"' OR 1=1 --"` → must return `ErrInvalidCredentials`, NOT a DB error
    or successful login.
  - `SetPassword` with userID `"'; DROP TABLE users; --"` → must return `ErrNotFound` or
    identity error. Assert `users` table still exists (query any user).
  - `RegisterLAN` with RUT containing SQL metacharacters (`"12345678-5'; --"`) → must return
    `ErrInvalidRUT` (rejected by `validateRUT` before reaching DB).
  - `GetUser` with ID `"1 UNION SELECT * FROM user_identities"` → must return `ErrNotFound`.
  - `GetRoleByCode` with code `"admin' OR '1'='1"` → must return `sql.ErrNoRows`.
  - All tests must confirm no unexpected rows are returned and no tables are dropped/modified.
