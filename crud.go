//go:build !wasm

package user

import (
	"database/sql"
	"strings"
	"time"

	"github.com/tinywasm/unixid"
)

// nullableStr converts "" to nil so SQLite stores NULL instead of an empty string.
func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func CreateUser(email, name, phone string) (User, error) {
	u, err := unixid.NewUnixID()
	if err != nil {
		return User{}, err
	}

	id := u.GetNewID()
	now := time.Now().Unix()

	if err := store.exec.Exec(
		`INSERT INTO users (id, email, name, phone, created_at)
         VALUES (?, ?, ?, ?, ?)`,
		id, nullableStr(email), name, phone, now,
	); err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrEmailTaken
		}
		return User{}, err
	}
	return User{ID: id, Email: email, Name: name, Phone: phone, Status: "active", CreatedAt: now}, nil
}

func GetUser(id string) (User, error) {
	var u User
	err := store.exec.QueryRow("SELECT id, COALESCE(email, ''), name, COALESCE(phone, ''), status, created_at FROM users WHERE id = ?", id).Scan(
		&u.ID, &u.Email, &u.Name, &u.Phone, &u.Status, &u.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return u, nil
}

func GetUserByEmail(email string) (User, error) {
	var u User
	err := store.exec.QueryRow("SELECT id, COALESCE(email, ''), name, COALESCE(phone, ''), status, created_at FROM users WHERE email = ?", email).Scan(
		&u.ID, &u.Email, &u.Name, &u.Phone, &u.Status, &u.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return u, nil
}

func UpdateUser(id, name, phone string) error {
	return store.exec.Exec("UPDATE users SET name = ?, phone = ? WHERE id = ?", name, phone, id)
}

func SuspendUser(id string) error {
	return store.exec.Exec("UPDATE users SET status = 'suspended' WHERE id = ?", id)
}

func ReactivateUser(id string) error {
	return store.exec.Exec("UPDATE users SET status = 'active' WHERE id = ?", id)
}

func isUniqueViolation(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed") ||
		strings.Contains(err.Error(), "constraint: unique") ||
		strings.Contains(err.Error(), "duplicate key")
}
