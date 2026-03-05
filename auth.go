//go:build !wasm

package user

import (
	"github.com/tinywasm/orm"

	"golang.org/x/crypto/bcrypt"
)

var PasswordHashCost = bcrypt.DefaultCost

func (m *Module) Login(email, password string) (User, error) {
	u, err := getUserByEmail(m.db, m.ucache, email)
	if err != nil {
		return User{}, ErrInvalidCredentials
	}
	if u.Status == "suspended" {
		return User{}, ErrSuspended
	}

	identity, err := getLocalIdentity(m.db, u.ID)
	if err != nil {
		return User{}, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(identity.ProviderID), []byte(password)); err != nil {
		return User{}, ErrInvalidCredentials
	}
	return u, nil
}

func getLocalIdentity(db *orm.DB, userID string) (Identity, error) {
	return getIdentityByUserAndProvider(db, userID, "local")
}

func (m *Module) SetPassword(userID, password string) error {
	if len(password) < 8 {
		return ErrWeakPassword
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
		return ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(identity.ProviderID), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}
