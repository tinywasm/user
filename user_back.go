//go:build !wasm

package user

import "sync"

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
	sessionCookieName = cfg.SessionCookieName

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
