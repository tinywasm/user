//go:build !wasm

package user

import (
	"golang.org/x/crypto/bcrypt"
)

var PasswordHashCost = bcrypt.DefaultCost

func Login(email, password string) (User, error) {
	u, err := GetUserByEmail(email)
	if err != nil {
		return User{}, ErrInvalidCredentials
	}
	if u.Status == "suspended" {
		return User{}, ErrSuspended
	}

	identity, err := getLocalIdentity(u.ID)
	if err != nil {
		return User{}, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(identity.ProviderID), []byte(password)); err != nil {
		return User{}, ErrInvalidCredentials
	}
	return u, nil
}

func getLocalIdentity(userID string) (Identity, error) {
	return getIdentityByUserAndProvider(userID, "local")
}

func SetPassword(userID, password string) error {
	if len(password) < 8 {
		return ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), PasswordHashCost)
	if err != nil {
		return err
	}
	return upsertIdentity(userID, "local", string(hash), "")
}

func VerifyPassword(userID, password string) error {
	identity, err := getLocalIdentity(userID)
	if err != nil {
		return ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(identity.ProviderID), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}
