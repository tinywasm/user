package userserver

import "github.com/tinywasm/model"

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/form"
	"sync"
	"time"

	"github.com/tinywasm/orm"
	"github.com/tinywasm/user"
)

// Module is the user/auth/rbac handle. All backend operations are methods on this type.
// Created exclusively via New().
type Module struct {
	db          *orm.DB
	cache       *sessionCache
	ucache      *userCache
	config      user.Config
	log         func(...any)
	providers   map[string]user.OAuthProvider
	providersMu sync.RWMutex
	mu          sync.RWMutex
}

// New initializes the user/rbac schema, warms the cache, and returns a Module handle.
// This is the ONLY entry point for this package on the backend.
func New(db *orm.DB, cfg user.Config) (*Module, error) {
	if cfg.AuthMode == user.AuthModeJWT || cfg.AuthMode == user.AuthModeBearer {
		if len(cfg.JWTSecret) == 0 {
			return nil, fmt.Err("userserver: JWTSecret required for selected AuthMode")
		}
	}

	if cfg.CookieName == "" {
		cfg.CookieName = "session"
	}
	if cfg.TokenTTL == 0 {
		cfg.TokenTTL = 86400
	}
	m := &Module{
		db:        db,
		cache:     newSessionCache(),
		ucache:    newUserCache(),
		config:    cfg,
		providers: make(map[string]user.OAuthProvider),
	}
	if err := initSchema(db, cfg.AuthMode); err != nil {
		return nil, err
	}
	for _, p := range cfg.OAuthProviders {
		m.registerProvider(p)
	}
	if cfg.AuthMode == user.AuthModeCookie {
		if err := m.cache.warmUp(db); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// SetLog configures optional logging. Call immediately after New().
// Default: no-op. Follows the tinywasm ecosystem SetLog convention (same as rbac).
//
// Example:
//
//	m.SetLog(func(msg ...any) { log.Println(msg...) })
func (m *Module) SetLog(fn func(...any)) {
	m.log = fn
}

func (m *Module) notify(e user.SecurityEvent) {
	if e.Timestamp == 0 {
		e.Timestamp = time.Now().Unix()
	}
	if m.config.OnSecurityEvent != nil {
		m.config.OnSecurityEvent(e)
		return
	}
	if m.log != nil {
		m.log("security_event", e.Type, e.IP, e.UserID)
	}
}

// SuspendUser sets Status = "suspended". Evicts user from cache.
func (m *Module) SuspendUser(id string) error {
	return suspendUser(m.db, m.ucache, id)
}

// ReactivateUser sets Status = "active". Evicts user from cache.
func (m *Module) ReactivateUser(id string) error {
	return reactivateUser(m.db, m.ucache, id)
}

// PurgeSessionsByUser deletes all sessions belonging to userID from cache and DB.
func (m *Module) PurgeSessionsByUser(userID string) error {
	// First from DB
	qb := m.db.Query(&user.Session{}).Where(user.Session_.UserID).Eq(userID)
	sessions, err := user.ReadAllSession(qb)
	if err != nil {
		return err
	}
	for _, s := range sessions {
		m.db.Delete(s, orm.Eq(user.Session_.ID, s.ID))
		m.cache.delete(s.ID)
	}
	return nil
}

func (m *Module) registerProvider(p user.OAuthProvider) {
	m.providersMu.Lock()
	defer m.providersMu.Unlock()
	m.providers[p.Name()] = p
}

func (m *Module) getProvider(name string) user.OAuthProvider {
	m.providersMu.RLock()
	defer m.providersMu.RUnlock()
	return m.providers[name]
}

func (m *Module) registeredProviders() []user.OAuthProvider {
	m.providersMu.RLock()
	defer m.providersMu.RUnlock()
	var list []user.OAuthProvider
	for _, p := range m.providers {
		list = append(list, p)
	}
	return list
}

// Add returns all admin-managed CRUDP handlers for registration.
// The concrete types are private — pass directly to crudp.RegisterHandlers.
//
// Usage: cp.RegisterHandlers(m.Add()...)
func (m *Module) Add() []any {
	return []any{
		&userCRUD{db: m.db, cache: m.ucache},
		&roleCRUD{m: m},
		&permissionCRUD{m: m},
		&lanipCRUD{m: m},
	}
}

// UIModules returns all standard authentication UI flow handlers bound to this module.
// Note: This returns the backend-side modules with SSR capability.
func (m *Module) UIModules() []any {
	return []any{
		&loginModule{m: m, form: mustForm("login", &user.LoginData{})},
		&registerModule{m: m, form: mustForm("register", &user.RegisterData{})},
		&profileModule{
			m:            m,
			form:         mustForm("profile", &user.ProfileData{}),
			passwordForm: mustForm("password", &user.PasswordData{}),
		},
		&lanModule{m: m},
		&oauthModule{m: m},
	}
}

func mustForm(parentID string, s model.Fielder) *form.Form {
	f, err := form.New(parentID, s)
	if err != nil {
		panic("userserver: mustForm: " + err.Error())
	}
	return f
}
