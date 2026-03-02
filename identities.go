//go:build !wasm

package user

import (
	"time"

	"github.com/tinywasm/unixid"
)

func CreateIdentity(userID, provider, providerID, email string) error {
	u, err := unixid.NewUnixID()
	if err != nil {
		return err
	}

	id := u.GetNewID()
	now := time.Now().Unix()

	i := &Identity{
		ID:         id,
		UserID:     userID,
		Provider:   provider,
		ProviderID: providerID,
		Email:      email,
		CreatedAt:  now,
	}

	if err := store.db.Create(i); err != nil {
		if isUniqueViolation(err) {
			return err
		}
		return err
	}
	return nil
}

func GetIdentityByProvider(provider, providerID string) (Identity, error) {
	qb := store.db.Query(&Identity{}).
		Where(IdentityMeta.Provider).Eq(provider).
		Where(IdentityMeta.ProviderID).Eq(providerID)

	results, err := ReadAllIdentity(qb)
	if err != nil {
		return Identity{}, err
	}
	if len(results) == 0 {
		return Identity{}, ErrNotFound
	}
	return *results[0], nil
}

func getIdentityByUserAndProvider(userID, provider string) (Identity, error) {
	qb := store.db.Query(&Identity{}).
		Where(IdentityMeta.UserID).Eq(userID).
		Where(IdentityMeta.Provider).Eq(provider)

	results, err := ReadAllIdentity(qb)
	if err != nil {
		return Identity{}, err
	}
	if len(results) == 0 {
		return Identity{}, ErrNotFound
	}
	return *results[0], nil
}

func GetUserIdentities(userID string) ([]Identity, error) {
	qb := store.db.Query(&Identity{}).Where(IdentityMeta.UserID).Eq(userID)
	results, err := ReadAllIdentity(qb)
	if err != nil {
		return nil, err
	}

	var identities []Identity
	for _, i := range results {
		identities = append(identities, *i)
	}
	return identities, nil
}

func upsertIdentity(userID, provider, providerID, email string) error {
	qb := store.db.Query(&Identity{}).
		Where(IdentityMeta.UserID).Eq(userID).
		Where(IdentityMeta.Provider).Eq(provider)

	results, err := ReadAllIdentity(qb)
	if err == nil && len(results) > 0 {
		i := results[0]
		i.ProviderID = providerID
		i.Email = email
		return store.db.Update(i)
	} else if len(results) == 0 {
		return CreateIdentity(userID, provider, providerID, email)
	} else {
		return err
	}
}

func UnlinkIdentity(userID, provider string) error {
	identities, err := GetUserIdentities(userID)
	if err != nil {
		return err
	}

	found := false
	for _, id := range identities {
		if id.Provider == provider {
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}

	if len(identities) <= 1 {
		return ErrCannotUnlink
	}

	qb := store.db.Query(&Identity{}).Where(IdentityMeta.UserID).Eq(userID).Where(IdentityMeta.Provider).Eq(provider)
	results, err := ReadAllIdentity(qb)
	if err != nil {
		return err
	}
	if len(results) > 0 {
		return store.db.Delete(results[0])
	}
	return ErrNotFound
}
