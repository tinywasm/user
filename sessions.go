//go:build !wasm

package user

import (
	"time"

	"github.com/tinywasm/orm"
	"github.com/tinywasm/unixid"
)

// RotateSession atomically deletes the old session and creates a new one
// with the same userID, updated IP/UserAgent, and a fresh TTL.
// Prevents session fixation attacks when called post-login.
func (m *Module) RotateSession(oldID, ip, userAgent string) (Session, error) {
	oldSess, err := m.GetSession(oldID)
	if err != nil {
		return Session{}, err
	}

	err = m.DeleteSession(oldID)
	if err != nil {
		return Session{}, err
	}

	return m.CreateSession(oldSess.UserID, ip, userAgent)
}

func (m *Module) CreateSession(userID, ip, userAgent string) (Session, error) {
	u, err := unixid.NewUnixID()
	if err != nil {
		return Session{}, err
	}

	ttl := m.config.TokenTTL
	if ttl == 0 {
		ttl = 86400
	}

	now := time.Now().Unix()
	sess := Session{
		ID:        u.GetNewID(),
		UserID:    userID,
		ExpiresAt: now + int64(ttl),
		IP:        ip,
		UserAgent: userAgent,
		CreatedAt: now,
	}

	if err := m.db.Create(&sess); err != nil {
		return Session{}, err
	}
	m.cache.set(sess.ID, sess)
	return sess, nil
}

func (m *Module) GetSession(id string) (Session, error) {
	if s, ok := m.cache.get(id); ok {
		if s.ExpiresAt < time.Now().Unix() {
			m.cache.delete(id)
			return Session{}, ErrSessionExpired
		}
		return s, nil
	}

	qb := m.db.Query(&Session{}).Where(Session_.ID).Eq(id)
	results, err := ReadAllSession(qb)

	if err != nil {
		return Session{}, err
	}
	if len(results) == 0 {
		return Session{}, ErrNotFound
	}
	s := *results[0]

	if s.ExpiresAt < time.Now().Unix() {
		return Session{}, ErrSessionExpired
	}

	m.cache.set(s.ID, s)
	return s, nil
}

func (m *Module) DeleteSession(id string) error {
	m.cache.delete(id)
	qb := m.db.Query(&Session{}).Where(Session_.ID).Eq(id)
	results, err := ReadAllSession(qb)
	if err == nil && len(results) > 0 {
		return m.db.Delete(results[0], orm.Eq(Session_.ID, results[0].ID))
	}
	return err
}

func (m *Module) PurgeExpiredSessions() error {
	now := time.Now().Unix()

	m.cache.mu.Lock()
	for k, v := range m.cache.items {
		if v.ExpiresAt < now {
			delete(m.cache.items, k)
		}
	}
	m.cache.mu.Unlock()

	qb := m.db.Query(&Session{}).Where(Session_.ExpiresAt).Lt(now)
	sessions, _ := ReadAllSession(qb)
	for _, s := range sessions {
		m.db.Delete(s, orm.Eq(Session_.ID, s.ID))
	}

	return nil
}
