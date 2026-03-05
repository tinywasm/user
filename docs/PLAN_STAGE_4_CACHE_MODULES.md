# Stage 4: Cache Concurrency & UI Modules

← [Stage 3](PLAN_STAGE_3_OAUTH_LAN.md) | [Master Plan](PLAN.md) | Next → [Stage 5: Security Events](PLAN_STAGE_5_SECURITY_EVENTS.md)

## Objective
Test the robustness of in-memory caches (`cache.go`, `cache_users.go`) under concurrent load to detect race conditions and validate the correct isomorphic rendering of UI modules (`module_*.go`), ensuring no error state is left without visual coverage.

## Already Covered — do NOT duplicate

No existing tests cover cache concurrency or the cases below. All test cases in this stage are new.

---

## 1. Concurrency & Race Conditions (`tests/cache_concurrency_test.go`)
- **Target:** `cache.go`, `cache_users.go`
- **Diagram:** `docs/diagrams/test_cache_concurrency.md` (Parallel Lock/Unlock access).
- **Test Cases:**
  - Cache Flooding: Start 100 goroutines attempting to read, write, and invalidate the same session/user simultaneously. Run with `gotest -race`.
  - User Cache Overflow: Insert 1001 users and verify the oldest user is evicted (**FIFO behavior** — the cache uses an insertion-order queue, not LRU), maintaining the 1000-item limit without memory leaks. Verify the 1001st insert evicts the 1st inserted user, regardless of how recently it was accessed.
  - Invalidation by Role/Permission: Modify a role's permission and verify in separate threads that the cache of users holding that role is invalidated immediately.

## 2. Isomorphic UI Modules SSR/WASM (`tests/module_rendering_test.go`)
- **Target:** `module_login.go`, `module_register.go`, `module_profile.go`, `module_lan.go`
- **Diagram:** `docs/diagrams/test_module_rendering.md` (SSR Rendering flow).

Already covered in `testModulesSSR` (`user_back_test.go`) — do NOT duplicate:

| Existing test | Cases already covered |
|---|---|
| `testModulesSSR` | `login` `RenderHTML()` contains `<form` · `register` `RenderHTML()` contains `<form` · `profile` `RenderHTML()` contains `<form` · `lan` `RenderHTML()` contains `<table` |

- **Test Cases (new only):**
  - Login `RenderHTML()` includes OAuth provider buttons when providers are registered in `Config`.
  - Validation Error Flow: inject malformed data into module `Create`/`Update` methods (e.g., invalid
    RUT in LANModule) and verify the returned error is the expected sentinel for 100% branch coverage.
  - WASM compilation stub test (`//go:build wasm` in `tests/user_front_test.go`): assert that
    `user.UIModules()` returns the expected module names. The current placeholder satisfies build
    compilation; replace it with a real assertion on `HandlerName()` for each expected module.
    Do NOT execute DB logic — WASM stubs must remain side-effect-free.
