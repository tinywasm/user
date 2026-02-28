# Master Plan: Integrating RBAC Logic into the User Module

This document is the **Master Orchestrator (Implementation Plan)** for integrating all Role-Based Access Control (RBAC) logic —currently isolated in the `rbac/` package— directly into the `user/` package.

The main goal is to create a unified structure where querying a user from the database *explicitly* returns all their roles and permissions in a single query. The cache will be managed in one place with a hard limit (Max 1000 users) and strict active/passive invalidation mechanisms.

All changes must follow the current architectural principles: zero external dependencies, Single Responsibility Principle (SRP) keeping files under 500 lines, and a flat hierarchy.

**Development Rules** (from `CLAUDE.md` and `user_global`):
- **No Hardcoded Database Logic:** Never write raw SQL strings. Always use `github.com/tinywasm/orm` typed queries (`ReadOneUser`, etc.).
- **Mandatory Dependency Injection:** The database dependency (`*orm.DB`) must be injected into the Store, never global.
- **Documentation-First:** Update architecture docs BEFORE coding; maintain consistency across all `*.md` files.
- **Testing Runner (`gotest`):** ALWAYS use the globally installed `gotest` CLI command.
- **File Organization:** Flat hierarchy, max 500 lines per file, consistent naming (no mixed prefixes like `RBAC_` and non-prefixed files).

---

## 🏗 Sequential Execution Phases

You must execute this plan step by step through the modular documents. Do not proceed to the next phase without fully completing, testing, and verifying the previous one.

### Phase 0: Documentation Cleanup & Standardization
**Target Files (to delete or unify):**
- Delete: `RBAC_ARQUITECTURE.md` (typo, superseded by `RBAC_ARCHITECTURE.md`)
- Consolidate: `RBAC_ARCHITECTURE.md` → Rename to `ARCHITECTURE.md` and merge with existing `ARCHITECTURE.md`
- Review all diagram files in `docs/diagrams/`:
  - Keep diagrams without `RBAC_` prefix (they are the canonical ones)
  - Remove `RBAC_*` prefixed diagram duplicates (e.g., delete `RBAC_USER_CRUD_FLOW.md`, keep `USER_CRUD_FLOW.md`)

**Instructions:**
1. Open `ARCHITECTURE.md` and integrate any unique content from `RBAC_ARCHITECTURE.md`.
2. Verify that all diagrams are consistent and non-duplicated.
3. Update `README.md` to ensure all remaining docs are properly linked.
4. Commit this cleanup with message: `"docs: consolidate RBAC documentation into unified structure"`

---

### Step-by-Step Execution Modules

Execute the following modular plans strictly in this order:

1. **[PLAN_DOMAIN.md](PLAN_DOMAIN.md)**  
   *Phase 1 & Phase 2:* Extend the `User` DB models to hold slices of Roles/Permissions and create the `userCache` struct with max 1000 limits and invalidation methods.

2. **[PLAN_ORM.md](PLAN_ORM.md)**  
   *Phase 2.5 & Phase 3:* Run `ormc` generation, inject `*orm.DB` into the store, and rewrite all queries globally using the ORM Fluent API.

3. **[PLAN_RBAC.md](PLAN_RBAC.md)**  
   *Phase 4:* Port legacy assignment and graph mutations into the `user` domain, wrapping them entirely in `tinywasm/orm` commands and invoking cache invalidation immediately.

4. **[PLAN_TESTING.md](PLAN_TESTING.md)**  
   *Phase 5:* Refactor legacy tests to use `user` instead of `rbac` and validate against the `tinywasm/sqlite` driver in `:memory:` mode using `tinywasm/orm`.

---

### Phase 6: Final Documentation Update & Integration
**Target Files:**
- `ARCHITECTURE.md` (update with ORM integration and cache architecture)
- Update all affected diagrams (if any changes to flows)
- `README.md` (verify all links are current)

**Instructions:**
1. **Update ARCHITECTURE.md:**
   - Add section explaining the transition to `tinywasm/orm` (elimination of magic strings).
   - Document the new integrated cache architecture (userCache with 1000-user limit).
   - Explain how RBAC is now part of the User domain (not separate).

2. **Verify Diagrams:**
   - Ensure `USER_CRUD_FLOW.md` shows the new RBAC hydration step (not just basic User CRUD).

3. **README.md:**
   - Ensure all remaining doc links point to the correct files.
   - Remove any references to the old `rbac/` package.
   - Add a note: "RBAC is now integrated into the User module (see ARCHITECTURE.md)"

4. **Commit:** After all code phases are complete, commit documentation with message: `"docs: update architecture and implementation for unified RBAC integration"`
