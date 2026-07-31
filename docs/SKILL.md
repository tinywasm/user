# tinywasm/user Skill

## Description
The `tinywasm/user` library manages user entities, authentication (password, OAuth, LAN), HTTP sessions, and RBAC.

## Core Concepts
- **Typed Definitions:** All models are `model.Definition` literals in `models.go`.
- **Identity-based authentication:** Secrets are in `user_identities`.
- **Integrated RBAC:** Users are hydrated with permissions.
- **No Views:** Views belong to the consumer. The library serves flow endpoints.

## Public API Contract

### Configuration
```go
type Config struct {
    AuthMode   AuthMode // AuthModeCookie | AuthModeJWT | AuthModeBearer
    CookieName string
    TokenTTL   int
    JWTSecret  []byte
    TrustProxy bool
    OAuthProviders []OAuthProvider
    AfterLoginPath string
}
```

### Server API
```go
// New (in authority)
func New(db *orm.DB, cfg Config) (*Module, error)

// MountAPI registers:
// POST /login, POST /logout, GET /oauth/:provider, GET /oauth/callback/:provider
func (m *Module) MountAPI(r router.Router)

func (m *Module) Bootstrap(s Seed) error

func (m *Module) GetUserByEmail(email string) (User, error)
```

### Authentication & Authorization
```go
func (m *Module) Login(email, password string) (User, error)
func (m *Module) Authenticate() router.Middleware
func (m *Module) Can(userID string, resource model.Resource, action model.Action) bool
```

## Consumer Integration

The application builds its own login page using `tinywasm/form` and the generated `user.LoginData` struct:

```go
// In consumer view
f := form.New("login", &user.LoginData{})
// ... render form and post to user.PathLogin
```

### End-to-End Application Chassis Integration

In a TinyWasm client application rendering via `platformd`, you can cleanly wire the logged-in session/identity into the chassis using the wsam-safe `ShellProfile`:

```go
//go:build wasm

package main

import (
	. "github.com/tinywasm/dom"
	. "github.com/tinywasm/html"
	"github.com/tinywasm/layout/platformd"
	"github.com/tinywasm/user"
)

// clientSession manages fetching and keeping track of the logged-in user's profile
type clientSession struct {
	profile *Signal[*user.ProfileDTO]
}

func (s *clientSession) Init(ctx Ctx) {
	s.profile = NewSignal[*user.ProfileDTO](nil)
	s.fetchProfile()
}

func (s *clientSession) fetchProfile() {
	// Call 'me' API operation to retrieve ProfileDTO from the server
	// (assume client has a network/router caller injected)
}

func main() {
	sess := &clientSession{}

	// UserBlock component to render in the platformd header/drawer
	userBlock := Div().BindChildren(DeriveNodes(func() []Component {
		p := sess.profile.Get()
		if p == nil {
			return []Component{Span().Text("Cargando...")}
		}

		// Convert ProfileDTO into wsam-safe ShellProfile
		identity := p.Shell()

		return []Component{
			Div().Set(Class("identity-display").AsAttr()).Child(
				Span().Set(Class("identity-name").AsAttr()).Text(identity.UserName()),
				Span().Set(Class("identity-area").AsAttr()).Text(identity.UserArea()),
			),
		}
	}))

	p := &platformd.Platform{
		AppName:   "Mi Plataforma",
		UserBlock: userBlock,
		Modules:   []platformd.UIModule{
			// ... register your app UI modules
		},
	}

	Append("body", p)
}
```
