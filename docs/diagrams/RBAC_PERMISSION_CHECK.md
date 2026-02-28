# Permission Check Flow

> All functions in `check.go` are **cache-only**. No SQL queries after `Init()`.

## `HasPermission(userID, resource, action)`

```mermaid
flowchart TD
    A([HasPermission userID resource action]) --> B{userID empty?}
    B -- Yes --> ERR1([return false, error: empty userID])
    B -- No --> C[cache.mu.RLock]
    C --> D["roles := cache.userRoles[userID]"]
    D --> E{roles == nil\nor empty?}
    E -- Yes --> UNLOCK1[cache.mu.RUnlock]
    UNLOCK1 --> DENY1([return false, nil\nuser has no roles])
    E -- No --> F[for each role in roles]
    F --> G["perms := cache.rolePerms[role.ID]"]
    G --> H[for each perm in perms]
    H --> I{perm.Resource == resource\nAND perm.Action == action?}
    I -- Yes --> UNLOCK2[cache.mu.RUnlock]
    UNLOCK2 --> OK([return true, nil])
    I -- No --> J{more perms?}
    J -- Yes --> H
    J -- No --> K{more roles?}
    K -- Yes --> F
    K -- No --> UNLOCK3[cache.mu.RUnlock]
    UNLOCK3 --> DENY2([return false, nil\nno matching permission])
```

**Complexity:** O(R×P) where R = user's roles count, P = permissions per role.
In practice, both are < 10 so this is effectively O(1) with no allocations.

---

## `GetUserRoleCodes(userID)`

```mermaid
flowchart TD
    A([GetUserRoleCodes userID]) --> B[cache.mu.RLock]
    B --> C["roles := cache.userRoles[userID]"]
    C --> D{roles empty?}
    D -- Yes --> E[cache.mu.RUnlock]
    E --> EMPTY([return empty slice, nil])
    D -- No --> F["collect role.Code bytes\ninto []byte slice"]
    F --> G[cache.mu.RUnlock]
    G --> CODES([return codes, nil])
```

Returns `[]byte{'a', 'e'}` directly — compatible with crudp `AllowedRoles`.
No struct allocations for role data, only the `[]byte` result.

---

## `GetUserRoles(userID)`

```mermaid
flowchart TD
    A([GetUserRoles userID]) --> B[cache.mu.RLock]
    B --> C["roles := cache.userRoles[userID]"]
    C --> D{roles empty?}
    D -- Yes --> E[cache.mu.RUnlock]
    E --> EMPTY([return empty slice, nil])
    D -- No --> F[copy roles to new slice\navoid exposing internal pointers]
    F --> G[cache.mu.RUnlock]
    G --> ROLES([return roles copy, nil])
```

Returns a copy of the slice to prevent callers from mutating cache internals.

---

## Write-Through: Cache Stays Consistent

When the authorization state changes via API calls, the cache is updated atomically:

```mermaid
flowchart LR
    A([rbac.AssignRole\nuserID roleID assignedAt]) --> B[exec.Exec SQL\nINSERT INTO rbac_user_roles\nON CONFLICT DO NOTHING]
    B --> C{SQL error?}
    C -- Yes --> ERR([return error])
    C -- No --> D["cache.mu.RLock\nrole := cache.roles[roleID]\ncache.mu.RUnlock"]
    D --> E["cache.addUserRole\nuserID role\n(write lock)"]
    E --> F([return nil])
```

Next call to `HasPermission` or `GetUserRoleCodes` for this user
will see the updated roles **without any DB query**.

---

## Notes

- `(false, nil)` means "user has no matching permission" — NOT an error.
- `(false, error)` only happens for invalid input (empty `userID`).
- The `error` return on `HasPermission` / `GetUserRoleCodes` is kept for API
  consistency and future extensibility (e.g., adding input validation).
- `deleteRole(id)` in cache cascades: removes role from `roles`, `rolesByCode`,
  `rolePerms[id]`, and filters it out of every `userRoles[*]` slice.
- `deletePerm(id)` in cache cascades: removes perm from `perms` and filters it
  out of every `rolePerms[*]` slice.
