package userserver

import (
	"github.com/tinywasm/orm"
	"github.com/tinywasm/user"

	"golang.org/x/crypto/bcrypt"
)

var PasswordHashCost = bcrypt.DefaultCost
var dummyHashOnce []byte

func getDummyHash(cost int) []byte {
	// Simple memoization for the specific cost.
	// In practice, this could be a map if cost changes,
	// but for now we follow the existing pattern.
	if len(dummyHashOnce) == 0 {
		dummyHashOnce, _ = bcrypt.GenerateFromPassword([]byte("dummy"), cost)
	}
	return dummyHashOnce
}

func (m *Module) Login(email, password string) (user.User, error) {
	u, err := getUserByEmail(m.db, m.ucache, email)
	if err != nil {
		// Dummy bcrypt: constant-time regardless of user existence.
		bcrypt.CompareHashAndPassword(getDummyHash(PasswordHashCost), []byte(password))
		return user.User{}, user.ErrInvalidCredentials
	}
	if u.Status != "active" {
		m.notify(user.SecurityEvent{Type: user.EventNonActiveAccess, UserID: u.Id})
		bcrypt.CompareHashAndPassword(getDummyHash(PasswordHashCost), []byte(password))
		return user.User{}, user.ErrInvalidCredentials
	}

	identity, err := getLocalIdentity(m.db, u.Id)
	if err != nil {
		bcrypt.CompareHashAndPassword(getDummyHash(PasswordHashCost), []byte(password))
		return user.User{}, user.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(identity.ProviderId), []byte(password)); err != nil {
		return user.User{}, user.ErrInvalidCredentials
	}
	return u, nil
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
	hash, err := bcrypt.GenerateFromPassword([]byte(password), PasswordHashCost)
	if err != nil {
		return err
	}
	return upsertIdentity(m.db, userID, "local", string(hash), "")
}

func (m *Module) VerifyPassword(userID, password string) error {
	identity, err := getLocalIdentity(m.db, userID)
	if err != nil {
		return user.ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(identity.ProviderId), []byte(password)); err != nil {
		return user.ErrInvalidCredentials
	}
	return nil
}
