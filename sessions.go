//go:build !wasm

package user

import (
	"time"

	"github.com/tinywasm/unixid"
)

func CreateSession(userID, ip, userAgent string) (Session, error) {
	u, err := unixid.NewUnixID()
	if err != nil {
		return Session{}, err
	}

	ttl := store.config.SessionTTL
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

	if err := store.db.Create(&sess); err != nil {
		return Session{}, err
	}
	store.cache.set(sess.ID, sess)
	return sess, nil
}

func GetSession(id string) (Session, error) {
	if s, ok := store.cache.get(id); ok {
		if s.ExpiresAt < time.Now().Unix() {
			store.cache.delete(id)
			return Session{}, ErrSessionExpired
		}
		return s, nil
	}

	qb := store.db.Query(&Session{}).Where(SessionMeta.ID).Eq(id)
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

	store.cache.set(s.ID, s)
	return s, nil
}

func DeleteSession(id string) error {
	store.cache.delete(id)
	qb := store.db.Query(&Session{}).Where(SessionMeta.ID).Eq(id)
	results, err := ReadAllSession(qb)
	if err == nil && len(results) > 0 {
		return store.db.Delete(results[0])
	}
	return err
}

func PurgeExpiredSessions() error {
	now := time.Now().Unix()

	store.cache.mu.Lock()
	for k, v := range store.cache.items {
		if v.ExpiresAt < now {
			delete(store.cache.items, k)
		}
	}
	store.cache.mu.Unlock()

	qb := store.db.Query(&Session{}).Where(SessionMeta.ExpiresAt).Lt(now)
	sessions, _ := ReadAllSession(qb)
	for _, s := range sessions {
		store.db.Delete(s)
	}

	return nil
}
