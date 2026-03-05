# Master Plan: 100% Security-Focused Test Coverage for `tinywasm/user`

## Context
The `tinywasm/user` library currently has 50.4% coverage. As the core identity and access (RBAC) resolver for the ecosystem, it is critical to raise coverage to **100%**, pushing boundaries on race conditions, injection, token expiration, OAuth state validation, and LAN trust.

## Development Rules
- `go install github.com/tinywasm/devflow/cmd/gotest@latest`
- Build tags: backend `//go:build !wasm`, WASM `//go:build wasm`
- **Standard Library Only:** **NEVER** use external assertion libraries (e.g., testify). Use `testing`, `net/http/httptest`.
- **Mocking:** Tests MUST use Mocks for external interfaces.
- **Coverage Goal:** 100%. All edge cases, especially failure branches, must be tested.
- **Diagram-Driven Testing (DDT):** Every logic flow MUST have a corresponding Markdown flowchart in `docs/diagrams/` referenced by the tests.
- **Test File Organization:** Each test group MUST be in an independent file inside `tests/`.

## Architecture & Scope
Since this is a security library, testing the "Happy Path" is not enough. We must intentionally test common attack vectors and weaknesses:
- **Tampering:** JWT forgery (manipulating headers/payload without knowing the secret).
- **Replay Attacks:** Reuse of `state` in OAuth Callback.
- **Cross-Provider State Hijacking:** Submitting OAuth state to wrong provider endpoint to destroy it.
- **LAN Spoofing:** Simulating untrusted `X-Forwarded-For` headers in on-premise login.
- **Race conditions:** Concurrent access to `User` and `Session` in-memory caches.
- **Security observability:** `OnSecurityEvent` hook for real-time attack detection and automated response.
- **Timing oracle:** Constant-time `Login` responses regardless of user existence (dummy bcrypt).
- **Session fixation:** `RotateSession` post-login to prevent pre-auth session ID reuse.
- **SQL injection boundary:** Validate ORM parameterization with adversarial inputs.
- **Cookie hardening:** Assert `HttpOnly`, `Secure`, `SameSite=Strict` flags on every `SetCookie` path.

## Known Limitations (document in tests, do not treat as bugs)
- **JWT mode + non-active user:** `validateSession` does not check `User.Status`. Active JWTs remain
  valid until expiry. Short `TokenTTL` is the mitigation. Call `SuspendUser` + `PurgeSessionsByUser`
  together for immediate effect in cookie mode; JWT mode requires waiting for token expiry.
- **`HasPermission` ignores status:** a non-active user with a valid session context passes RBAC checks.
  Suspension is enforced at Login time only.

## Execution Stages
← [README.md](../README.md)

**Testing stages (no new code):**
1. [Stage 1: Core Authentication, Sessions & JWT](PLAN_STAGE_1_AUTH.md)
2. [Stage 2: RBAC, Permissions & Access Control](PLAN_STAGE_2_RBAC.md)
3. [Stage 3: OAuth States & LAN Identity Proxies](PLAN_STAGE_3_OAUTH_LAN.md)
4. [Stage 4: Cache Concurrency & UI Modules](PLAN_STAGE_4_CACHE_MODULES.md)

**Feature stages (new code + tests):**
5. [Stage 5: Security Events & User State Machine](PLAN_STAGE_5_SECURITY_EVENTS.md)
6. [Stage 6: Production Hardening](PLAN_STAGE_6_HARDENING.md)

Stage 5 requires implementing new API surface before writing tests:
- `SecurityEvent` type + `SecurityEventType` constants → `user.go`
- `OnSecurityEvent func(SecurityEvent)` field → `Config` in `user.go`
- `notify()`, `SuspendUser()`, `ReactivateUser()`, `PurgeSessionsByUser()` → `user_back.go`
- Status guard fix: `== "suspended"` → `!= "active"` in `Login` and `LoginLAN` → `auth.go`, `lan.go`
- Instrumentation of 8 detection points → `jwt_back.go`, `oauth.go`, `lan.go`, `auth.go`, `middleware_back.go`

Stage 6 requires implementing new API surface before writing tests:
- Timing-safe Login: dummy `bcrypt.CompareHashAndPassword` on user-not-found, suspended, and
  no-local-identity paths → `auth.go`
- `OnPasswordValidate func(string) error` field → `Config` in `user.go`
- `RotateSession(oldID, ip, userAgent) (Session, error)` → `sessions.go`

**Not in scope (caller/infrastructure responsibility):**
- Rate limiting → reverse proxy (`nginx limit_req`) or app middleware
- HTTPS enforcement → server TLS config; library already sets `Secure` cookie flag
- CSRF tokens → `tinywasm/site` form handling; library sets `SameSite=Strict` as mitigation

Once all stages are completed, running `gotest` from the root should confirm: `✅ coverage: 100.0%`.
