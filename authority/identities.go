package authority

import (
	"github.com/tinywasm/time"

	"github.com/tinywasm/orm"
	"github.com/tinywasm/unixid"
	"github.com/tinywasm/user"
)

func createIdentity(db *orm.DB, userID, provider, providerID, email string) error {
	// Verify user exists to enforce relationship before insert
	qb := db.Query(&user.User{}).Where(user.User_.Id).Eq(userID)
	users, errRead := user.ReadAllUser(qb)
	if errRead != nil || len(users) == 0 {
		return user.ErrNotFound
	}

	u, err := unixid.NewUnixID()
	if err != nil {
		return err
	}

	id := u.GetNewID()
	now := time.Now() / 1e9

	i := &user.Identity{
		Id:         id,
		UserId:     userID,
		Provider:   provider,
		ProviderId: providerID,
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

func getIdentityByProvider(db *orm.DB, provider, providerID string) (user.Identity, error) {
	qb := db.Query(&user.Identity{}).
		Where(user.Identity_.Provider).Eq(provider).
		Where(user.Identity_.ProviderId).Eq(providerID)

	results, err := user.ReadAllIdentity(qb)
	if err != nil {
		return user.Identity{}, err
	}
	if len(results) == 0 {
		return user.Identity{}, user.ErrNotFound
	}
	return *results[0], nil
}

func getIdentityByUserAndProvider(db *orm.DB, userID, provider string) (user.Identity, error) {
	qb := db.Query(&user.Identity{}).
		Where(user.Identity_.UserId).Eq(userID).
		Where(user.Identity_.Provider).Eq(provider)

	results, err := user.ReadAllIdentity(qb)
	if err != nil {
		return user.Identity{}, err
	}
	if len(results) == 0 {
		return user.Identity{}, user.ErrNotFound
	}
	return *results[0], nil
}

func (m *Module) GetUserIdentities(userID string) ([]user.Identity, error) {
	qb := m.db.Query(&user.Identity{}).Where(user.Identity_.UserId).Eq(userID)
	results, err := user.ReadAllIdentity(qb)
	if err != nil {
		return nil, err
	}

	var identities []user.Identity
	for _, i := range results {
		identities = append(identities, *i)
	}
	return identities, nil
}

func upsertIdentity(db *orm.DB, userID, provider, providerID, email string) error {
	qb := db.Query(&user.Identity{}).
		Where(user.Identity_.UserId).Eq(userID).
		Where(user.Identity_.Provider).Eq(provider)

	results, err := user.ReadAllIdentity(qb)
	if err == nil && len(results) > 0 {
		i := results[0]
		i.ProviderId = providerID
		i.Email = email
		return db.Update(i, orm.Eq(user.Identity_.Id, i.Id))
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
		return user.ErrNotFound
	}

	if len(identities) <= 1 {
		return user.ErrCannotUnlink
	}

	qb := m.db.Query(&user.Identity{}).Where(user.Identity_.UserId).Eq(userID).Where(user.Identity_.Provider).Eq(provider)
	results, err := user.ReadAllIdentity(qb)
	if err != nil {
		return err
	}
	if len(results) > 0 {
		return m.db.Delete(results[0], orm.Eq(user.Identity_.Id, results[0].Id))
	}
	return user.ErrNotFound
}
