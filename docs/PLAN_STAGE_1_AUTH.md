# Stage 1: Core Authentication, Sessions & JWT

← [Master Plan](PLAN.md) | Next → [Stage 2: RBAC](PLAN_STAGE_2_RBAC.md)

## Objective
Thoroughly test `auth.go`, `jwt_back.go`, `sessions.go`, and `middleware_back.go`, resolving common
vulnerabilities such as state injection, tampered cookies, and sessions belonging to suspended users.

## Already Covered — do NOT duplicate

The following cases are already implemented in `tests/suite_back_test.go` under `RunUserTests`.
The new files below must NOT reproduce them.

| Existing test | Cases already covered |
|---|---|
| `testAuth` | Happy-path login (active user) · wrong password → `ErrInvalidCredentials` · `SetPassword` · `VerifyPassword` round-trip |
| `testSessions` | `CreateSession` → `GetSession` (fields match, userID/IP/UserAgent) · `ExpiresAt=0` in DB → `ErrSessionExpired` |
| `testJWTCookieMode` | Valid JWT cookie → middleware injects correct `*User` · malformed/random JWT cookie → HTTP 401 |

---

## 1. Authentication (`tests/auth_security_test.go`)
- **Target:** `auth.go`, `crud.go`
- **Diagram:** `docs/diagrams/test_auth_security.md`
- **Test Cases (new only):**
  - Login with a **non-existent email** → must return `ErrInvalidCredentials`.
  - Login for a **suspended** user → must return **`ErrSuspended`** (NOT `ErrInvalidCredentials`).
    The suspended check runs BEFORE bcrypt to avoid unnecessary CPU cost.
  - `SetPassword` with password shorter than 8 characters → must return `ErrWeakPassword`.
  - `VerifyPassword` with a manipulated hash (bcrypt mismatch) → must return `ErrInvalidCredentials`.
  - Ultra-long strings (>1000 chars) as password input to both `Login` and `SetPassword` — must not panic.

## 2. JWT & Cookie Validation (`tests/jwt_session_test.go`)
- **Target:** `jwt_back.go`, `sessions.go`
- **Diagram:** `docs/diagrams/test_jwt_session.md`
- **Test Cases (new only):**
  - JWT Tampering: alter the `payload` segment (change `sub`) keeping the original header and signature
    → `ValidateJWT` must return `ErrInvalidToken`.
  - JWT Tampering: alter the `header` segment → must return `ErrInvalidToken`.
  - JWT with only 2 segments (missing signature) → must return `ErrInvalidToken`.
  - JWT with invalid base64 in the payload segment → must return `ErrInvalidToken`.
  - JWT Expiration: generate with `ttl=1`, advance `exp` to past via manual timestamp in payload,
    re-sign with correct secret → must return `ErrSessionExpired`.
  - Valid JWT with `ttl=0` defaults to 86400 seconds — assert `exp - iat ≈ 86400`.
  - `DeleteSession` → `GetSession` → must return `ErrNotFound`.
  - `PurgeExpiredSessions`: insert expired sessions, run purge, confirm they are absent from both cache
    and DB. Confirm non-expired sessions are untouched.

## 3. Middleware Context Injection (`tests/middleware_auth_test.go`)
- **Target:** `middleware_back.go`
- **Diagram:** `docs/diagrams/test_middleware_auth.md`
- **Test Cases (new only):**
  - Request with **no session cookie** → `Middleware` must return HTTP 401.
  - Request with a valid session cookie for `AuthModeCookie` → `FromContext` must return the correct `*User`.
  - **JWT Suspended bypass (known limitation):** suspend a user AFTER issuing a valid JWT; the JWT
    remains valid until expiry — `Middleware` still returns HTTP 200. Document this behavior explicitly
    in the test as an accepted trade-off (short JWT TTL mitigates the risk).
  - `AccessCheck`: valid session + user has permission → returns `true`.
  - `AccessCheck`: valid session + user lacks permission → returns `false`.
  - `AccessCheck`: no `*http.Request` in `data` vararg → returns `false`.
  - `FromContext` with a context carrying no user value → must return `(nil, false)`.
  - `RegisterMCP` delegates to `Middleware` — assert identical behavior (no cookie → 401, valid cookie → 200).
