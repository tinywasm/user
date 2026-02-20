package user

import (
	"database/sql"
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

	if err := store.exec.Exec(
		`INSERT INTO user_identities (id, user_id, provider, provider_id, email, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, userID, provider, providerID, nullableStr(email), now,
	); err != nil {
		if isUniqueViolation(err) {
			return err
		}
		return err
	}
	return nil
}

func GetIdentityByProvider(provider, providerID string) (Identity, error) {
	var i Identity
	err := store.exec.QueryRow(
		"SELECT id, user_id, provider, provider_id, COALESCE(email, ''), created_at FROM user_identities WHERE provider = ? AND provider_id = ?",
		provider, providerID,
	).Scan(&i.ID, &i.UserID, &i.Provider, &i.ProviderID, &i.Email, &i.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return Identity{}, ErrNotFound
		}
		return Identity{}, err
	}
	return i, nil
}

func getIdentityByUserAndProvider(userID, provider string) (Identity, error) {
	var i Identity
	err := store.exec.QueryRow(
		"SELECT id, user_id, provider, provider_id, COALESCE(email, ''), created_at FROM user_identities WHERE user_id = ? AND provider = ?",
		userID, provider,
	).Scan(&i.ID, &i.UserID, &i.Provider, &i.ProviderID, &i.Email, &i.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return Identity{}, ErrNotFound
		}
		return Identity{}, err
	}
	return i, nil
}

func GetUserIdentities(userID string) ([]Identity, error) {
	rows, err := store.exec.Query(
		"SELECT id, user_id, provider, provider_id, COALESCE(email, ''), created_at FROM user_identities WHERE user_id = ?",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var identities []Identity
	for rows.Next() {
		var i Identity
		if err := rows.Scan(&i.ID, &i.UserID, &i.Provider, &i.ProviderID, &i.Email, &i.CreatedAt); err != nil {
			return nil, err
		}
		identities = append(identities, i)
	}
	return identities, rows.Err()
}

func upsertIdentity(userID, provider, providerID, email string) error {
	_, err := getIdentityByUserAndProvider(userID, provider)
	if err == nil {
		return store.exec.Exec("UPDATE user_identities SET provider_id = ?, email = ? WHERE user_id = ? AND provider = ?", providerID, nullableStr(email), userID, provider)
	} else if err == ErrNotFound {
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

	return store.exec.Exec("DELETE FROM user_identities WHERE user_id = ? AND provider = ?", userID, provider)
}
