package user

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
)

var (
	ErrInvalidCredentials = fmt.Err("access", "denied")             // EN: Access Denied                    / ES: Acceso Denegado
	ErrSuspended          = fmt.Err("user", "suspended")            // EN: User Suspended                   / ES: Usuario Suspendido
	ErrEmailTaken         = fmt.Err("email", "registered")          // EN: Email Registered                 / ES: Correo electrónico Registrado
	ErrWeakPassword       = fmt.Err("password", "weak")             // EN: Password Weak                    / ES: Contraseña Débil
	ErrSessionExpired     = fmt.Err("token", "expired")             // EN: Token Expired                    / ES: Token Expirado
	ErrNotFound           = fmt.Err("user", "not", "found")         // EN: User Not Found                   / ES: Usuario No Encontrado
	ErrProviderNotFound   = fmt.Err("provider", "not", "found")     // EN: Provider Not Found               / ES: Proveedor No Encontrado
	ErrInvalidOAuthState  = fmt.Err("state", "invalid")             // EN: State Invalid                    / ES: Estado Inválido
	ErrCannotUnlink       = fmt.Err("identity", "cannot", "unlink") // EN: Identity Cannot Unlink           / ES: Identidad No puede Desvincular
	ErrInvalidRUT         = fmt.Err("rut", "invalid")               // EN: Rut Invalid                      / ES: Rut Inválido
	ErrRUTTaken           = fmt.Err("rut", "registered")            // EN: Rut Registered                   / ES: Rut Registrado
	ErrIPTaken            = fmt.Err("ip", "registered")             // EN: Ip Registered                    / ES: Ip Registrado
)

type SecurityEventType uint8

const (
	EventJWTTampered        SecurityEventType = iota // validateJWT: jwt.Forged (never jwt.Expired)
	EventOAuthReplay                                 // consumeState: state already consumed (2nd use)
	EventOAuthExpiredState                           // consumeState: state found but past ExpiresAt
	EventOAuthCrossProvider                          // consumeState: provider mismatch (state preserved)
	EventIPMismatch                                  // LoginLAN: IP not registered
	EventNonActiveAccess                             // Login/LoginLAN: status != "active"
	EventUnauthorizedAccess                          // validateSession: cookie present but session invalid
	EventAccessDenied                                // AccessCheck: RBAC denied with valid session
	EventPermissionCorrupt                           // HasPermission: permissions.action is not a CRUD string
	EventRateLimited                                 // POST /login: Config.RateLimit rejected the attempt before bcrypt
)

type SecurityEvent struct {
	Type      SecurityEventType
	IP        string // client IP, empty if not available
	UserID    string // empty if user not yet identified
	Provider  string // OAuth provider name, for OAuth events
	Resource  string // RBAC resource, for EventAccessDenied
	Timestamp int64  // time.Now().Unix()
}

type OAuthUserInfo struct {
	ID    string
	Email string
	Name  string
}

// OAuthToken is what a provider returns when it exchanges the code. It replaces
// oauth2.Token: that type dragged net/http in, and net/http does not exist under TinyGo —
// which put this whole module out of the edge for one function call.
type OAuthToken struct {
	AccessToken string
	TokenType   string
	ExpiresIn   int
}

func (t *OAuthToken) DecodeFields(r model.FieldReader) {
	t.AccessToken, _ = r.String("access_token")
	t.TokenType, _ = r.String("token_type")
	exp, _ := r.Int("expires_in")
	t.ExpiresIn = int(exp)
}

func (t OAuthToken) IsNil() bool { return false }

// OAuthConfig is the provider's registration: what the app declares in the provider console.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	AuthURL      string // provider's authorization endpoint
	TokenURL     string // provider's token endpoint
}

type OAuthProvider interface {
	Name() string
	AuthCodeURL(state string) string
	ExchangeCode(code string) (OAuthToken, error)
	GetUserInfo(token OAuthToken) (OAuthUserInfo, error)
}

// AuthMode selects the session strategy.
type AuthMode uint8

const (
	// AuthModeCookie stores a session ID in an HttpOnly cookie.
	// Stateful: requires user_sessions table. Supports immediate revocation.
	AuthModeCookie AuthMode = iota // default

	// AuthModeJWT stores a signed JWT in an HttpOnly cookie.
	// Stateless: no DB lookup per request. No immediate revocation.
	// Ideal for SPA/PWA and multi-server deployments.
	AuthModeJWT

	// AuthModeBearer reads a signed JWT from the "Authorization: Bearer <token>" header.
	// Stateless: for API clients (MCP servers, IDEs, LLMs) that cannot use cookies.
	// Requires JWTSecret.
	AuthModeBearer
)

type Config struct {
	AuthMode AuthMode // default: AuthModeCookie

	// Shared by all modes
	CookieName string // default: "session"
	TokenTTL   int    // default: 86400 (seconds). Session TTL in cookie mode, JWT expiry in JWT mode.

	// Required when AuthMode == AuthModeJWT or AuthMode == AuthModeBearer.
	// Also required to call GenerateAPIToken regardless of AuthMode.
	JWTSecret []byte

	TrustProxy     bool
	OAuthProviders []OAuthProvider

	// Optional hook for receiving security events (e.g. tampering, brute force)
	OnSecurityEvent func(SecurityEvent)

	// OnPasswordValidate is called by SetPassword before hashing.
	// Return a non-nil error to reject the password.
	// If nil, only the built-in len >= 8 check applies.
	OnPasswordValidate func(password string) error

	// AfterLoginPath is the path to redirect to after successful login.
	// Default: PathAfterLogin ("/")
	AfterLoginPath string

	// RateLimit is called by endpoints before processing a request.
	// Return a non-nil error to reject the request (429 Too Many Requests).
	// remoteAddr is the client's IP address.
	RateLimit func(remoteAddr string) error
}

const (
	PathLogin      = "/login"
	PathLogout     = "/logout"
	PathAfterLogin = "/"

)

// ProfileDTO is a safe subset of User data for public/API consumption.
type ProfileDTO struct {
	Id          string
	Name        string
	Email       string
	Avatar      string
	Roles       []string
	Permissions []string // "resource:actions" pairs, e.g. "service_catalog:rc"
	Locale      string
}

func (p ProfileDTO) EncodeFields(w model.FieldWriter) {
	w.String("id", p.Id)
	w.String("name", p.Name)
	w.String("email", p.Email)
	w.String("avatar", p.Avatar)
	w.String("locale", p.Locale)
	aw := w.Array("roles", len(p.Roles))
	for _, r := range p.Roles {
		aw.String(r)
	}
	aw.Close()
	pw := w.Array("permissions", len(p.Permissions))
	for _, perm := range p.Permissions {
		pw.String(perm)
	}
	pw.Close()
}

func (p ProfileDTO) IsNil() bool { return false }

func (p *ProfileDTO) DecodeFields(r model.FieldReader) {
	p.Id, _ = r.String("id")
	p.Name, _ = r.String("name")
	p.Email, _ = r.String("email")
	p.Avatar, _ = r.String("avatar")
	p.Locale, _ = r.String("locale")
	if ar, ok := r.Array("roles"); ok {
		p.Roles = make([]string, ar.Len())
		for i := 0; i < ar.Len(); i++ {
			p.Roles[i] = ar.String(i)
		}
	}
	if ap, ok := r.Array("permissions"); ok {
		p.Permissions = make([]string, ap.Len())
		for i := 0; i < ap.Len(); i++ {
			p.Permissions[i] = ap.String(i)
		}
	}
}
