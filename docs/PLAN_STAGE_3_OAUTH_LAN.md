# Stage 3: OAuth States & LAN Identity Proxies

← [Stage 2](PLAN_STAGE_2_RBAC.md) | Next → [Stage 4: Cache & Modules](PLAN_STAGE_4_CACHE_MODULES.md)

## Objective
Completely test federated authentication in `oauth.go`, RUT handling in `lan_ips.go`, and
cross-identities (Identities) without passwords in `identities.go`. Ensure prevention of
"Replay Attacks" in callbacks, "Cross-Provider State Hijacking", and "IP Spoofing" in LAN networks.

## Already Covered — do NOT duplicate

The following cases are already implemented in `tests/suite_back_test.go` (`testOAuth`).
The new files below must NOT reproduce them.

| Existing test | Cases already covered |
|---|---|
| `testOAuth` | `BeginOAuth` happy path · `CompleteOAuth` new user (`isNew=true`) · `CompleteOAuth` existing user (`isNew=false`, same ID returned) |

---

## 1. OAuth State Expiry & Forgery (`tests/oauth_state_test.go`)
- **Target:** `oauth.go`
- **Diagram:** `docs/diagrams/test_oauth_state.md`
- **Important:** `consumeState` is an **unexported** package-level function. All tests in
  `package tests` (external) MUST go through `m.CompleteOAuth(providerName, r, ip, ua)`.
  This requires a **mock `OAuthProvider`** that implements the `user.OAuthProvider` interface.
  Define a `mockProvider` type in the test file with configurable `ExchangeCode` / `GetUserInfo`
  returns. For state-consumption tests, the mock `ExchangeCode` and `GetUserInfo` can return
  valid stubbed values — the assertion focuses on the error returned by `CompleteOAuth` before
  those calls are reached.
- **Test Cases:**
  - **Replay Attack:** call `m.CompleteOAuth("providerA", r, ip, ua)` with a valid state (success),
    then call it a second time with the same state string → second call MUST return
    `ErrInvalidOAuthState` (state was deleted on first use).
  - **Expired State:** call `m.BeginOAuth("providerA")` to obtain a state, then manually update the
    `OAuthState.ExpiresAt` in DB to a past timestamp. Call `m.CompleteOAuth("providerA", r, ip, ua)`
    → must return `ErrInvalidOAuthState`. Assert the expired state IS deleted from DB (the current
    implementation deletes before checking expiry, so deletion is guaranteed even on expiry failure).
  - **Cross-Provider State Hijacking:** call `m.BeginOAuth("providerA")` to create a state, then
    submit it to provider B via `m.CompleteOAuth("providerB", r, ip, ua)` → must return
    `ErrInvalidOAuthState` AND the state must **NOT** be deleted from DB. The state must remain
    valid so the legitimate provider A flow can still complete. Deleting on mismatch would *enable*
    the attack (DoS against the legitimate flow). Assert: after the failed cross-provider call,
    `m.CompleteOAuth("providerA", r, ip, ua)` still succeeds (mock returns valid user info).
  - `m.BeginOAuth` with a non-existent provider name → must return `ErrProviderNotFound` (no state
    is written to DB).
  - `m.PurgeExpiredOAuthStates`: insert a mix of expired and valid states directly into DB, run
    purge, assert only expired ones are removed and valid ones remain.

## 2. LAN IP Identity, Proxies & Spoofing (`tests/lan_proxy_test.go`)
- **Target:** `lan_ips.go`, `lan.go`
- **Diagram:** `docs/diagrams/test_lan_proxy.md`

Already covered in `testLAN` (`suite_back_test.go`) — do NOT duplicate:

| Existing test | Cases already covered |
|---|---|
| `testLAN` | Login from registered IP → success · Login from wrong IP → `ErrInvalidCredentials` · `TrustProxy=true` + `X-Forwarded-For` → success · `RevokeLANIP` + retry → `ErrInvalidCredentials` · `RegisterLAN` with invalid RUT → `ErrInvalidRUT` · `UnregisterLAN` + retry → `ErrInvalidCredentials` |

- **Test Cases (new only):**
  - **IP Spoofing with TrustProxy=false:** send `X-Forwarded-For: 192.168.1.1` but real `RemoteAddr`
    is `10.0.0.1`. Module MUST use `RemoteAddr` and fail authentication (`ErrInvalidCredentials`).
  - **`RegisterLAN` RUT already taken by another user** → must return `ErrRUTTaken`.
  - **`RegisterLAN` idempotent:** register same RUT for the same user twice → second call returns `nil`.
  - **`UnregisterLAN` for user with no LAN identity** → must return `ErrNotFound`.
  - **`AssignLANIP` duplicate IP:** assign the same IP to two different users → second call must return
    `ErrIPTaken`.
  - **`GetLANIPs`:** assign 3 IPs to a user, assert the slice has 3 entries ordered by `CreatedAt ASC`.
  - **`RevokeLANIP` for non-existent (userID, IP) pair** → must return `ErrNotFound`.

## 3. Identity Management (`tests/identities_test.go`)
- **Target:** `identities.go`
- **Diagram:** `docs/diagrams/test_identities.md`
- **Test Cases:**
  - `GetUserIdentities`: create a user with a local identity + one OAuth identity → assert slice
    contains exactly 2 entries with correct `Provider` values.
  - `GetUserIdentities` for a user with no identities → must return empty slice (not error).
  - `UnlinkIdentity`: user with 2 identities; unlink one → assert only 1 identity remains in DB.
    Confirm the remaining identity is the correct one.
  - **`UnlinkIdentity` last identity guard:** user with only 1 identity; call `UnlinkIdentity`
    → must return `ErrCannotUnlink`. DB must be unchanged.
  - **`UnlinkIdentity` provider not found:** call `UnlinkIdentity` for a provider the user
    never registered → must return `ErrNotFound`.
