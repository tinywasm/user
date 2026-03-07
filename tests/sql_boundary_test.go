//go:build !wasm

package tests

import (
	"database/sql"
	"testing"

	"github.com/tinywasm/user"
)

func TestSQLBoundary(t *testing.T) {
	db := newTestDB(t)
	m, _ := user.New(db, user.Config{})

	userCRUD := getHandler(m, "users")
	resU, _ := userCRUD.Create(user.User{Email: "sql@example.com", Name: "SQL Test"})
	u := resU.(user.User)
	_ = m.SetPassword(u.ID, "password123")

	t.Run("Login Injection", func(t *testing.T) {
		_, err := m.Login("' OR 1=1 --", "password123")
		if err != user.ErrInvalidCredentials {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("SetPassword Injection", func(t *testing.T) {
		err := m.SetPassword("'; DROP TABLE users; --", "password123")
		// The error from getting identity or updating should not be nil.
		// It will return ErrNotFound or some identity creation error, but NOT crash or drop tables.
		if err == nil {
			t.Errorf("expected error for injected user ID")
		}

		// Ensure table still exists
		_, err = m.GetUser(u.ID)
		if err != nil {
			t.Errorf("expected table to still exist, got %v", err)
		}
	})

	t.Run("RegisterLAN Injection", func(t *testing.T) {
		err := m.RegisterLAN(u.ID, "12345678-5'; --")
		if err != user.ErrInvalidRUT {
			t.Errorf("expected ErrInvalidRUT, got %v", err)
		}
	})

	t.Run("GetUser Injection", func(t *testing.T) {
		_, err := m.GetUser("1 UNION SELECT * FROM user_identities")
		if err != user.ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("GetRoleByCode Injection", func(t *testing.T) {
		_, err := m.GetRoleByCode("admin' OR '1'='1")
		if err != sql.ErrNoRows {
			t.Errorf("expected sql.ErrNoRows, got %v", err)
		}
	})
}
