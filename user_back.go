//go:build !wasm

package user

import (
	"sync"

	"github.com/tinywasm/orm"
)

// Module is the user/auth/rbac handle. All backend operations are methods on this type.
// Created exclusively via New().
type Module struct {
	db          *orm.DB
	cache       *sessionCache
	ucache      *userCache
	config      Config
	log         func(...any)
	providers   map[string]OAuthProvider
	providersMu sync.RWMutex
	mu          sync.RWMutex
}

// New initializes the user/rbac schema, warms the cache, and returns a Module handle.
// This is the ONLY entry point for this package on the backend.
func New(db *orm.DB, cfg Config) (*Module, error) {
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
		providers: make(map[string]OAuthProvider),
	}
	if err := initSchema(db, cfg.AuthMode); err != nil {
		return nil, err
	}
	for _, p := range cfg.OAuthProviders {
		m.registerProvider(p)
	}
	if cfg.AuthMode == AuthModeCookie {
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

func (m *Module) registerProvider(p OAuthProvider) {
	m.providersMu.Lock()
	defer m.providersMu.Unlock()
	m.providers[p.Name()] = p
}

func (m *Module) getProvider(name string) OAuthProvider {
	m.providersMu.RLock()
	defer m.providersMu.RUnlock()
	return m.providers[name]
}

func (m *Module) registeredProviders() []OAuthProvider {
	m.providersMu.RLock()
	defer m.providersMu.RUnlock()
	var list []OAuthProvider
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
// Isomorphic: The signature exists in both WASM and backend. On the backend, it links to the DB.
func (m *Module) UIModules() []any {
	for _, mod := range uiModules {
		if lm, ok := mod.(*loginModule); ok {
			lm.m = m
		} else if rm, ok := mod.(*registerModule); ok {
			rm.m = m
		} else if pm, ok := mod.(*profileModule); ok {
			pm.m = m
		} else if lam, ok := mod.(*lanModule); ok {
			lam.m = m
		} else if om, ok := mod.(*oauthModule); ok {
			om.m = m
		}
	}
	return uiModules
}
