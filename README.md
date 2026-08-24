# tinywasm/user
<img src="docs/img/badges.svg">

Stable identity value contract for the TinyWasm ecosystem. Authentication,
sessions, concrete providers, and authorization live in sibling libraries
`github.com/tinywasm/auth` and `github.com/tinywasm/rbac`.

## Package Structure

| Package | Purpose |
|---|---|
| `github.com/tinywasm/user` | WASM-safe root package defining `SubjectID` and `Subject` |
| `github.com/tinywasm/auth` | Authentication, sessions, credential modes, OAuth2, providers, `auth/local` |
| `github.com/tinywasm/rbac` | Roles, permissions, assignments, `Can` |

Dependency direction: `auth` and `rbac` import `user`; neither imports the
other. Only the application composition root imports both.

```mermaid
flowchart TD
    U[user] --> A[auth]
    U --> R[rbac]
    A --> C[app]
    R --> C
```

## Documentation

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — Dependency graph and retained contract
- [docs/DESIGN.md](docs/DESIGN.md) — Rejected alternatives and local authenticator rationale
- [docs/SKILL.md](docs/SKILL.md) — Minimal API contract

## Usage

```go
import (
    "github.com/tinywasm/user"
    "github.com/tinywasm/auth/authority"
    "github.com/tinywasm/auth/oauth2"
    "github.com/tinywasm/auth/oauth2/provider/google"
    "github.com/tinywasm/rbac"
    "github.com/tinywasm/orm"
    "github.com/tinywasm/sqlite"
    "github.com/tinywasm/unixid"
)

ids, _ := unixid.NewUnixID()
conn, _ := sqlite.Open("app.db")
db := orm.New(conn)

// auth owns subjects and sessions
mod, _ := authority.New(db, auth.Config{IDs: ids})
gProv := &google.GoogleProvider{
    ClientID:     "client-id",
    ClientSecret: "secret",
    RedirectURL:  "https://example.com" + auth.PathOAuthCallback(google.ProviderName),
}
mod.Enable(oauth2.New(mod, mod, mod, []auth.OAuthProvider{gProv}))

// rbac owns assignments by SubjectID
rb, _ := rbac.New(db)

// composition root wires them: sessions from auth, authorization from rbac
// middleware: mod.Authenticate() sets ctx.UserID to string(user.SubjectID)
// authorize: rb.Can(ctx.UserID(), resource, action)
```

Local development: build a `local` authenticator with explicit scenarios and
mount it in the development composition root. Production builds use only the
Google provider and never register `local`. See `github.com/tinywasm/auth/local`.

## Status

Stable `SubjectID`/`Subject` contract. See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
for dependency rules.
