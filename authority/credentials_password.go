package authority

import (
	emailpassword "github.com/tinywasm/user/email_password"

	"github.com/tinywasm/user"
)

// Login verifies email+password directly (no HTTP) — used by
// email_password/credentials tests and any admin flow that needs to verify a
// user's password without going through the mounted route.
func (m *Module) Login(email, password string) (user.User, error) {
	u, err := m.UserByEmail(email)
	if err != nil {
		emailpassword.DummyCompare(password, emailpassword.DefaultHashCost)
		return user.User{}, user.ErrInvalidCredentials
	}
	if u.Status != "active" {
		emailpassword.DummyCompare(password, emailpassword.DefaultHashCost)
		return user.User{}, user.ErrInvalidCredentials
	}
	identity, err := m.IdentityFor(u.Id, "email_password")
	if err != nil {
		emailpassword.DummyCompare(password, emailpassword.DefaultHashCost)
		return user.User{}, user.ErrInvalidCredentials
	}
	if err := emailpassword.VerifyPassword(identity.ProviderId, password); err != nil {
		return user.User{}, err
	}
	return u, nil
}

// SetPassword hashes and stores password as userID's email_password credential.
func (m *Module) SetPassword(userID, password string) error {
	if len(password) < 8 {
		return user.ErrWeakPassword
	}
	if m.config.OnPasswordValidate != nil {
		if err := m.config.OnPasswordValidate(password); err != nil {
			return err
		}
	}
	hash, err := emailpassword.HashPassword(password, emailpassword.DefaultHashCost)
	if err != nil {
		return err
	}
	return m.UpsertIdentity(userID, "email_password", hash, "")
}

// VerifyPassword checks password against userID's stored email_password hash.
func (m *Module) VerifyPassword(userID, password string) error {
	identity, err := m.IdentityFor(userID, "email_password")
	if err != nil {
		return user.ErrInvalidCredentials
	}
	return emailpassword.VerifyPassword(identity.ProviderId, password)
}
