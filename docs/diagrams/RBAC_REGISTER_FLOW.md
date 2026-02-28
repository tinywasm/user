# Register Flow

> `rbac.Register(handlers ...any) error` seeds permissions and role assignments into the
> database by reading `HandlerName()` and `AllowedRoles()` from each handler via duck-typing.
> All DB operations are idempotent (`ON CONFLICT DO NOTHING`).
> Must be called after `Init()` and after global roles are created.

## Happy Path

```mermaid
flowchart TD
    A([rbac.Register handlers...]) --> B{for each handler}

    B --> C{implements\nHandlerName AND\nAllowedRoles?}
    C -- No --> B

    C -- Yes --> D[resource = handler.HandlerName]
    D --> E{for each action\nc r u d}

    E --> F[roles = handler.AllowedRoles action]
    F --> G{len roles == 0?}
    G -- Yes, skip action --> E

    G -- No --> H[CreatePermission\nunixid.New, resource:action, resource, action\nON CONFLICT DO NOTHING]
    H --> I[perm = cache.findPermByAction\nresource, action\nO1 lookup]
    I --> J{perm == nil?}
    J -- Yes, unexpected --> E

    J -- No --> K{for each roleCode\nin roles}
    K --> L[role = GetRoleByCode roleCode]
    L --> M{role == nil?}
    M -- Yes, not seeded --> K

    M -- No --> N[AssignPermission\nrole.ID, perm.ID\nON CONFLICT DO NOTHING]
    N --> K
    K -- done --> E
    E -- done --> B
    B -- done --> O([return nil])
```

## Error Path — CreatePermission Failure

```mermaid
sequenceDiagram
    participant App as Application
    participant R as rbac (global)
    participant DB as PostgreSQL

    App->>R: rbac.Register(hs...)
    R->>R: handler.HandlerName() → "invoice"
    R->>R: handler.AllowedRoles('r') → ['a','e']
    R->>DB: INSERT INTO rbac_permissions ... ON CONFLICT DO NOTHING
    DB-->>R: error (connection lost)
    R-->>App: error "rbac: CreatePermission invoice:r: connection lost"
    Note over App: Startup should abort — permissions may be partially seeded.
```

## Error Path — AssignPermission Failure

```mermaid
sequenceDiagram
    participant App as Application
    participant R as rbac (global)
    participant DB as PostgreSQL

    App->>R: rbac.Register(hs...)
    R->>DB: INSERT INTO rbac_permissions ... ON CONFLICT DO NOTHING
    DB-->>R: OK (perm seeded or already existed)
    R->>R: cache.findPermByAction("invoice", 'r') → perm
    R->>R: GetRoleByCode('a') → admin role
    R->>DB: INSERT INTO rbac_role_permissions ... ON CONFLICT DO NOTHING
    DB-->>R: error (FK violation: role deleted mid-startup?)
    R-->>App: error "rbac: AssignPermission: ..."
```

## Notes

- Handlers not implementing `HandlerName() string` or `AllowedRoles(action byte) []byte`
  are **silently skipped** — not an error.
- `AllowedRoles(action)` returning `nil` / empty for an action means "no access required for
  that action" — the action is skipped, no permission is created.
- Roles referenced by code (`'a'`, `'e'`, `'v'`) must be seeded **before** `Register()` is
  called. Unknown role codes are silently skipped (not an error).
- `Register` is safe to call on every startup. All DB operations use `ON CONFLICT DO NOTHING`.
- The cache is already warm after `Init()`. `findPermByAction` performs an O(1) lookup into
  `permsByRA` — no additional DB query after `CreatePermission`.
