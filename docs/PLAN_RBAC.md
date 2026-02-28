# RBAC Plan: Migrating Graph Assignments (`user`)

## Goal
Centralize all mutation logic (assigning roles to users, adding permissions to roles) into the `user` module, ensuring cache consistency.

## Execution Steps

### Phase 4: Migration of RBAC Mutations and Assignments
**Target Files:**
- Create new: `user_rbac_mutations.go`
- Modify: `user_back.go` (Register main instantiation + inject `*orm.DB`)

**Instructions:**
1. Port the RBAC creation/assignment functions (`CreateRole`, `AssignRole`, `CreatePermission`, `AssignPermission`, etc.) into the `user` package in the `user_rbac_mutations.go` file. Use the ORM for all inserts/updates.
2. **Crucial:** Every time one of these mutations (like `AssignPermission` to a Role) occurs and touches the DB, it **MUST call the cache invalidation function** created in Phase 2 (`PLAN_DOMAIN.md`) so no user in RAM remains out of sync.
3. Update `Init()` in `user_back.go` to initialize the users cache (`userCache`) and prepare the migration environment for the RBAC tables (now inside the `user` domain).

---

## 📟 Annex C: Legacy Context for RBAC Migration

To successfully migrate RBAC into `user`, here is the condensed technical context from the legacy `rbac` package. You must replicate this logic within the `user` domain using `tinywasm/orm`.

### 1. Legacy Schema (For ORM Models generation)
The underlying tables were:
- `rbac_roles`: id, code, name, description
- `rbac_permissions`: id, name, resource, action
- `rbac_role_permissions`: role_id, permission_id
- `rbac_user_roles`: user_id, role_id

*(Your new `ormc`-generated `User`, `Role`, and `Permission` models must map to these concepts, managing relations effectively).*

### 2. Zero-I/O Verification Rule
In the legacy package, authorization checks (`GetUserRoles`, `HasPermission`) were strictly **cache-only (Zero I/O)**. 
When porting these checks to the `user` package, they must query the *hydrated* `User` struct from `userCache` (Phase 2) without ever touching `*orm.DB`.

### 3. Dynamic Handler Registration
The legacy `register.go` seeded permissions programmatically.
- It used `Register(handlers ...)` receiving module structs.
- It relied on duck-typing interfaces: `HandlerName()` and `AllowedRoles()`.
- *Requirement:* You must port this dynamic seeding logic into the `user` domain so applications can still auto-register their module permissions upon `Init()`.

### 4. Legacy Assignment Logic (For ORM Translation)
When writing `AssignRole` or `AssignPermission` in `user_rbac_mutations.go`, you must use `tinywasm/orm` instead of `s.exec.Exec`.
Crucially, you must trigger cache invalidation immediately after the ORM `.Create()` succeeds.

```go
// Legacy Reference (Replace with ORM + Cache Invalidation):
// INSERT INTO rbac_user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT ...
// INSERT INTO rbac_role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT ...
```
