//go:build !wasm

package user

import (
	"time"

	"github.com/tinywasm/orm"
	"github.com/tinywasm/unixid"
)

func createIdentity(db *orm.DB, userID, provider, providerID, email string) error {
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

	if err := db.Create(i); err != nil {
		if isUniqueViolation(err) {
			return err
		}
		return err
	}
	return nil
}

func getIdentityByProvider(db *orm.DB, provider, providerID string) (Identity, error) {
	qb := db.Query(&Identity{}).
		Where(Identity_.Provider).Eq(provider).
		Where(Identity_.ProviderID).Eq(providerID)

	results, err := ReadAllIdentity(qb)
	if err != nil {
		return Identity{}, err
	}
	if len(results) == 0 {
		return Identity{}, ErrNotFound
	}
	return *results[0], nil
}

func getIdentityByUserAndProvider(db *orm.DB, userID, provider string) (Identity, error) {
	qb := db.Query(&Identity{}).
		Where(Identity_.UserID).Eq(userID).
		Where(Identity_.Provider).Eq(provider)

	results, err := ReadAllIdentity(qb)
	if err != nil {
		return Identity{}, err
	}
	if len(results) == 0 {
		return Identity{}, ErrNotFound
	}
	return *results[0], nil
}

func (m *Module) GetUserIdentities(userID string) ([]Identity, error) {
	qb := m.db.Query(&Identity{}).Where(Identity_.UserID).Eq(userID)
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

func upsertIdentity(db *orm.DB, userID, provider, providerID, email string) error {
	qb := db.Query(&Identity{}).
		Where(Identity_.UserID).Eq(userID).
		Where(Identity_.Provider).Eq(provider)

	results, err := ReadAllIdentity(qb)
	if err == nil && len(results) > 0 {
		i := results[0]
		i.ProviderID = providerID
		i.Email = email
		return db.Update(i, orm.Eq(Identity_.ID, i.ID))
	} else if len(results) == 0 {
		return createIdentity(db, userID, provider, providerID, email)
	} else {
		return err
	}
}

func (m *Module) UnlinkIdentity(userID, provider string) error {
	identities, err := m.GetUserIdentities(userID)
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

	qb := m.db.Query(&Identity{}).Where(Identity_.UserID).Eq(userID).Where(Identity_.Provider).Eq(provider)
	results, err := ReadAllIdentity(qb)
	if err != nil {
		return err
	}
	if len(results) > 0 {
		return m.db.Delete(results[0], orm.Eq(Identity_.ID, results[0].ID))
	}
	return ErrNotFound
}
