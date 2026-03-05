//go:build !wasm

package user

import (
	"net/http"
	"time"

	"github.com/tinywasm/orm"
	"github.com/tinywasm/unixid"
)

func (m *Module) BeginOAuth(providerName string) (string, error) {
	p := m.getProvider(providerName)
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

	if err := m.db.Create(stateObj); err != nil {
		return "", err
	}

	return p.AuthCodeURL(state), nil
}

func (m *Module) CompleteOAuth(providerName string, r *http.Request, ip, ua string) (User, bool, error) {
	state := r.URL.Query().Get("state")
	if err := consumeState(m.db, state, providerName); err != nil {
		return User{}, false, ErrInvalidOAuthState
	}

	p := m.getProvider(providerName)
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

	identity, err := getIdentityByProvider(m.db, providerName, info.ID)
	if err == nil {
		u, err := getUser(m.db, m.ucache, identity.UserID)
		return u, false, err
	}

	u, err := getUserByEmail(m.db, m.ucache, info.Email)
	if err == nil {
		_ = createIdentity(m.db, u.ID, providerName, info.ID, info.Email)
		return u, false, nil
	}

	u, err = createUser(m.db, info.Email, info.Name, "")
	if err != nil {
		return User{}, false, err
	}
	_ = createIdentity(m.db, u.ID, providerName, info.ID, info.Email)
	return u, true, nil
}

func consumeState(db *orm.DB, state, provider string) error {
	qb := db.Query(&OAuthState{}).Where(OAuthState_.State).Eq(state)
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
	if err := db.Delete(stateObj, orm.Eq(OAuthState_.State, stateObj.State)); err != nil {
		return err
	}

	if stateObj.ExpiresAt < time.Now().Unix() {
		return ErrInvalidOAuthState
	}

	return nil
}

func (m *Module) PurgeExpiredOAuthStates() error {
	qb := m.db.Query(&OAuthState{}).Where(OAuthState_.ExpiresAt).Lt(time.Now().Unix())
	states, _ := ReadAllOAuthState(qb)
	for _, s := range states {
		m.db.Delete(s, orm.Eq(OAuthState_.State, s.State))
	}
	return nil
}
