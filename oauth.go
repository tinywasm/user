//go:build !wasm

package user

import (
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

	stateObj := &OAuthState{
		State:     state,
		Provider:  providerName,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}

	if err := store.db.Create(stateObj); err != nil {
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
	qb := store.db.Query(&OAuthState{}).Where(OAuthStateMeta.State).Eq(state)
	results, err := ReadAllOAuthState(qb)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return ErrInvalidOAuthState
	}
	stateObj := results[0]

	if stateObj.Provider != provider {
		return ErrInvalidOAuthState
	}

	// Delete state (single use) - done regardless of expiration to prevent reuse
	if err := store.db.Delete(stateObj); err != nil {
		return err
	}

	if stateObj.ExpiresAt < time.Now().Unix() {
		return ErrInvalidOAuthState
	}

	return nil
}

func PurgeExpiredOAuthStates() error {
	qb := store.db.Query(&OAuthState{}).Where(OAuthStateMeta.ExpiresAt).Lt(time.Now().Unix())
	states, _ := ReadAllOAuthState(qb)
	for _, s := range states {
		store.db.Delete(s)
	}
	return nil
}
