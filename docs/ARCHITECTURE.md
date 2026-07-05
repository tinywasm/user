# Architecture

> **Status:** Published — February 2026

`tinywasm/user` manages user entities, password authentication, LAN (local network)
authentication by RUT + IP, HTTP sessions, and provides **isomorphic UI modules** for
auth flows (login, register, profile, LAN management, OAuth callbacks).

The package is split into three parts:
1. `github.com/tinywasm/user`: Agnostic types and models (shared by all).
2. `github.com/tinywasm/user/user/server` (`userserver`): Backend logic and Module handle.
3. `github.com/tinywasm/user/user/ui` (`userui`): UI modules and OnMount logic.

---

## Core Principles

- **Separation of Concerns:** Backend logic (`userserver`) is decoupled from UI wiring (`userui`). Both share agnostic models in the root `user` package.
- **Single Responsibility:** `Login` validates credentials only. Session creation is explicit.
- **Identity-based authentication:** `Login` routes through `user_identities` — only users
  with a `local` identity can authenticate via email+password. OAuth-only and LAN-only
  users have no local identity, so password login is structurally unreachable.
- **Shared ORM Connection:** Uses the injected `*orm.DB` passed from `main.go`. No separate DB config, entirely handled by `tinywasm/orm`.
- **Integrated RBAC:** RBAC logic is fully integrated into the `user` domain. `User` structs are explicitly hydrated with their `Roles` and `Permissions` via sequential queries from `tinywasm/orm`.
- **Integrated Cache:** An in-memory read-through cache manages both HTTP sessions and up to 1000 hydrated users. Mutations immediately trigger cache invalidations based on role ID or permission ID.
- **Unified identity model:** Local auth, OAuth providers, and LAN auth share the same
  `user_identities` table. One `user_id` can have multiple identities. Passwords are
  stored as bcrypt hashes in `provider_id` for `provider='local'` — the `users` table
  holds no auth secrets.
- **Auto-link by email:** OAuth login with an email that matches an existing local account
  links the OAuth identity automatically.
- **No framework dependencies:** Standard library + `golang.org/x/crypto` + `golang.org/x/oauth2`.
- **Isomorphic modules:** Auth UI modules implement `site.Module` via duck typing. Backend modules (in `userserver`) handle SSR and `Create/Update/Delete`. Frontend modules (in `userui`) self-manage via signals; delegation of `OnMount` is removed.
- **Config struct:** All configuration (`CookieName`, `TokenTTL`, `TrustProxy`,
  `OAuthProviders`, `AuthMode`, `JWTSecret`) is grouped logically and passed during initialization.
- **Form-backed validation:** Each module holds a `*form.Form`. `ValidateData` (crudp.DataValidator) delegates to the form — same validation rules on frontend and backend, zero duplication.

---

## Package Structure

- `user/`: Agnostic models (`User`, `Session`, `Identity`, etc.) and error constants.
- `user/server/`: `Module` struct, `New()`, authentication logic (as `router.Middleware`), RBAC, and SSR modules.
- `user/ui/`: `UIModules()` for frontend registration; forms self-manage via signals.

---

## Schema

### Automated ORM DDL

Raw SQL migrations are obsolete. The `initSchema` method in `userserver` iterates through all the `tinywasm/orm` compliant struct models (`User`, `Role`, `Permission`, `Identity`, `Session`, `LANIP`, `OAuthState`, `UserRole`, `RolePermission`) and utilizes `db.CreateTable(m)` to initialize or alter the database cleanly.

The entities translate logically to the following structures:
- `users`: Managed by `User` struct (supports explicit hydration of Roles and Permissions).
- `user_sessions`: Managed by `Session` struct.
- `user_identities`: Unified identities across providers (local, google, microsoft, lan).
- `user_oauth_states`: Ephemeral state validation.
- `user_lan_ips`: Whitelisted IPs per LAN RUT.
- `rbac_*`: Direct graph representation mapping roles, permissions, and users.

Note: `users.email` is nullable; local-auth and OAuth users always have an email;
LAN-only users may omit it entirely (stored as NULL, not `""`).

---

## Public API Contract & Integration

The specific details for the API contract, exposed functions, and `tinywasm/site` integration steps have been extracted into the `SKILL.md` file. The Architecture document must remain focused on the abstract design.

Please refer to:
- [SKILL.md](SKILL.md) — For API signatures, Usage Snippets, Configuration, and Module Registration details.

---

## Component Relationships

```mermaid
graph TD
    APP["Application\n(web/server.go)"]
    SITE["tinywasm/site\n(routing + RBAC)"]
    SERVER["tinywasm/user/user/server\n(auth backend + SSR)"]
    UI["tinywasm/user/user/ui\n(frontend wiring)"]
    FORM["tinywasm/form\n(validation)"]
    DB["Database\n(via orm.DB)"]

    APP -->|"userserver.New(...)\nm.UIModules()"| SERVER
    APP -->|"userui.UIModules()"| UI
    SERVER -->|"db.CreateTable"| DB
    UI -->|"Signals"| FORM
    SERVER -->|"form.String (SSR)"| FORM
```

---

## Dependencies

```
tinywasm/user
├── github.com/tinywasm/fmt    (errors, logging, string conversion)
├── github.com/tinywasm/form   (UI form construction + ValidateData for modules)
├── github.com/tinywasm/unixid (ID generation)
├── golang.org/x/crypto        (bcrypt, password hashing)
└── golang.org/x/oauth2        (OAuth 2.0 token exchange)
```

---

## Related Documentation

- [diagrams/AUTH_FLOW.md](diagrams/AUTH_FLOW.md) — Local login credential validation
- [diagrams/SESSION_FLOW.md](diagrams/SESSION_FLOW.md) — Session lifecycle
- [diagrams/USER_CRUD_FLOW.md](diagrams/USER_CRUD_FLOW.md) — User creation pipeline
- [diagrams/OAUTH_FLOW.md](diagrams/OAUTH_FLOW.md) — OAuth begin/callback flow
- [diagrams/LAN_AUTH_FLOW.md](diagrams/LAN_AUTH_FLOW.md) — LAN login: RUT validation + IP allowlist check
- [diagrams/LAN_IP_FLOW.md](diagrams/LAN_IP_FLOW.md) — LAN IP management
