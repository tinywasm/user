package authority

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/user"
	"github.com/tinywasm/user/local"
)

// PasswordHashCost delegates to local package
var PasswordHashCost = local.PasswordHashCost

func (m *Module) Login(email, password string) (user.User, error) {
	getUserByEmailFn := func(db *orm.DB, email string) (user.User, error) {
		return getUserByEmail(db, m.ucache, email)
	}
	getLocalIdentityFn := func(db *orm.DB, userID string) (user.Identity, error) {
		return getLocalIdentity(db, userID)
	}
	notifyFn := func(e user.SecurityEvent) {
		m.notify(e)
	}

	auth := local.New(m.db, m.ids)
	return auth.Login(email, password, getUserByEmailFn, getLocalIdentityFn, notifyFn)
}

func getLocalIdentity(db *orm.DB, userID string) (user.Identity, error) {
	return getIdentityByUserAndProvider(db, userID, "local")
}

func (m *Module) SetPassword(userID, password string) error {
	if len(password) < 8 {
		return user.ErrWeakPassword
	}
	if m.config.OnPasswordValidate != nil {
		if err := m.config.OnPasswordValidate(password); err != nil {
			return err
		}
	}
	upsertIdentityFn := func(db *orm.DB, ids model.IDGenerator, uID, provider, providerID, email string) error {
		return upsertIdentity(db, m.ids, uID, provider, providerID, email)
	}

	auth := local.New(m.db, m.ids)
	return auth.SetPassword(userID, password, upsertIdentityFn)
}

func (m *Module) VerifyPassword(userID, password string) error {
	getLocalIdentityFn := func(db *orm.DB, userID string) (user.Identity, error) {
		return getLocalIdentity(db, userID)
	}

	auth := local.New(m.db, m.ids)
	return auth.VerifyPassword(userID, password, getLocalIdentityFn)
}
