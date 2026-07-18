package local

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/user"

	"golang.org/x/crypto/bcrypt"
)

var PasswordHashCost = bcrypt.DefaultCost
var dummyHashOnce []byte

func getDummyHash(cost int) []byte {
	if len(dummyHashOnce) == 0 {
		dummyHashOnce, _ = bcrypt.GenerateFromPassword([]byte("dummy"), cost)
	}
	return dummyHashOnce
}

type Authenticator struct {
	db  *orm.DB
	ids model.IDGenerator
}

func New(db *orm.DB, ids model.IDGenerator) *Authenticator {
	return &Authenticator{db: db, ids: ids}
}

func (a *Authenticator) Login(email, password string, getUserByEmail func(*orm.DB, string) (user.User, error), getLocalIdentity func(*orm.DB, string) (user.Identity, error), notify func(user.SecurityEvent)) (user.User, error) {
	u, err := getUserByEmail(a.db, email)
	if err != nil {
		bcrypt.CompareHashAndPassword(getDummyHash(PasswordHashCost), []byte(password))
		return user.User{}, user.ErrInvalidCredentials
	}
	if u.Status != "active" {
		notify(user.SecurityEvent{Type: user.EventNonActiveAccess, UserID: u.Id})
		bcrypt.CompareHashAndPassword(getDummyHash(PasswordHashCost), []byte(password))
		return user.User{}, user.ErrInvalidCredentials
	}

	identity, err := getLocalIdentity(a.db, u.Id)
	if err != nil {
		bcrypt.CompareHashAndPassword(getDummyHash(PasswordHashCost), []byte(password))
		return user.User{}, user.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(identity.ProviderId), []byte(password)); err != nil {
		return user.User{}, user.ErrInvalidCredentials
	}
	return u, nil
}

func (a *Authenticator) SetPassword(userID, password string, upsertIdentity func(*orm.DB, model.IDGenerator, string, string, string, string) error) error {
	if len(password) < 8 {
		return user.ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), PasswordHashCost)
	if err != nil {
		return err
	}
	return upsertIdentity(a.db, a.ids, userID, "local", string(hash), "")
}

func (a *Authenticator) VerifyPassword(userID, password string, getLocalIdentity func(*orm.DB, string) (user.Identity, error)) error {
	identity, err := getLocalIdentity(a.db, userID)
	if err != nil {
		return user.ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(identity.ProviderId), []byte(password)); err != nil {
		return user.ErrInvalidCredentials
	}
	return nil
}
