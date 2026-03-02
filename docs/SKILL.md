# tinywasm/user Skill

## Description
The `tinywasm/user` library manages user entities, authentication (password, OAuth, LAN), HTTP sessions, and provides isomorphic UI modules for auth flows (login, register, profile).

## Core Concepts
- **Identity-based authentication:** The `users` table holds no auth secrets. Passwords are in `user_identities` (`provider='local'`). OAuth and LAN users have no local password.
- **Integrated RBAC:** `User` structs are explicitly hydrated with their `Roles` and `Permissions` via sequential queries from `tinywasm/orm`.
- **Integrated Cache:** An in-memory cache manages HTTP sessions and up to 1000 hydrated users. Mutations immediately trigger cache invalidations.
- **Isomorphic Modules:** UI modules implement `site.Module` via duck typing.

## Public API Contract

### Configuration
```go
// Config — pass to Init. Zero values use defaults.
type Config struct {
    SessionCookieName string          // default: "session"
    SessionTTL        int             // default: 86400 (24h), in seconds
    TrustProxy        bool            // default: false
    OAuthProviders    []OAuthProvider // optional; register before Init
}

// Init — creates tables via ORM, warms session cache, applies config.
func Init(db *orm.DB, cfg Config) error
```

### UI Modules
Register into `site.RegisterHandlers()`:
- `user.LoginModule`: `/login` (email+password form + OAuth buttons)
- `user.RegisterModule`: `/register`
- `user.ProfileModule`: `/profile` (edit name, phone, password)
- `user.LANModule`: `/lan` (LAN IP management)
- `user.OAuthCallback`: `/oauth/callback`

### User CRUD
```go
// CreateUser does NOT set a password. Call SetPassword separately.
func CreateUser(email, name, phone string) (User, error)  
func GetUser(id string) (User, error)
func GetUserByEmail(email string) (User, error)
func UpdateUser(id, name, phone string) error
func SuspendUser(id string) error
func ReactivateUser(id string) error  
```

### Authentication & Passwords
```go
func SetPassword(userID, password string) error     
func VerifyPassword(userID, password string) error   
func Login(email, password string) (User, error)
```

### Sessions
```go
func CreateSession(userID, ip, userAgent string) (Session, error)
func GetSession(id string) (Session, error)           
func DeleteSession(id string) error
func PurgeExpiredSessions() (int64, error)             
```

### OAuth & Identities
```go
func GetUserIdentities(userID string) ([]Identity, error)
func UnlinkIdentity(userID, provider string) error     
```

### LAN Authentication
```go
func LoginLAN(rut string, r *http.Request) (User, error)
func RegisterLAN(userID, rut string) error
func UnregisterLAN(userID string) error  
func AssignLANIP(userID, ip, label string) error
func RevokeLANIP(userID, ip string) error
func GetLANIPs(userID string) ([]LANIP, error)
```

## Integration with `tinywasm/site`

```go
// main.go setup

// 1. Configure site
site.SetDB(db)
site.SetUserID(extractUserID) // e.g. reads session cookie, calls user.GetSession

// 2. Configure user
site.SetUserConfig(user.Config{
    SessionTTL: 86400,
})

// 3. Register UI modules
site.RegisterHandlers(
    user.LoginModule,
    user.RegisterModule,
    user.ProfileModule,
    user.LANModule,
    &myapp.Dashboard{},
)

// 4. Serve
site.Serve(":8080") // internal: applies DB and User configs
```
