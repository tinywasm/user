//go:build !wasm

package user

import (
	"github.com/tinywasm/orm"

	"sync"
	"time"
)

type sessionCache struct {
	mu    sync.RWMutex
	items map[string]Session
}

func newSessionCache() *sessionCache {
	return &sessionCache{
		items: make(map[string]Session),
	}
}

func (c *sessionCache) warmUp(db *orm.DB) error {
	qb := db.Query(&Session{}).Where(SessionMeta.ExpiresAt).Gt(time.Now().Unix())
	sessions, err := ReadAllSession(qb)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, s := range sessions {
		c.items[s.ID] = *s
	}
	return nil
}

func (c *sessionCache) set(id string, s Session) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[id] = s
}

func (c *sessionCache) get(id string) (Session, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.items[id]
	return s, ok
}

func (c *sessionCache) delete(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, id)
}
