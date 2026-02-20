package user

import (
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

func (c *sessionCache) warmUp(exec Executor) error {
	rows, err := exec.Query("SELECT id, user_id, expires_at, ip, user_agent, created_at FROM user_sessions WHERE expires_at > ?", time.Now().Unix())
	if err != nil {
		return err
	}
	defer rows.Close()

	c.mu.Lock()
	defer c.mu.Unlock()

	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.UserID, &s.ExpiresAt, &s.IP, &s.UserAgent, &s.CreatedAt); err != nil {
			return err
		}
		c.items[s.ID] = s
	}
	return rows.Err()
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
