# PLAN — Production wiring surface: `MountAPI` (login/logout), `me` with permissions, `Bootstrap` [PARTIALLY IMPLEMENTED]

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

## Progress Summary

The core logic for production wiring has been implemented. The module now supports mounting its own API, returning permissions in the `me` tool, and a safe bootstrap method.

### ✅ Completed
1. **Stage 1 — `MountAPI`**: implemented in `server/mount.go`. Added SSR support in `server/mount_ssr.go`. Path constants added to root `user.go`.
2. **Stage 2 — `me` permissions**: `ProfileDTO` extended and `me` tool updated in `server/tools.go`.
3. **Stage 3 — `Bootstrap`**: implemented in `server/bootstrap.go` with wildcard support in `server/rbac.go`.
4. **Environment plumbing**: added `server/init.go` for SSR form input registration and `server/export.go` for test visibility.

### ❌ Pending
1. **Stage 4 — Tests (Blocked)**: `tests/production_wiring_test.go` is implemented but `testMountAPI` fails.
   - **Issue**: `GET /login` renders a form without fields.
   - **Cause**: The SSR rendering via `tinywasm/form` is not picking up the `LoginData` schema. While `models_orm.go` contains generated `Schema()` methods, they might need to be explicitly registered or the Fielder implementation in `models.go` vs `models_orm.go` is causing a conflict/shadowing that prevents `form.New` from seeing the fields.
2. **Stage 5 — Documentation**: `README.md`, `docs/ARCHITECTURE.md`, and `docs/SKILL.md` updates were deferred to prioritize implementation.

## Revised Plan for next Agent

1. **Fix SSR Form Rendering**:
   - Investigate why `loginModule.form` (initialized in `server/module.go`) is not rendering fields in SSR mode.
   - Ensure `LoginData` correctly implements `fmt.Fielder` and its `Schema()` is visible to `form.New`.
   - Verify that `form.RegisterInput` in `server/init.go` is sufficient for the fields used.
2. **Fix `POST /login` test case**:
   - Once the form renders, ensure the test correctly simulates a form submission or JSON post as expected by the handler.
3. **Complete Documentation**:
   - Update `README.md` with "Production wiring" section.
   - Update diagrams in `docs/diagrams/` (via `docs/ARCHITECTURE.md`).
   - Update `docs/SKILL.md` with new conventions.
4. **Final Verification**:
   - Run `gotest ./...` and ensure all tests pass.
