//go:build !wasm

package user

import (
	"database/sql"
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

	if err := store.exec.Exec(
		`INSERT INTO user_sessions (id, user_id, expires_at, ip, user_agent, created_at)
         VALUES (?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.UserID, sess.ExpiresAt, sess.IP, sess.UserAgent, sess.CreatedAt,
	); err != nil {
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

	var s Session
	err := store.exec.QueryRow(
		"SELECT id, user_id, expires_at, ip, user_agent, created_at FROM user_sessions WHERE id = ?",
		id,
	).Scan(&s.ID, &s.UserID, &s.ExpiresAt, &s.IP, &s.UserAgent, &s.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return Session{}, ErrNotFound
		}
		return Session{}, err
	}

	if s.ExpiresAt < time.Now().Unix() {
		return Session{}, ErrSessionExpired
	}

	store.cache.set(s.ID, s)
	return s, nil
}

func DeleteSession(id string) error {
	store.cache.delete(id)
	return store.exec.Exec("DELETE FROM user_sessions WHERE id = ?", id)
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

	return store.exec.Exec("DELETE FROM user_sessions WHERE expires_at < ?", now)
}
