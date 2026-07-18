package authority

import (
	"sync"

	"github.com/tinywasm/events"
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/router"
	"github.com/tinywasm/time"
	"github.com/tinywasm/user"
)

type providerItem struct {
	key string
	val user.OAuthProvider
}

// Module is the user/auth/rbac handle. All backend operations are methods on this type.
// Created exclusively via New().
type Module struct {
	db          *orm.DB
	cache       *sessionCache
	ucache      *userCache
	config      user.Config
	log         func(...any)
	providers   []providerItem
	providersMu sync.RWMutex
	mu          sync.RWMutex
	ids         model.IDGenerator
	events      events.Publisher
}

// New initializes the user/rbac schema, warms the cache, and returns a Module handle.
// This is the ONLY entry point for this package on the backend.
func New(db *orm.DB, cfg user.Config) (*Module, error) {
	if cfg.IDs == nil {
		return nil, fmt.Err("user:", "Config.IDs", "is", "required")
	}

	if cfg.AuthMode == user.AuthModeJWT || cfg.AuthMode == user.AuthModeBearer {
		if len(cfg.JWTSecret) == 0 {
			return nil, fmt.Err("authority: JWTSecret required for selected AuthMode")
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
		providers: make([]providerItem, 0),
		ids:       cfg.IDs,
		events:    cfg.Events,
	}
	if err := initSchema(db, cfg.AuthMode); err != nil {
		return nil, err
	}
	for _, auth := range cfg.Authenticators {
		auth.Mount(nil, m)
	}
	if cfg.AuthMode == user.AuthModeCookie {
		if err := m.cache.warmUp(db); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// Expose public methods for modular authenticators to consume
func (m *Module) Config() user.Config { return m.config }
func (m *Module) DB() *orm.DB { return m.db }
func (m *Module) IDs() model.IDGenerator { return m.ids }
func (m *Module) Notify(e user.SecurityEvent) { m.notify(e) }
func (m *Module) IssueToken(userID string, ttl int) (string, error) { return m.issueToken(userID, ttl) }

func (m *Module) ExtractClientIP(ctx router.Context) string {
	return extractClientIP(ctx, m.config.TrustProxy)
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
	if m.events == nil {
		return
	}
	e.Timestamp = time.Now() / 1e9
	m.events.Publish(events.Event{Topic: user.TopicSecurity, Payload: &e})
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
	qb := m.db.Query(&user.Session{}).Where(user.Session_.UserId).Eq(userID)
	sessions, err := user.ReadAllSession(qb)
	if err != nil {
		return err
	}
	for _, s := range sessions {
		m.db.Delete(s, orm.Eq(user.Session_.Id, s.Id))
		m.cache.delete(s.Id)
	}
	return nil
}

func (m *Module) RegisterProvider(p user.OAuthProvider) {
	m.providersMu.Lock()
	defer m.providersMu.Unlock()
	for i, item := range m.providers {
		if item.key == p.Name() {
			m.providers[i].val = p
			return
		}
	}
	m.providers = append(m.providers, providerItem{key: p.Name(), val: p})
}

func (m *Module) getProvider(name string) user.OAuthProvider {
	m.providersMu.RLock()
	defer m.providersMu.RUnlock()
	for _, item := range m.providers {
		if item.key == name {
			return item.val
		}
	}
	return nil
}

func (m *Module) registeredProviders() []user.OAuthProvider {
	m.providersMu.RLock()
	defer m.providersMu.RUnlock()
	var list []user.OAuthProvider
	for _, item := range m.providers {
		list = append(list, item.val)
	}
	return list
}

// Add returns all admin-managed CRUDP handlers for registration.
// The concrete types are private — pass directly to crudp.RegisterHandlers.
//
// Usage: cp.RegisterHandlers(m.Add()...)
func (m *Module) Add() []any {
	return []any{
		&userCRUD{db: m.db, cache: m.ucache, ids: m.ids},
		&roleCRUD{m: m},
		&permissionCRUD{m: m},
		&lanipCRUD{m: m},
	}
}
