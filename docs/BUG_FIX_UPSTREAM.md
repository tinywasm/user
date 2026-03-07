# Upstream Bug Fixes Required Before Completing Stage 2

**Status:** Blocked — bugs found in upstream dependencies. Do NOT proceed with Stage 2 tests until these fixes are applied.

---

## Bug 1 — `github.com/tinywasm/sqlite`: Missing connection pool limit

**Current version in use:** `v0.1.8`

**Root cause:** `sqlite.Open()` uses `sql.Open()` without calling `db.SetMaxOpenConns(1)`. SQLite's `:memory:` databases are **per-connection** — each connection creates a completely independent database. With an unlimited pool, concurrent goroutines obtain different connections and see different (empty) databases.

**Symptoms observed:**
- `"no such table: rbac_user_roles"` during concurrent `AssignRole` calls
- `"no such table: rbac_roles"` during concurrent `AssignPermission` calls
- `"database is locked"` on write contention

**Fix published:** `github.com/tinywasm/sqlite@v0.1.9` ✅

---

## Bug 2 — `github.com/tinywasm/sqlite`: No composite primary key support in `buildCreateTable`

**Current version in use:** `v0.1.8`

**Root cause:** The adapter emitted `PRIMARY KEY` as an inline per-column constraint. SQL does not allow two inline `PRIMARY KEY` declarations on the same table. Junction tables require a table-level constraint:

```sql
-- WRONG (v0.1.8 output):
CREATE TABLE rbac_user_roles (user_id TEXT, role_id TEXT)

-- CORRECT (v0.1.9):
CREATE TABLE rbac_user_roles (
    user_id TEXT NOT NULL,
    role_id TEXT NOT NULL,
    PRIMARY KEY (user_id, role_id)
)
```

Without this, `rbac_user_roles` and `rbac_role_permissions` had **no uniqueness constraint**, making `isUniqueViolation()` unreachable and allowing duplicate rows on concurrent inserts.

**Fix published:** `github.com/tinywasm/sqlite@v0.1.9` ✅

---

## Bug 3 — `tinywasm/user/models.go`: Missing `db:"pk"` on junction table fields

**This is a change required in this repo.**

The ORM generator already supports multiple `db:"pk"` fields per struct. The tags were simply missing in `models.go`.

---

## Bug 4 — `tinywasm/user/user_rbac_mutations.go`: `RBACObject.AllowedRoles` returns `[]byte`

**This is a change required in this repo.**

`registerRBAC` iterates each `byte` as a single-character role code via `string(code)`. Role codes like `"admin"` are impossible to express — `[]byte("admin")` would be treated as 5 separate 1-char lookups (`"a"`, `"d"`, `"m"`, `"i"`, `"n"`), all failing silently. The return type must be `[]string`.

---

## Changes Required in This Repo

### Change A — `models.go`: Add `db:"pk"` to junction table fields

```go
type UserRole struct {
    UserID string `json:"user_id" db:"pk"`
    RoleID string `json:"role_id" db:"pk"`
}

type RolePermission struct {
    RoleID       string `json:"role_id"       db:"pk"`
    PermissionID string `json:"permission_id" db:"pk"`
}
```

After editing, regenerate `models_orm.go`:
```bash
go generate ./...
```

### Change B — `user_rbac_mutations.go`: Fix `RBACObject` interface

```go
// BEFORE:
type RBACObject interface {
    HandlerName() string
    AllowedRoles(action byte) []byte
}

// AFTER:
type RBACObject interface {
    HandlerName() string
    AllowedRoles(action byte) []string
}
```

Update the loop inside `registerRBAC`:
```go
// BEFORE:
for _, code := range roles {
    r, err := m.GetRoleByCode(string(code))

// AFTER:
for _, code := range roles {
    r, err := m.GetRoleByCode(code)
```

Update `mockRBACObject` in `rbac_capability_test.go`:
```go
type mockRBACObject struct {
    Name  string
    Roles map[byte][]string
}
func (m *mockRBACObject) AllowedRoles(action byte) []string { return m.Roles[action] }
```

---

## Execution Checklist for Jules

```
[x] 1. Upstream fix published: github.com/tinywasm/sqlite@v0.1.9
[ ] 2. go get github.com/tinywasm/sqlite@v0.1.9
[ ] 3. Edit models.go — add db:"pk" to UserRole and RolePermission fields
[ ] 4. go generate ./...
[ ] 5. Edit user_rbac_mutations.go — change AllowedRoles return type to []string
[ ] 6. Update mockRBACObject in rbac_capability_test.go to use []string
[ ] 7. gotest
[ ] 8. Resume Stage 2
```

---

## Version Reference

| Package | Version in use | Required version |
|---|---|---|
| `github.com/tinywasm/sqlite` | `v0.1.8` | `v0.1.9` ✅ published |
| `github.com/tinywasm/orm` | `v0.2.4` | `v0.2.4` ✅ no change needed |

← [PLAN.md](PLAN.md) | [Stage 2 →](PLAN_STAGE_2_RBAC.md)
