//go:build !wasm

package tests

import (
	"testing"

	"github.com/tinywasm/router"
	"github.com/tinywasm/router/mock"
	"github.com/tinywasm/user"
	"github.com/tinywasm/user/server"
	"github.com/tinywasm/model"
)

func TestMCPAuthorizer(t *testing.T) {
	db := newTestDB(t)
	secret := []byte("test-secret-32-bytes-minimum-len")
	m, err := userserver.New(db, user.Config{
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

	token, err := m.GenerateAPIToken(u.Id, 3600)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("TestAuthenticate_Bearer_ValidJWT", func(t *testing.T) {
		ctx := &mock.Context{}
		ctx.SetHeader("Authorization", "Bearer "+token)

		m.Authenticate()(func(c router.Context) {
			if c.UserID() != u.Id {
				t.Errorf("expected user %s, got %s", u.Id, c.UserID())
			}
		})(ctx)
	})

	t.Run("TestAuthenticate_Bearer_InvalidJWT", func(t *testing.T) {
		ctx := &mock.Context{}
		ctx.SetHeader("Authorization", "Bearer invalid.token")
		m.Authenticate()(func(c router.Context) {
			if c.UserID() != "" {
				t.Error("expected anonymous for invalid JWT")
			}
		})(ctx)
	})

	t.Run("TestAuthenticate_Bearer_MissingHeader", func(t *testing.T) {
		ctx := &mock.Context{}
		m.Authenticate()(func(c router.Context) {
			if c.UserID() != "" {
				t.Error("expected anonymous for missing header")
			}
		})(ctx)
	})

	t.Run("TestCan", func(t *testing.T) {
		err := m.CreateRole("r1", "admin", "Admin", "")
		if err != nil {
			t.Fatal(err)
		}

		err = m.CreatePermission("p1", "Read Data", "data", model.Read)
		if err != nil {
			t.Fatal(err)
		}

		if err := m.AssignPermission("r1", "p1"); err != nil {
			t.Fatal(err)
		}
		if err := m.AssignRole(u.Id, "r1"); err != nil {
			t.Fatal(err)
		}

		if !m.Can(u.Id, "data", model.Read) {
			t.Error("expected Can to return true")
		}
		if m.Can(u.Id, "data", model.Update) {
			t.Error("expected Can to return false for wrong action")
		}
		if m.Can(u.Id, "other", model.Read) {
			t.Error("expected Can to return false for wrong resource")
		}
	})

	t.Run("TestGenerateAPIToken_ZeroTTL", func(t *testing.T) {
		token, err := m.GenerateAPIToken(u.Id, 0)
		if err != nil {
			t.Fatal(err)
		}
		// Should be valid
		userID, err := userserver.ValidateJWT(secret, token)
		if err != nil || userID != u.Id {
			t.Errorf("token invalid: %v", err)
		}
	})

	t.Run("TestGenerateAPIToken_NoSecret", func(t *testing.T) {
		m2, _ := userserver.New(db, user.Config{})
		_, err := m2.GenerateAPIToken(u.Id, 0)
		if err == nil {
			t.Error("expected error when JWTSecret is missing")
		}
	})
}
