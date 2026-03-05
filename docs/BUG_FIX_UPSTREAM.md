# Upstream Bug Fixes Required Before Completing Stage 2

**Status:** Blocked — bugs found in upstream dependencies. Do NOT proceed with Stage 2 tests until these fixes are published and updated in `go.mod`.

---

## Summary of Root Causes

Three real architectural bugs were identified. They are not test-level issues. Jules's concurrent RBAC tests exposed them correctly.

---

## Bug 1 — `github.com/tinywasm/sqlite`: Missing connection pool limit

**Current version in use:** `v0.1.8`

**Root cause:** `sqlite.Open()` uses `sql.Open()` without calling `db.SetMaxOpenConns(1)`. SQLite's `:memory:` databases are **per-connection** — each connection creates a completely independent database. With an unlimited pool, concurrent goroutines obtain different connections and see different (empty) databases.

**Symptoms observed:**
- `"no such table: rbac_user_roles"` during concurrent `AssignRole` calls
- `"no such table: rbac_roles"` during concurrent `AssignPermission` calls
- `"database is locked"` on write contention

**Fix applied in:** `github.com/tinywasm/sqlite` — will be published as `v0.1.9` (or higher).

**Update instruction:**
```
go get github.com/tinywasm/sqlite@latest
```

---

## Bug 2 — `github.com/tinywasm/sqlite` + `github.com/cdvelop/postgre`: No composite primary key support in `buildCreateTable`

**Current versions in use:** `sqlite v0.1.8`, `postgres v0.1.8` (module: `github.com/cdvelop/postgre`)

**Root cause:** Both adapters emit `PRIMARY KEY` as an inline per-column constraint. SQL does not allow two inline `PRIMARY KEY` declarations. Junction tables with composite PKs require a table-level constraint:
```sql
-- WRONG (current output):
CREATE TABLE rbac_user_roles (user_id TEXT, role_id TEXT)

-- CORRECT (after fix):
CREATE TABLE rbac_user_roles (
    user_id TEXT NOT NULL,
    role_id TEXT NOT NULL,
    PRIMARY KEY (user_id, role_id)
)
```

Without this, `rbac_user_roles` and `rbac_role_permissions` have **no uniqueness constraint**, making `isUniqueViolation()` unreachable and allowing duplicate rows on concurrent inserts.

**Fix applied in:** both `github.com/tinywasm/sqlite` and `github.com/cdvelop/postgre`.

**Note:** `tinywasm/user` does NOT directly depend on `github.com/cdvelop/postgre`. Production apps wire postgres in their `main.go`. This fix matters for production deployments.

**Update instruction (for `tinywasm/user`):**
```
go get github.com/tinywasm/sqlite@latest
```

---

## Bug 3 — `github.com/tinywasm/orm`: Composite PK not expressed in model schema

**Current version in use:** `v0.2.4`

**Root cause:** The ORM code generator (`ormc`) was already able to handle multiple `db:"pk"` tags per struct (it uses a per-field `!fieldIsPK` check, not a global `pkFound` gate for explicit tags). However, `tinywasm/user/models.go` was missing the `db:"pk"` tags on junction table fields. This is a **models.go fix** in `tinywasm/user` itself, not in the ORM package.

The ORM `v0.2.4` is already correct for this. No upstream ORM version bump is needed for Bug 3.

---

## Changes Required in `tinywasm/user` (this repo)

These two changes must be applied **after** updating the sqlite dependency.

### Change A — `models.go`: Add `db:"pk"` to junction table fields

```go
// UserRole — composite PK
type UserRole struct {
    UserID string `json:"user_id" db:"pk"`
    RoleID string `json:"role_id" db:"pk"`
}

// RolePermission — composite PK
type RolePermission struct {
    RoleID       string `json:"role_id"       db:"pk"`
    PermissionID string `json:"permission_id" db:"pk"`
}
```

After editing `models.go`, regenerate `models_orm.go`:
```bash
go generate ./...
```

This will update `UserRole.Schema()` and `RolePermission.Schema()` to emit `orm.ConstraintPK` on both fields, which the fixed adapters will then translate into a proper composite `PRIMARY KEY` table constraint.

### Change B — `user_rbac_mutations.go`: Fix `RBACObject` interface

The `AllowedRoles` method returns `[]byte`, but `registerRBAC` iterates each byte as a single-character role code via `string(code)`. Role codes like `"admin"` or `"editor"` are impossible to express. Jules's test confirmed the confusion in the commented code.

```go
// BEFORE (broken — each byte is treated as a separate 1-char role code):
type RBACObject interface {
    HandlerName() string
    AllowedRoles(action byte) []byte
}

// AFTER (correct — each string is a role code):
type RBACObject interface {
    HandlerName() string
    AllowedRoles(action byte) []string
}
```

Also update the loop inside `registerRBAC`:
```go
// BEFORE:
for _, code := range roles {
    r, err := m.GetRoleByCode(string(code))

// AFTER:
for _, code := range roles {
    r, err := m.GetRoleByCode(code)
```

---

## Execution Checklist for Jules

```
[ ] 1. Wait for upstream fix signal (or check that sqlite >= v0.1.9 is published)
[ ] 2. go get github.com/tinywasm/sqlite@latest
[ ] 3. Edit models.go — add db:"pk" to UserRole and RolePermission fields
[ ] 4. go generate ./...   (regenerates models_orm.go)
[ ] 5. Edit user_rbac_mutations.go — change AllowedRoles return type to []string
[ ] 6. Update rbac_capability_test.go mockRBACObject.AllowedRoles to return []string
[ ] 7. Run: gotest
[ ] 8. Confirm no "database is locked" and no "no such table" errors
[ ] 9. Resume Stage 2 test implementation
```

---

## Version Pinning Reference

| Package | Version in use | Min version with fix |
|---|---|---|
| `github.com/tinywasm/sqlite` | `v0.1.8` | `v0.1.9` (pending) |
| `github.com/cdvelop/postgre` | `v0.1.8` | `v0.1.9` (pending) |
| `github.com/tinywasm/orm` | `v0.2.4` | `v0.2.4` ✅ no change needed |

← [PLAN.md](PLAN.md) | [Stage 2 →](PLAN_STAGE_2_RBAC.md)
