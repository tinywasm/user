← [Stage 4](PLAN_STAGE_4_TESTS_MODULE.md) | Next → [Stage 6](PLAN_STAGE_6_DOCS.md)

# Stage 5 — OWASP hardening: enumeration fix, RateLimit hook, regression suite

## 5a — fix user enumeration in `POST /login` (OWASP A07)

`Login` returns `ErrSuspended` ("user suspended") for non-active accounts but
`ErrInvalidCredentials` ("access denied") otherwise, and the login handler in
[server/mount.go](../server/mount.go) writes `err.Error()` verbatim into the
`401` body. An attacker can therefore distinguish "account exists but is
suspended" from "account does not exist".

- Every authentication failure must produce an **identical** response: status
  `401`, body `user.ErrInvalidCredentials.Error()` — regardless of whether
  `Login` returned `ErrInvalidCredentials`, `ErrSuspended`, or anything else.
- No new string literals: reuse the exported `user.ErrInvalidCredentials`.
- The distinction is preserved **internally**: `Login` already emits
  `EventNonActiveAccess` for suspended accounts
  ([server/auth.go](../server/auth.go)) and the handler keeps emitting
  `EventAccessDenied`. Consumers who need the distinction get it via
  `Config.OnSecurityEvent`, never via the HTTP body.
- Do **not** change `Login`'s return values — `ErrSuspended` stays a typed
  error for programmatic callers. Only the HTTP response is normalized.

## 5b — `Config.RateLimit` hook (OWASP A07 / ASVS 2.2.1)

Brute-force mitigation cannot live inside this library: in ephemeral,
distributed isolates an in-memory counter is useless (each isolate keeps its
own, multiplying the effective limit), and picking a shared store would break
DB-agnosticism. The mitigation belongs to the consumer's edge-native storage
(KV, Durable Objects, Redis) or WAF — but the library must make that
responsibility an **explicit, typed contract**.

- In [user.go](../user.go), add to `Config` (follow the existing
  `OnPasswordValidate` hook pattern):

  ```go
  // RateLimit, when set, is consulted before password verification on
  // POST login. The library is stateless by design (edge isolates cannot
  // hold counters); the consumer injects a limiter backed by its
  // edge-native storage (KV, Durable Objects, Redis) or WAF. A non-nil
  // error rejects the attempt with 429 before any bcrypt work.
  RateLimit func(ip, email string) error
  ```

- In [user.go](../user.go), append `EventRateLimited` to the
  `SecurityEventType` enum — **at the end** of the iota block (existing values
  are persisted/compared; their order must not shift).

- In `server/mount.go` `POST /login`, after decoding the credentials and
  extracting the client IP, **before** calling `m.Login`:

  ```go
  if m.config.RateLimit != nil {
      if err := m.config.RateLimit(ip, data.Email); err != nil {
          m.notify(user.SecurityEvent{Type: user.EventRateLimited, IP: ip, UserID: data.Email})
          ctx.WriteStatus(429)
          ctx.Write([]byte(err.Error()))
          return
      }
  }
  ```

  Extract the IP **once** (`extractClientIP(ctx, m.config.TrustProxy)`) and
  reuse it for the hook, the security events and `CreateSession` — the handler
  currently calls it twice.

- A `nil` hook means current behavior, zero overhead. Scope is the password
  login route only; OAuth callbacks and LAN login are out of scope.

## 5c — OWASP regression suite (`tests/owasp_test.go`, inside the tests module)

One top-level `TestOWASP(t *testing.T)` running the subtests below; each
carries a short comment naming the OWASP item it guards. Existing coverage
(SQL injection A03, timing-safe A07, RBAC-closed A01, cookie flags A05,
session rotation A07) stays untouched — do not duplicate it. All login bodies
are **JSON** (Stage 2).

- **A07 — user enumeration:** three `POST /login` attempts: (a) unknown email,
  (b) known email + wrong password, (c) suspended account — create the user,
  `m.SetPassword`, then `m.SuspendUser(id)` (the exported API in
  `server/module.go`; do not hand-write an ORM status update, and note the
  status literal `"active"` is private to `server/`). All three must return
  **identical** status (401) and **identical** body
  (`user.ErrInvalidCredentials.Error()`).

- **A02/A07 — JWT rejection paths** (direct `ValidateJWT` calls). The error
  lives in the **server** package (`userserver.ErrInvalidToken`,
  `server/jwt.go`) — package `user` has no `ErrInvalidToken`:
  - token signed with secret A validated with secret B → `ErrInvalidToken`;
  - tampered payload (decode, change `sub`, re-encode, keep the old signature)
    → `ErrInvalidToken`;
  - empty signature segment (`header.payload.`) → `ErrInvalidToken`;
  - expired token — `GenerateJWT` with a **negative** TTL (e.g. `-10`); TTL `0`
    is not usable, `GenerateJWT` rewrites it to the 86400 default →
    `user.ErrSessionExpired`;
  - garbage string → `ErrInvalidToken`;
  - happy path: round-trip returns the original userID.

- **A01 — privilege escalation:** grant a role to user A; assert user B (no
  roles) fails `Can` for the same resource/action; assert `Can` with a made-up
  user ID is denied.

- **A07 — session reuse after logout:** login via `MountAPI`, capture the
  session cookie, `POST /logout`, then send an authenticated request through
  `m.Authenticate()` with the old cookie value → the identity must be empty
  (invalid server-side, not merely cleared client-side).

- **A09 — security-event contract:** register `Config.OnSecurityEvent` and
  assert the expected event is **present** in the collected slice — not that it
  is the only one. A suspended-account login legitimately emits **two** events:
  `Login` emits `EventNonActiveAccess` (`server/auth.go`) and the mount handler
  then emits `EventAccessDenied` for the same failure. **Do not "fix" that
  duplication** — it is delivered behavior and out of scope.
  - failed password login → `EventAccessDenied` with the attempted email and an
    IP;
  - suspended-account login → `EventNonActiveAccess` (plus the
    `EventAccessDenied` above);
  - tampered bearer JWT through the middleware → `EventJWTTampered` (emitted by
    `validateJWT`, `server/middleware.go`);
  - `RateLimit` rejection → `EventRateLimited`.

- **A04 — open redirect:** a login body smuggling an extra field
  (`"redirect": "https://evil.example"`) plus a query string on the path; on
  success the `Location` header must equal the configured `AfterLoginPath`
  **exactly** — never derived from request data.

- **CSRF surface:** a `POST /login` whose body is not valid JSON → `400`.
  Combined with `SameSite=Strict` cookies, that is this library's CSRF posture.

- **RateLimit hook semantics:**
  - hook returns an error → `429`, body from the hook error, **no** session
    cookie, and `Login` never ran — prove it: the hook flips a flag while the
    credentials are valid, so a `429` with valid credentials means the gate
    fired first;
  - hook returns nil → login proceeds normally (302 + cookie);
  - hook nil (unset) → current behavior untouched.

## Acceptance

- The three enumeration probes return byte-identical 401 responses.
- All JWT rejection subtests pass; a reintroduced `err.Error()` body in the
  login 401, or a removed signature check, fails the suite.
- `Config.RateLimit` rejection short-circuits **before** bcrypt with `429` +
  `EventRateLimited`; an unset hook changes nothing.
- The security-event subtests pin the four emissions.
- `gotest ./...` green at the root and inside `tests/`.
