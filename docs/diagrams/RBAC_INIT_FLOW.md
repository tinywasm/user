# Init and Migration Flow

## Happy Path

```mermaid
sequenceDiagram
    participant App as Application
    participant G as rbac (global)
    participant R as *store instance
    participant M as migrate.go
    participant C as cache.go
    participant DB as PostgreSQL

    App->>G: rbac.Init(exec)
    activate G
    G->>G: sync.Once check
    Note over G: First call only — subsequent calls are no-ops

    G->>R: New(exec)
    activate R

    R->>M: r.Migrate()
    activate M
    M->>DB: Exec CREATE TABLE IF NOT EXISTS rbac_roles
    DB-->>M: OK
    M->>DB: Exec CREATE TABLE IF NOT EXISTS rbac_permissions
    DB-->>M: OK
    M->>DB: Exec CREATE TABLE IF NOT EXISTS rbac_role_permissions
    DB-->>M: OK
    M->>DB: Exec CREATE TABLE IF NOT EXISTS rbac_user_roles
    DB-->>M: OK
    M-->>R: nil (success)
    deactivate M

    R->>C: r.loadCache()
    activate C
    C->>DB: SELECT id, code, name, description FROM rbac_roles
    DB-->>C: rows → populate cache.roles + cache.rolesByCode
    C->>DB: SELECT id, name, resource, action FROM rbac_permissions
    DB-->>C: rows → populate cache.perms
    C->>DB: SELECT rp.role_id, p.* FROM rbac_role_permissions JOIN rbac_permissions p
    DB-->>C: rows → populate cache.rolePerms[roleID]
    C->>DB: SELECT ur.user_id, r.* FROM rbac_user_roles JOIN rbac_roles r
    DB-->>C: rows → populate cache.userRoles[userID]
    C-->>R: nil (cache warm)
    deactivate C

    R-->>G: *store, nil
    deactivate R

    G-->>App: nil (success)
    deactivate G

    Note over App: Cache is warm. Authorization reads never touch DB again.

    App->>G: rbac.HasPermission(userID, resource, action)
    G->>G: mustGlobal() → *store
    G->>C: cache.hasPermission(userID, resource, action)
    Note over C: Pure in-memory lookup. Zero I/O.
    C-->>G: true/false
    G-->>App: bool, nil
```

## Error Path A — Migration Failure

```mermaid
sequenceDiagram
    participant App as Application
    participant G as rbac (global)
    participant M as migrate.go
    participant DB as PostgreSQL

    App->>G: rbac.Init(exec)
    G->>G: sync.Once check (first call)
    G->>M: r.Migrate()
    M->>DB: Exec CREATE TABLE IF NOT EXISTS rbac_roles
    DB-->>M: error (permission denied)
    M-->>G: error "rbac migrate: permission denied"
    G-->>App: error
    Note over G: globalRbac remains nil
```

## Error Path B — loadCache Failure

```mermaid
sequenceDiagram
    participant App as Application
    participant G as rbac (global)
    participant M as migrate.go
    participant C as cache.go
    participant DB as PostgreSQL

    App->>G: rbac.Init(exec)
    G->>M: r.Migrate()
    M->>DB: 4x DDL — all OK
    M-->>G: nil

    G->>C: r.loadCache()
    C->>DB: SELECT ... FROM rbac_roles
    DB-->>C: error (connection lost)
    C-->>G: error "rbac loadCache: connection lost"
    G-->>App: error
    Note over G: globalRbac remains nil. Subsequent global calls panic.
```

## Notes

- `sync.Once` guarantees `Init()` body runs exactly once, even under concurrent calls.
- `loadCache()` is the second phase of `New()`. If it fails, the instance is discarded.
- After a successful `Init()`, the DB is only accessed for write mutations (CreateRole, AssignRole, etc.).
- `New()` can be called multiple times (for DI instances). Each call runs both `Migrate()` and `loadCache()`, both of which are safe to repeat.
