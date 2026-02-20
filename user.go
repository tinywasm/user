package user

import (
	"context"
	"sync"

	"github.com/tinywasm/fmt"
	"golang.org/x/oauth2"
)

var (
	ErrInvalidCredentials = fmt.Err("access", "denied")              // EN: Access Denied                    / ES: Acceso Denegado
	ErrSuspended          = fmt.Err("user", "suspended")             // EN: User Suspended                   / ES: Usuario Suspendido
	ErrEmailTaken         = fmt.Err("email", "registered")           // EN: Email Registered                 / ES: Correo electrónico Registrado
	ErrWeakPassword       = fmt.Err("password", "weak")              // EN: Password Weak                    / ES: Contraseña Débil
	ErrSessionExpired     = fmt.Err("token", "expired")              // EN: Token Expired                    / ES: Token Expirado
	ErrNotFound           = fmt.Err("user", "not", "found")          // EN: User Not Found                   / ES: Usuario No Encontrado
	ErrProviderNotFound   = fmt.Err("provider", "not", "found")      // EN: Provider Not Found               / ES: Proveedor No Encontrado
	ErrInvalidOAuthState  = fmt.Err("state", "invalid")              // EN: State Invalid                    / ES: Estado Inválido
	ErrCannotUnlink       = fmt.Err("identity", "cannot", "unlink")  // EN: Identity Cannot Unlink           / ES: Identidad No puede Desvincular
	ErrInvalidRUT         = fmt.Err("rut", "invalid")                // EN: Rut Invalid                      / ES: Rut Inválido
	ErrRUTTaken           = fmt.Err("rut", "registered")             // EN: Rut Registered                   / ES: Rut Registrado
	ErrIPTaken            = fmt.Err("ip", "registered")              // EN: Ip Registered                    / ES: Ip Registrado
)

type User struct {
	ID        string `json:"id"`
	Email     string `json:"email,omitempty"`
	Name      string `json:"name"`
	Phone     string `json:"phone,omitempty"`
	Status    string `json:"status"` // "active", "suspended"
	CreatedAt int64  `json:"created_at"`
}

type Session struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	ExpiresAt int64  `json:"expires_at"`
	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
	CreatedAt int64  `json:"created_at"`
}

type Identity struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	Provider   string `json:"provider"`
	ProviderID string `json:"provider_id"`
	Email      string `json:"email,omitempty"`
	CreatedAt  int64  `json:"created_at"`
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

type Config struct {
	SessionCookieName string          // default: "session"
	SessionTTL        int             // default: 86400 (24h)
	TrustProxy        bool            // default: false
	OAuthProviders    []OAuthProvider
}

type Store struct {
	exec   Executor
	cache  *sessionCache
	config Config
	mu     sync.RWMutex
}

var store *Store

func Init(exec Executor, cfg Config) error {
	if cfg.SessionCookieName == "" {
		cfg.SessionCookieName = "session"
	}
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = 86400
	}
	if err := runMigrations(exec); err != nil {
		return err
	}
	store = &Store{
		exec:   exec,
		cache:  newSessionCache(),
		config: cfg,
	}
	for _, p := range cfg.OAuthProviders {
		registerProvider(p)
	}
	return store.cache.warmUp(exec)
}

func SessionCookieName() string {
	if store == nil {
		return "session"
	}
	return store.config.SessionCookieName
}

// Global providers registry
var (
	providersMu sync.RWMutex
	providers   = make(map[string]OAuthProvider)
)

func registerProvider(p OAuthProvider) {
	providersMu.Lock()
	defer providersMu.Unlock()
	providers[p.Name()] = p
}

func getProvider(name string) OAuthProvider {
	providersMu.RLock()
	defer providersMu.RUnlock()
	return providers[name]
}

func registeredProviders() []OAuthProvider {
	providersMu.RLock()
	defer providersMu.RUnlock()
	var list []OAuthProvider
	for _, p := range providers {
		list = append(list, p)
	}
	return list
}
