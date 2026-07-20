# tinywasm/user
<img src="docs/img/badges.svg">

User management library for the tinywasm ecosystem. Handles user entities, login credentials, authentication, session management, and RBAC.

In `v0.2.0`, the system has been refactored to make `authority` a pure orchestrator, decoupling authentication modes and session strategies into completely independent, injectable components.

## Package Structure

| Package | Purpose |
|---|---|
| `github.com/tinywasm/user` | WASM-safe root package defining contracts, ports, common models, and DTOs |
| `github.com/tinywasm/user/session/cookie` | Stateful opaque session IDs stored in HttpOnly cookies (default) |
| `github.com/tinywasm/user/session/jwt` | Stateless cryptographically signed JWT sessions (carried in HttpOnly cookies or Bearer headers) |
| `github.com/tinywasm/user/email_password` | Independent email+password credential authenticator |
| `github.com/tinywasm/user/trusted_ip` | Independent Chilean RUT checksum and IP allowlist authenticator |
| `github.com/tinywasm/user/oauth2` | Independent OAuth2 begin/callback flow authenticator |
| `github.com/tinywasm/user/authority` | Pure orchestrator carrying database tables, RBAC rules, central operations, and logout endpoints |

## Documentation

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — What & Why: schema, contracts, design principles
- [docs/SKILL.md](docs/SKILL.md) — API contract, configuration, and usage snippets

---

## Composition Examples

### Example 1: App with ONLY OAuth2 login and cookie sessions

```go
import (
	"github.com/tinywasm/user"
	"github.com/tinywasm/user/authority"
	"github.com/tinywasm/user/oauth2"
	"github.com/tinywasm/user/oauth2/provider/google"
)

// Initialize pure orchestrator
m, err := authority.New(db, user.Config{IDs: testIDs})

// Build Google Provider
gProv := google.New(user.OAuthConfig{...})

// Construct independent OAuth2 authenticator
oaAuth := oauth2.New(m, m, m, []user.OAuthProvider{gProv})

// Register/enable the authenticator
m.Enable(oaAuth)

// Mount logout and all enabled authenticator login routes
m.MountAPI(router)
```

### Example 2: App with Email/Password + Trusted IP logins and JWT sessions

```go
import (
	"github.com/tinywasm/user"
	"github.com/tinywasm/user/authority"
	emailpassword "github.com/tinywasm/user/email_password"
	"github.com/tinywasm/user/session/jwt"
	trustedip "github.com/tinywasm/user/trusted_ip"
)

// Initialize pure orchestrator
m, err := authority.New(db, user.Config{IDs: testIDs})

// Build and set the stateless JWT session strategy
strategy, err := jwt.New([]byte("your-secret-key-must-be-32-bytes"), 3600, m, m)
m.SetStrategy(strategy)

// Construct authenticators passing orchestrator ports
epAuth := emailpassword.New(m, m, m)
tiAuth := trustedip.New(m, m, m, m, true) // true = trustProxy

// Enable authenticators
m.Enable(epAuth, tiAuth)

// Mount logout and all enabled authenticator login routes
m.MountAPI(router)
```

## Diagrams

- [docs/diagrams/AUTH_FLOW.md](docs/diagrams/AUTH_FLOW.md) — Local login credential validation
- [docs/diagrams/SESSION_FLOW.md](docs/diagrams/SESSION_FLOW.md) — Session lifecycle
- [docs/diagrams/USER_CRUD_FLOW.md](docs/diagrams/USER_CRUD_FLOW.md) — User creation pipeline
- [docs/diagrams/OAUTH_FLOW.md](docs/diagrams/OAUTH_FLOW.md) — OAuth begin/callback flow (all branches)
- [docs/diagrams/LAN_AUTH_FLOW.md](docs/diagrams/LAN_AUTH_FLOW.md) — LAN login: RUT validation + IP allowlist check
- [docs/diagrams/LAN_IP_FLOW.md](docs/diagrams/LAN_IP_FLOW.md) — LAN IP management

## Production Wiring

`tinywasm/user` handles authentication flows, while views belong to the consumer.

1. **Mount API**: Call `m.MountAPI(router)` to publish standard authentication routes (`POST /login`, `POST /logout`, `/oauth/:provider`).
2. **Bootstrap**: Call `m.Bootstrap(Seed)` on startup to ensure a first user and their initial role/permissions exist.
3. **Consumer Views**: The application builds its own login page using `form.New(&user.LoginData{})` and posts to `user.PathLogin` using JSON.
4. **Protect Routes**: Inject `m.Authenticate()` (middleware) and `m.Can` (authorization) into your host router.
5. **Client-side gating**: Use the `me` MCP tool to retrieve user profile and permissions for cosmetic UI gating.

## Status

> Decoupled architecture complete. Orchestrator pure + pluggable auth modes.
