package user

import (
	"context"

	"github.com/tinywasm/fmt"
	"golang.org/x/oauth2"
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
	EventJWTTampered        SecurityEventType = iota // ValidateJWT: HMAC mismatch
	EventOAuthReplay                                 // consumeState: state already consumed (2nd use)
	EventOAuthExpiredState                           // consumeState: state found but past ExpiresAt
	EventOAuthCrossProvider                          // consumeState: provider mismatch (state preserved)
	EventIPMismatch                                  // LoginLAN: IP not registered
	EventNonActiveAccess                             // Login/LoginLAN: status != "active"
	EventUnauthorizedAccess                          // validateSession: cookie present but session invalid
	EventAccessDenied                                // AccessCheck: RBAC denied with valid session
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

type OAuthProvider interface {
	Name() string
	AuthCodeURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error)
	GetUserInfo(ctx context.Context, token *oauth2.Token) (OAuthUserInfo, error)
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
	// Structurally implements mcp.Authorizer via InjectIdentity + CanExecute methods.
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
}
