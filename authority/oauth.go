package authority

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/router"
	"github.com/tinywasm/time"
	"github.com/tinywasm/user"
)

func (m *Module) BeginOAuth(providerName string) (string, error) {
	p := m.getProvider(providerName)
	if p == nil {
		return "", user.ErrProviderNotFound
	}

	state := m.ids.NewID()

	now := time.Now() / 1e9
	expiresAt := now + 600 // 10 minutes

	stateObj := &user.OAuthState{
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

func (m *Module) CompleteOAuth(providerName string, ctx router.Context, ip, ua string) (user.User, bool, error) {
	var state, code string
	path := ctx.Path()
	if fmt.Contains(path, "?") {
		query := fmt.Split(path, "?")[1]
		parts := fmt.Split(query, "&")
		for _, part := range parts {
			kv := fmt.Split(part, "=")
			if len(kv) == 2 {
				if kv[0] == "state" {
					state = kv[1]
				} else if kv[0] == "code" {
					code = kv[1]
				}
			}
		}
	}

	if err := consumeState(m.db, state, providerName); err != nil {
		return user.User{}, false, user.ErrInvalidOAuthState
	}

	p := m.getProvider(providerName)
	if p == nil {
		return user.User{}, false, user.ErrProviderNotFound
	}

	token, err := p.ExchangeCode(code)
	if err != nil {
		return user.User{}, false, err
	}

	info, err := p.GetUserInfo(token)
	if err != nil {
		return user.User{}, false, err
	}

	identity, err := getIdentityByProvider(m.db, providerName, info.ID)
	if err == nil {
		u, err := getUser(m.db, m.ucache, identity.UserId)
		return u, false, err
	}

	u, err := getUserByEmail(m.db, m.ucache, info.Email)
	if err == nil {
		_ = createIdentity(m.db, m.ids, u.Id, providerName, info.ID, info.Email)
		return u, false, nil
	}

	u, err = createUser(m.db, m.ids, info.Email, info.Name, "")
	if err != nil {
		return user.User{}, false, err
	}
	_ = createIdentity(m.db, m.ids, u.Id, providerName, info.ID, info.Email)
	return u, true, nil
}

func consumeState(db *orm.DB, state, provider string) error {
	qb := db.Query(&user.OAuthState{}).Where(user.OAuthState_.State).Eq(state)
	results, err := user.ReadAllOAuthState(qb)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return user.ErrInvalidOAuthState
	}
	stateObj := results[0]

	if stateObj.Provider != provider {
		return user.ErrInvalidOAuthState
	}

	// Delete state (single use) - done regardless of expiration to prevent reuse
	if err := db.Delete(stateObj, orm.Eq(user.OAuthState_.State, stateObj.State)); err != nil {
		return err
	}

	if stateObj.ExpiresAt < time.Now()/1e9 {
		return user.ErrInvalidOAuthState
	}

	return nil
}

func (m *Module) PurgeExpiredOAuthStates() error {
	qb := m.db.Query(&user.OAuthState{}).Where(user.OAuthState_.ExpiresAt).Lt(time.Now() / 1e9)
	states, _ := user.ReadAllOAuthState(qb)
	for _, s := range states {
		m.db.Delete(s, orm.Eq(user.OAuthState_.State, s.State))
	}
	return nil
}
