# Implementation Plan: Fix RBAC Integration

> **Module:** `github.com/tinywasm/user`
> **Goal:** Resolve compilation errors in the tests and update architecture diagrams that were missed during the RBAC integration plan (`CHECK_PLAN.md`).

---

## Development Rules

- **Testing Runner (`gotest`):** For Go tests, ALWAYS use the globally installed `gotest` CLI command.
- **Mandatory Dependency Injection (DI):** The database dependency (`*orm.DB`) must be injected into the Store, never global.
- **Documentation First:** You MUST update the documentation *before* coding or running `gopush`.
- **Diagram Standards:** Markdown files (`*.md`) containing Mermaid code (`docs/diagrams/`). Use simple, vertical, linear flowcharts. NEVER use the `subgraph` directive.
- **File Organization:** Flat hierarchy, max 500 lines per file.

### Prerequisites

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
go install github.com/tinywasm/orm/cmd/ormc@latest
```

---

## Issues to Correct

### 1. Update Architecture Diagram (`docs/diagrams/USER_CRUD_FLOW.md`)

**Problem:** Phase 6 of `CHECK_PLAN.md` required: *"Ensure `USER_CRUD_FLOW.md` shows the new RBAC hydration step (not just basic User CRUD)"*, but the diagram currently has no mention of Roles or Permissions.
**Action:** 
- Modify `docs/diagrams/USER_CRUD_FLOW.md` (specifically around the `GetUser` / `GetUserByEmail` notes).
- Add the flow showing how the `user` module now fetches RBAC data (Roles and Permissions) alongside the user query to hydrate the domain model.

### 2. Fix Test Compilation Errors

**Problem:** Running `gotest` fails with multiple compilation errors in `tests/rbac_integration_test.go`:
```
tests/rbac_integration_test.go:21:7: undefined: user.Init
tests/rbac_integration_test.go:23:17: undefined: user.CreateRole
tests/rbac_integration_test.go:37:17: undefined: user.Register
tests/rbac_integration_test.go:41:7: undefined: user.CreateUser
tests/rbac_integration_test.go:48:18: undefined: user.HasPermission
```
**Action:**
- The test file relies on an outdated global API (e.g., `user.Init`, `user.CreateRole`). Under the new DI architecture and ORM integration, these methods likely reside on an injected `Store` or similar struct (e.g., `New(db)`).
- Refactor the test to correctly instantiate the local in-memory DB (`tinywasm/sqlite`), inject the `*orm.DB` dependency, and invoke the methods from the actual implemented API.
- Ensure all tests pass by executing `gotest`.

---

## Execution Steps

1. **Modify `docs/diagrams/USER_CRUD_FLOW.md`** to explicitly include the RBAC hydration steps.
2. **Refactor `tests/rbac_integration_test.go`** to use the current API matching the DI requirements.
3. **Run `gotest`** at the project root (`/home/cesar/Dev/Project/tinywasm/user`) and ensure all tests complete without errors.
4. **Deploy** via `gopush 'fix: resolve RBAC integration tests and diagrams'` once execution is complete.
