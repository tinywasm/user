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

## Getting Started (Complete Guide)

Here is a complete, edge-compatible example demonstrating how to set up the database connection, initialize the authority orchestrator with a JWT session strategy, configure two authentication modes, protect paths, and seed initial data.

```go
package main

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/router"
	"github.com/tinywasm/server/httpd"
	"github.com/tinywasm/sqlite"
	"github.com/tinywasm/unixid"
	"github.com/tinywasm/user"
	"github.com/tinywasm/user/authority"
	emailpassword "github.com/tinywasm/user/email_password"
	"github.com/tinywasm/user/session/jwt"
	trustedip "github.com/tinywasm/user/trusted_ip"
)

func main() {
	// 1. Establish database connection and wrap in ORM
	conn, err := sqlite.Open("app.db")
	if err != nil {
		panic(err)
	}
	db := orm.New(conn)

	// 2. Generate required ID Generator (e.g. tinywasm/unixid)
	ids, err := unixid.NewUnixID()
	if err != nil {
		panic(err)
	}

	// 3. Initialize the pure authority orchestrator
	m, err := authority.New(db, user.Config{
		IDs:        ids,
		CookieName: "session",
		TokenTTL:   86400, // 24 hours
		TrustProxy: true,
	})
	if err != nil {
		panic(err)
	}

	// 4. Opt into a stateless JWT strategy (replacing default cookie session)
	secret := []byte("your-secret-key-must-be-32-bytes")
	strategy, err := jwt.New(secret, 86400, m, m)
	if err != nil {
		panic(err)
	}
	m.SetStrategy(strategy)

	// 5. Construct and configure authentication modes.
	// We inject Module 'm' which implements the narrow ports.
	//
	// emailpassword.New receives 'm' three times:
	// - 1st 'm' (user.IdentityStore): finds user identities & verifies passwords
	// - 2nd 'm' (user.SessionIssuer): issues cookies/JWT sessions on login
	// - 3rd 'm' (user.SecurityNotifier): reports logins/failures to events publisher
	epAuth := emailpassword.New(m, m, m, emailpassword.WithTrustProxy(true))

	// trustedip.New receives 'm' four times:
	// - 1st 'm' (user.IdentityStore): resolves users/identities
	// - 2nd 'm' (user.TrustedIPStore): checks if the caller's IP is allowed
	// - 3rd 'm' (user.SessionIssuer): issues the session
	// - 4th 'm' (user.SecurityNotifier): publishes security events (IPMismatch, etc.)
	// - true: trust proxy headers for IP check
	tiAuth := trustedip.New(m, m, m, m, true)

	// 6. Enable the authenticators in the authority orchestrator
	m.Enable(epAuth, tiAuth)

	// 7. Mount central user APIs and start the server.
	// The concrete Router is managed by httpd under the hood.
	// Authn globally identifies the user and injects their ID in ctx.UserID().
	// Authorize evaluates declarative RBAC checks specified on routes via .Requires(...).
	srv := httpd.New(httpd.Config{
		Port:      "8080",
		Authn:     m.Authenticate(), // router.Middleware: identifies the user and injects their ID
		Authorize: m.Can,            // model.Authorizer: central RBAC evaluator
	}).Mount(m)                      // m is a router.APIModule -> mounts central flows (POST /logout, etc.)

	// 8. Protect custom routes.
	// We can declare permission gates via .Requires(...) or check m.Can manually inside the handler.
	srv.Router().Get("/api/dashboard", func(ctx router.Context) {
		if !m.Can(ctx.UserID(), "reports", model.Read) {
			ctx.WriteStatus(403)
			return
		}
		ctx.Write([]byte("Welcome to reports dashboard"))
	}).Requires("reports", model.Read)

	// 9. Bootstrap / Seed first administrator user
	err = m.Bootstrap(authority.Seed{
		Email:    "admin@company.com",
		Password: "super-secure-admin-password",
		Name:     "Administrator",
		Role:     "admin",
		Grants: []model.Grant{
			{Resource: model.Wildcard, Actions: model.AllActions}, // full permissions
		},
	})
	if err != nil {
		panic(err)
	}

	srv.ListenAndServe()
}
```

---

## Composition Examples

### Example 1: App with ONLY OAuth2 login and cookie sessions

```go
import (
	"github.com/tinywasm/unixid"
	"github.com/tinywasm/user"
	"github.com/tinywasm/user/authority"
	"github.com/tinywasm/user/oauth2"
	"github.com/tinywasm/user/oauth2/provider/google"
)

// Initialize UnixID generator
ids, _ := unixid.NewUnixID()

// Initialize pure orchestrator
m, err := authority.New(db, user.Config{IDs: ids})

// Build Google Provider via struct literal
gProv := &google.GoogleProvider{
	ClientID:     "your-google-client-id",
	ClientSecret: "your-google-client-secret",
	RedirectURL:  "https://miapp.cl/oauth/callback/google",
}

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
	"github.com/tinywasm/unixid"
	"github.com/tinywasm/user"
	"github.com/tinywasm/user/authority"
	emailpassword "github.com/tinywasm/user/email_password"
	"github.com/tinywasm/user/session/jwt"
	trustedip "github.com/tinywasm/user/trusted_ip"
)

// Initialize UnixID generator
ids, _ := unixid.NewUnixID()

// Initialize pure orchestrator
m, err := authority.New(db, user.Config{IDs: ids})

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

---

## Upgrading from v0.1.0 to v0.2.0 (Breaking)

This release shifts `Identity.Provider` values from `"local"` and `"lan"` to `"email_password"` and `"trusted_ip"`. To upgrade an existing database in production, run the following SQL statements:

```sql
UPDATE identity SET provider = 'email_password' WHERE provider = 'local';
UPDATE identity SET provider = 'trusted_ip'    WHERE provider = 'lan';
```

---

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
