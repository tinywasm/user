# Stage 2: RBAC, Permissions & Access Control

← [Stage 1](PLAN_STAGE_1_AUTH.md) | Next → [Stage 3: OAuth & LAN](PLAN_STAGE_3_OAUTH_LAN.md)

## Objective
Methodically secure and test the flat graph structure of `user_rbac_mutations.go`
(Users ↔ Roles ↔ Permissions) to ensure destructive mutations prevent permission gaps, confirm
`HasPermission` string-matching semantics, and verify cache invalidation on every mutation.

## Already Covered — do NOT duplicate

The following cases are already implemented in `tests/rbac_integration_test.go`
(`TestIntegration_FullFlow`). The new files below must NOT reproduce them.

| Existing test | Cases already covered |
|---|---|
| `TestIntegration_FullFlow` | `CreateRole` · `AssignRole` · `HasPermission` true (Editor→read) · `HasPermission` false (Editor→write) · `DeleteRole` → permission lost · `Register` with mock `RBACObject` (roles not in DB silently skipped) |

---

## 1. Role & Permission Assignments (`tests/rbac_assignment_test.go`)
- **Target:** `user_rbac_mutations.go`
- **Diagram:** `docs/diagrams/test_rbac_assignment.md`
- **Test Cases (new only):**
  - Concurrent `AssignRole` for the same (userID, roleID) pair → both calls must return `nil`
    (duplicate is silently ignored via `isUniqueViolation`).
  - Concurrent `AssignPermission` for the same (roleID, permissionID) → both calls return `nil`.
  - `AssignRole` triggers `ucache.Delete(userID)`: hydrate user into cache first, assign role,
    assert user is absent from cache after assignment (next `GetUser` must hit DB).
  - `AssignPermission` triggers `ucache.InvalidateByRole(roleID)`: hydrate users with that role
    into cache first, call `AssignPermission`, assert those users are evicted from cache.
  - `DeleteRole(roleID)` full cascade assertion:
    1. Create role, assign to 2 users, assign 1 permission.
    2. Call `DeleteRole`.
    3. Assert `UserRole` and `RolePermission` link rows are deleted (query DB directly).
    4. Hydrate each of the 2 users and confirm the role is absent from `u.Roles`.
  - `DeletePermission(id)` triggers `ucache.InvalidateByPermission(id)` — hydrate affected users
    into cache first, call `DeletePermission`, assert they are evicted from cache.
  - `RevokeRole(userID, roleID)` for a non-existent link → must return `sql.ErrNoRows` (propagated
    from `ReadOneUserRole`).
  - `CreateRole` with a pre-existing ID: must perform an upsert (update name/code/description),
    verifying that existing role-user links remain intact after the update.

## 2. RBAC Declarative System (`tests/rbac_capability_test.go`)
- **Target:** `crud.go` (hydrate), `user_rbac_mutations.go` (`HasPermission`, `Register`)
- **Diagram:** `docs/diagrams/test_rbac_capability.md`
- **Test Cases (new only):**
  - `HasPermission` for a **non-existent userID** → must return `(false, nil)`.
  - `HasPermission` for a **suspended** user with a valid hydrated context: `HasPermission` does NOT
    check status — it returns `(true, nil)` if the permission exists. Document this explicitly as a
    known limitation. Enforcement of suspension must happen at the session/middleware layer.
  - `GetRoleByCode` for a non-existent code → must return `sql.ErrNoRows`.
  - `Register(handlers...)` with roles **not pre-existing** in DB: assert `CreatePermission` IS
    called (permission registered) but `AssignPermission` is NOT called for that role
    (role lookup fails silently, `Register` returns no error).
