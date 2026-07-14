package userserver

import (
	"sync"
	"github.com/tinywasm/time"

	"github.com/tinywasm/orm"
	"github.com/tinywasm/unixid"
	"github.com/tinywasm/user"
)

type sessionCache struct {
	mu    sync.RWMutex
	items map[string]user.Session
}

func newSessionCache() *sessionCache {
	return &sessionCache{
		items: make(map[string]user.Session),
	}
}

func (c *sessionCache) warmUp(db *orm.DB) error {
	qb := db.Query(&user.Session{}).Where(user.Session_.ExpiresAt).Gt(time.Now() / 1e9)
	sessions, err := user.ReadAllSession(qb)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, s := range sessions {
		c.items[s.Id] = *s
	}
	return nil
}

func (c *sessionCache) set(id string, s user.Session) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[id] = s
}

func (c *sessionCache) get(id string) (user.Session, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.items[id]
	return s, ok
}

func (c *sessionCache) delete(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, id)
}

// RotateSession atomically deletes the old session and creates a new one
// with the same userID, updated IP/UserAgent, and a fresh TTL.
// Prevents session fixation attacks when called post-login.
func (m *Module) RotateSession(oldID, ip, userAgent string) (user.Session, error) {
	oldSess, err := m.GetSession(oldID)
	if err != nil {
		return user.Session{}, err
	}

	err = m.DeleteSession(oldID)
	if err != nil {
		return user.Session{}, err
	}

	return m.CreateSession(oldSess.UserId, ip, userAgent)
}

func (m *Module) CreateSession(userID, ip, userAgent string) (user.Session, error) {
	u, err := unixid.NewUnixID()
	if err != nil {
		return user.Session{}, err
	}

	ttl := m.config.TokenTTL
	if ttl == 0 {
		ttl = 86400
	}

	now := time.Now() / 1e9
	sess := user.Session{
		Id:        u.GetNewID(),
		UserId:    userID,
		ExpiresAt: now + int64(ttl),
		Ip:        ip,
		UserAgent: userAgent,
		CreatedAt: now,
	}

	if err := m.db.Create(&sess); err != nil {
		return user.Session{}, err
	}
	m.cache.set(sess.Id, sess)
	return sess, nil
}

func (m *Module) GetSession(id string) (user.Session, error) {
	if s, ok := m.cache.get(id); ok {
		if s.ExpiresAt < time.Now() / 1e9 {
			m.cache.delete(id)
			return user.Session{}, user.ErrSessionExpired
		}
		return s, nil
	}

	qb := m.db.Query(&user.Session{}).Where(user.Session_.Id).Eq(id)
	results, err := user.ReadAllSession(qb)

	if err != nil {
		return user.Session{}, err
	}
	if len(results) == 0 {
		return user.Session{}, user.ErrNotFound
	}
	s := *results[0]

	if s.ExpiresAt < time.Now() / 1e9 {
		return user.Session{}, user.ErrSessionExpired
	}

	m.cache.set(s.Id, s)
	return s, nil
}

func (m *Module) DeleteSession(id string) error {
	m.cache.delete(id)
	qb := m.db.Query(&user.Session{}).Where(user.Session_.Id).Eq(id)
	results, err := user.ReadAllSession(qb)
	if err == nil && len(results) > 0 {
		return m.db.Delete(results[0], orm.Eq(user.Session_.Id, results[0].Id))
	}
	return err
}

func (m *Module) PurgeExpiredSessions() error {
	now := time.Now() / 1e9

	m.cache.mu.Lock()
	for k, v := range m.cache.items {
		if v.ExpiresAt < now {
			delete(m.cache.items, k)
		}
	}
	m.cache.mu.Unlock()

	qb := m.db.Query(&user.Session{}).Where(user.Session_.ExpiresAt).Lt(now)
	sessions, _ := user.ReadAllSession(qb)
	for _, s := range sessions {
		m.db.Delete(s, orm.Eq(user.Session_.Id, s.Id))
	}

	return nil
}
