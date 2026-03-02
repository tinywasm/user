//go:build !wasm

package user

import (
	"sync"

	"github.com/tinywasm/orm"
)

type Store struct {
	db        *orm.DB
	cache     *sessionCache
	userCache *userCache
	config    Config
	mu        sync.RWMutex
}

var store *Store

func Init(db *orm.DB, cfg Config) error {
	if cfg.SessionCookieName == "" {
		cfg.SessionCookieName = "session"
	}
	sessionCookieName = cfg.SessionCookieName

	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = 86400
	}
	store = &Store{
		db:        db,
		cache:     newSessionCache(),
		userCache: newUserCache(),
		config:    cfg,
	}
	if err := initSchema(db); err != nil {
		return err
	}
	for _, p := range cfg.OAuthProviders {
		registerProvider(p)
	}
	return store.cache.warmUp()
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
