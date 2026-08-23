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
// POST /login (user.PathLogin), POST /logout (user.PathLogout),
// GET user.PathOAuthStart(provider), GET user.PathOAuthCallback(provider)
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

| Quiero… | Uso |
|---|---|
| enlazar el botón de login social | `user.PathOAuthStart(google.ProviderName)` |
| registrar la URI de redirección en el proveedor | dominio público + `user.PathOAuthCallback(google.ProviderName)` |

The application builds its own login page using `tinywasm/form` and the generated `user.LoginData` struct:

```go
// In consumer view
f := form.New("login", &user.LoginData{})
// ... render form and post to user.PathLogin
```

---

## Wiring Sessions into layout.Platform (End-to-End Client snippet)

Here is a snippet showing how to fetch the authenticated caller's profile via the `me` RPC/Op and wire it directly into the application chassis shell (`platformd.Platform`) using the `ShellProfile` adapter:

```go
package client

import (
	"github.com/tinywasm/layout/platformd"
	"github.com/tinywasm/view"
	"github.com/tinywasm/user"
)

type ShellController struct {
	caller view.Caller
	shell  *platformd.Platform
}

func NewShellController(caller view.Caller) *ShellController {
	return &ShellController{caller: caller}
}

func (sc *ShellController) Initialize() {
	// 1. Query the 'me' operation to fetch current ProfileDTO
	sc.caller.Call(user.OpMe, nil, &user.ProfileDTO{}, func(err error) {
		if err != nil {
			// Handle unauthenticated state (e.g., redirect to login view)
			return
		}

		// 2. Wrap ProfileDTO in ShellProfile to satisfy platformd.Identity
		var profile user.ProfileDTO
		// ... (assume profile was successfully unmarshaled into sc.profile)
		shellIdentity := profile.Shell()

		// 3. Inject the identity directly into the platform layout chassis
		sc.shell = &platformd.Platform{
			AppName:   "Enterprise Portal",
			UserBlock: platformd.NewUserBlock(shellIdentity),
			// ... other configurations
		}
	})
}
```
