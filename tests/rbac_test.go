//go:build !wasm

package tests

import (
	"testing"

	"github.com/tinywasm/context"
	"github.com/tinywasm/json"
	"github.com/tinywasm/mcp"
	"github.com/tinywasm/user"
	"github.com/tinywasm/user/authority"
	"github.com/tinywasm/model"
)

func TestRBAC_ClosedByDefault(t *testing.T) {
	db := newTestDB(t)
	m, err := authority.New(db, user.Config{})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Anonymous has no permissions", func(t *testing.T) {
		if m.Can("", "any", model.Read) {
			t.Error("anonymous user should have no permissions")
		}
	})

	t.Run("User without roles has no permissions", func(t *testing.T) {
		userCRUD := getHandler(m, "users")
		res, err := userCRUD.Create(user.User{Email: "norole@test.com", Name: "No Role"})
		if err != nil {
			t.Fatal(err)
		}
		u := res.(user.User)

		if m.Can(u.Id, "docs", model.Read) {
			t.Error("user without roles should have no permissions")
		}
	})

	t.Run("Permissions work after seeding", func(t *testing.T) {
		userCRUD := getHandler(m, "users")
		res, _ := userCRUD.Create(user.User{Email: "withrole@test.com", Name: "With Role"})
		u := res.(user.User)

		if err := m.CreateRole("r1", "editor", "Editor", ""); err != nil {
			t.Fatal(err)
		}
		if err := m.CreatePermission("p1", "Read", "docs", model.Read); err != nil {
			t.Fatal(err)
		}
		if err := m.AssignPermission("r1", "p1"); err != nil {
			t.Fatal(err)
		}
		if err := m.AssignRole(u.Id, "r1"); err != nil {
			t.Fatal(err)
		}

		if !m.Can(u.Id, "docs", model.Read) {
			t.Error("expected true for assigned permission")
		}
		if m.Can(u.Id, "docs", model.Update) {
			t.Error("expected false for unassigned action")
		}
	})
}

func TestTools_Me(t *testing.T) {
	db := newTestDB(t)
	m, _ := authority.New(db, user.Config{})

	userCRUD := getHandler(m, "users")
	res, err := userCRUD.Create(user.User{Email: "me@test.com", Name: "Me User"})
	if err != nil {
		t.Fatal(err)
	}
	u := res.(user.User)

	t.Run("Authenticated me returns profile", func(t *testing.T) {
		tools := m.Tools()
		var meTool *mcp.Tool
		for i := range tools {
			if tools[i].Name == "me" {
				meTool = &tools[i]
				break
			}
		}
		if meTool == nil {
			t.Fatal("me tool not found")
		}

		ctx := context.Background()
		if err := ctx.Set(mcp.CtxKeyUserID, u.Id); err != nil {
			t.Fatal(err)
		}

		res, err := meTool.Execute(ctx, mcp.Request{})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if res.IsError {
			t.Fatalf("Execute returned error: %s", res.Content)
		}

		var profile user.ProfileDTO
		if err := json.Decode([]byte(res.Content), &profile); err != nil {
			t.Fatalf("Decode failed: %v", err)
		}
		if profile.Id != u.Id || profile.Email != u.Email {
			t.Errorf("mismatch: got %v, want %v", profile, u)
		}
	})

	t.Run("Anonymous me returns error", func(t *testing.T) {
		tools := m.Tools()
		meTool := &tools[0] // Assume it's the first one

		ctx := context.Background()
		// No userID set
		_, err := meTool.Execute(ctx, mcp.Request{})
		if err == nil {
			t.Error("expected error for anonymous user")
		}
	})
}

func TestNew_Validation(t *testing.T) {
	db := newTestDB(t)

	t.Run("AuthModeBearer requires JWTSecret", func(t *testing.T) {
		_, err := authority.New(db, user.Config{AuthMode: user.AuthModeBearer})
		if err == nil {
			t.Error("expected error when JWTSecret is missing for AuthModeBearer")
		}
	})

	t.Run("AuthModeJWT requires JWTSecret", func(t *testing.T) {
		_, err := authority.New(db, user.Config{AuthMode: user.AuthModeJWT})
		if err == nil {
			t.Error("expected error when JWTSecret is missing for AuthModeJWT")
		}
	})

	t.Run("AuthModeCookie does not require JWTSecret", func(t *testing.T) {
		_, err := authority.New(db, user.Config{AuthMode: user.AuthModeCookie})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
