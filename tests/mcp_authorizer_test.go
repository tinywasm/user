//go:build !wasm

package tests

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/tinywasm/user"
)

func TestMCPAuthorizer(t *testing.T) {
	db := newTestDB(t)
	secret := []byte("test-secret-32-bytes-minimum-len")
	m, err := user.New(db, user.Config{
		AuthMode:  user.AuthModeBearer,
		JWTSecret: secret,
	})
	if err != nil {
		t.Fatal(err)
	}

	userCRUD := getHandler(m, "users")
	res, err := userCRUD.Create(user.User{Email: "mcp@test.com", Name: "MCP User"})
	if err != nil {
		t.Fatal(err)
	}
	u := res.(user.User)

	token, err := m.GenerateAPIToken(u.ID, 3600)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("TestValidateSession_Bearer_ValidJWT", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		ctx := m.InjectIdentity(context.Background(), req)
		u2, ok := m.FromContext(ctx)
		if !ok || u2.ID != u.ID {
			t.Errorf("expected user %s, got %v", u.ID, u2)
		}
	})

	t.Run("TestValidateSession_Bearer_InvalidJWT", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer invalid.token")
		ctx := m.InjectIdentity(context.Background(), req)
		_, ok := m.FromContext(ctx)
		if ok {
			t.Error("expected unauthorized for invalid JWT")
		}
	})

	t.Run("TestValidateSession_Bearer_MissingHeader", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		ctx := m.InjectIdentity(context.Background(), req)
		_, ok := m.FromContext(ctx)
		if ok {
			t.Error("expected unauthorized for missing header")
		}
	})

	t.Run("TestCanExecute", func(t *testing.T) {
		err := m.CreateRole("r1", "admin", "Admin", "")
		if err != nil {
			t.Fatal(err)
		}

		err = m.CreatePermission("p1", "Read Data", "data", "R")
		if err != nil {
			t.Fatal(err)
		}

		if err := m.AssignPermission("r1", "p1"); err != nil {
			t.Fatal(err)
		}
		if err := m.AssignRole(u.ID, "r1"); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		ctx := m.InjectIdentity(context.Background(), req)

		if !m.CanExecute(ctx, "data", 'R') {
			t.Error("expected CanExecute to return true")
		}
		if m.CanExecute(ctx, "data", 'W') {
			t.Error("expected CanExecute to return false for wrong action")
		}
		if m.CanExecute(ctx, "other", 'R') {
			t.Error("expected CanExecute to return false for wrong resource")
		}
	})

	t.Run("TestGenerateAPIToken_ZeroTTL", func(t *testing.T) {
		token, err := m.GenerateAPIToken(u.ID, 0)
		if err != nil {
			t.Fatal(err)
		}
		// Should be valid
		userID, err := user.ValidateJWT(secret, token)
		if err != nil || userID != u.ID {
			t.Errorf("token invalid: %v", err)
		}
	})

	t.Run("TestGenerateAPIToken_NoSecret", func(t *testing.T) {
		m2, _ := user.New(db, user.Config{})
		_, err := m2.GenerateAPIToken(u.ID, 0)
		if err == nil {
			t.Error("expected error when JWTSecret is missing")
		}
	})
}
