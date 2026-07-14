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
