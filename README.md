# tinywasm/user
<img src="docs/img/badges.svg">

User management library for the tinywasm ecosystem. Handles user entities,
password authentication, OAuth providers (Google, Microsoft), LAN (local network)
authentication by RUT + IP, and session management.

## Documentation

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — What & Why: schema, contracts, design principles
- [docs/SKILL.md](docs/SKILL.md) — API contract, configuration, and usage snippets

> **Note**: RBAC is integrated into the User module (see ARCHITECTURE.md).

## Diagrams

- [docs/diagrams/AUTH_FLOW.md](docs/diagrams/AUTH_FLOW.md) — Local login credential validation
- [docs/diagrams/SESSION_FLOW.md](docs/diagrams/SESSION_FLOW.md) — Session lifecycle
- [docs/diagrams/USER_CRUD_FLOW.md](docs/diagrams/USER_CRUD_FLOW.md) — User creation pipeline
- [docs/diagrams/OAUTH_FLOW.md](docs/diagrams/OAUTH_FLOW.md) — OAuth begin/callback flow (all branches)
- [docs/diagrams/LAN_AUTH_FLOW.md](docs/diagrams/LAN_AUTH_FLOW.md) — LAN login: RUT validation + IP allowlist check
- [docs/diagrams/LAN_IP_FLOW.md](docs/diagrams/LAN_IP_FLOW.md) — LAN IP management

## Initialization

```go
import "github.com/tinywasm/user/server"

// ...

// Initialize the user module directly with an ORM db instance
m, err := userserver.New(db, user.Config{
    CookieName: "session_id", // default: "session"
    TokenTTL:   86400,        // default: 86400 (24h)
    TrustProxy: true,         // default: false
    JWTSecret:  []byte("your-secret"), // Required for JWT/Bearer modes
})
```

## Production Wiring

`tinywasm/user` handles authentication flows, while views belong to the consumer.

1. **Mount API**: Call `m.MountAPI(router)` to publish standard authentication routes (`POST /login`, `POST /logout`, `/oauth/:provider`).
2. **Bootstrap**: Call `m.Bootstrap(email, password)` on startup to ensure a first administrator exists.
3. **Consumer Views**: The application builds its own login page using `form.New(&user.LoginData{})` and posts to `user.PathLogin`.
4. **Protect Routes**: Inject `m.Authenticate()` (middleware) and `m.Can` (authorization) into your host router.
5. **Client-side gating**: Use the `me` MCP tool to retrieve user profile and permissions for cosmetic UI gating.

## Status

> Implementation complete. Ready for production wiring.
