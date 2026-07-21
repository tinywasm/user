package authority

import (
	"github.com/tinywasm/orm"
	"github.com/tinywasm/router"
	"github.com/tinywasm/time"
	"github.com/tinywasm/user"
)

var (
	_ user.IdentityStore    = (*Module)(nil)
	_ user.StateStore       = (*Module)(nil)
	_ user.TrustedIPStore   = (*Module)(nil)
	_ user.SessionRepo      = (*Module)(nil)
	_ user.SecurityNotifier = (*Module)(nil)
	_ user.SessionIssuer    = (*Module)(nil)
)

func (m *Module) UserByID(id string) (user.User, error) { return getUser(m.db, m.ucache, id) }
func (m *Module) UserByEmail(email string) (user.User, error) {
	return getUserByEmail(m.db, m.ucache, email)
}
func (m *Module) CreateUser(email, name, phone string) (user.User, error) {
	return createUser(m.db, m.ids, email, name, phone)
}
func (m *Module) IdentityByProvider(provider, providerID string) (user.Identity, error) {
	return getIdentityByProvider(m.db, provider, providerID)
}
func (m *Module) IdentityFor(userID, provider string) (user.Identity, error) {
	return getIdentityByUserAndProvider(m.db, userID, provider)
}
func (m *Module) UpsertIdentity(userID, provider, providerID, email string) error {
	return upsertIdentity(m.db, m.ids, userID, provider, providerID, email)
}

func (m *Module) CreateState(provider string) (string, error) {
	state := m.ids.NewID()
	now := time.Now() / 1e9
	s := &user.OAuthState{State: state, Provider: provider, ExpiresAt: now + 600, CreatedAt: now}
	if err := m.db.Create(s); err != nil {
		return "", err
	}
	return state, nil
}
func (m *Module) ConsumeState(state, provider string) error {
	return consumeState(m.db, state, provider)
}

// PurgeExpiredOAuthStates is maintenance, not part of any port — call it
// periodically from a cron-like task in the consuming app.
func (m *Module) PurgeExpiredOAuthStates() error {
	qb := m.db.Query(&user.OAuthState{}).Where(user.OAuthState_.ExpiresAt).Lt(time.Now() / 1e9)
	states, _ := user.ReadAllOAuthState(qb)
	for _, s := range states {
		m.db.Delete(s, orm.Eq(user.OAuthState_.State, s.State))
	}
	return nil
}

func (m *Module) IsTrustedIP(userID, ip string) bool { return checkLANIP(m.db, userID, ip) == nil }

func (m *Module) Notify(e user.SecurityEvent) { m.notify(e) }

func (m *Module) IssueSession(ctx router.Context, userID string) error {
	return m.strategy.Issue(ctx, userID)
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
	if err := db.Delete(stateObj, orm.Eq(user.OAuthState_.State, stateObj.State)); err != nil {
		return err
	}
	if stateObj.ExpiresAt < time.Now()/1e9 {
		return user.ErrInvalidOAuthState
	}
	return nil
}
