//go:build !wasm

package user

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/tinywasm/unixid"
)

func BeginOAuth(providerName string) (string, error) {
	p := getProvider(providerName)
	if p == nil {
		return "", ErrProviderNotFound
	}

	u, err := unixid.NewUnixID()
	if err != nil {
		return "", err
	}
	state := u.GetNewID()

	now := time.Now().Unix()
	expiresAt := now + 600 // 10 minutes

	if err := store.exec.Exec(
		"INSERT INTO user_oauth_states (state, provider, expires_at, created_at) VALUES (?, ?, ?, ?)",
		state, providerName, expiresAt, now,
	); err != nil {
		return "", err
	}

	return p.AuthCodeURL(state), nil
}

func CompleteOAuth(providerName string, r *http.Request, ip, ua string) (User, bool, error) {
	state := r.URL.Query().Get("state")
	if err := consumeState(state, providerName); err != nil {
		return User{}, false, ErrInvalidOAuthState
	}

	p := getProvider(providerName)
	if p == nil {
		return User{}, false, ErrProviderNotFound
	}

	token, err := p.ExchangeCode(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		return User{}, false, err
	}

	info, err := p.GetUserInfo(r.Context(), token)
	if err != nil {
		return User{}, false, err
	}

	identity, err := GetIdentityByProvider(providerName, info.ID)
	if err == nil {
		u, err := GetUser(identity.UserID)
		return u, false, err
	}

	u, err := GetUserByEmail(info.Email)
	if err == nil {
		_ = CreateIdentity(u.ID, providerName, info.ID, info.Email)
		return u, false, nil
	}

	u, err = CreateUser(info.Email, info.Name, "")
	if err != nil {
		return User{}, false, err
	}
	_ = CreateIdentity(u.ID, providerName, info.ID, info.Email)
	return u, true, nil
}

func consumeState(state, provider string) error {
	var expiresAt int64
	var dbProvider string
	err := store.exec.QueryRow("SELECT expires_at, provider FROM user_oauth_states WHERE state = ?", state).Scan(&expiresAt, &dbProvider)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrInvalidOAuthState
		}
		return err
	}

	if dbProvider != provider {
		return ErrInvalidOAuthState
	}

	// Delete state (single use) - done regardless of expiration to prevent reuse
	if err := store.exec.Exec("DELETE FROM user_oauth_states WHERE state = ?", state); err != nil {
		return err
	}

	if expiresAt < time.Now().Unix() {
		return ErrInvalidOAuthState
	}

	return nil
}

func PurgeExpiredOAuthStates() error {
	return store.exec.Exec("DELETE FROM user_oauth_states WHERE expires_at < ?", time.Now().Unix())
}
