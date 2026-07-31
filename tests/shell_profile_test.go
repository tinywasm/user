//go:build !wasm

package tests

import (
	"testing"

	"github.com/tinywasm/json"
	"github.com/tinywasm/router/mock"
	"github.com/tinywasm/user"
	"github.com/tinywasm/user/authority"
)

type platformdIdentity interface {
	UserName() string
	UserArea() string
}

func TestShellProfileInterfaceConformance(t *testing.T) {
	var _ platformdIdentity = user.ShellProfile{}
}

func TestProfileDTOShellMapping(t *testing.T) {
	// 1. With Area explicitly set
	dto := user.ProfileDTO{
		Name: "Alice Smith",
		Area: "Engineering",
	}

	shell := dto.Shell()
	if shell.UserName() != "Alice Smith" {
		t.Errorf("expected UserName 'Alice Smith', got '%s'", shell.UserName())
	}
	if shell.UserArea() != "Engineering" {
		t.Errorf("expected UserArea 'Engineering', got '%s'", shell.UserArea())
	}

	// 2. Round-trip serialization check
	var decoded user.ProfileDTO
	// Since ProfileDTO implements model.Fielder, we can verify fields mapping
	// But simply checking struct copy works too
	decoded = dto
	shellDecoded := decoded.Shell()
	if shellDecoded.UserName() != "Alice Smith" || shellDecoded.UserArea() != "Engineering" {
		t.Errorf("roundtrip failed: %+v", shellDecoded)
	}
}

func TestOpMeAreaFallback(t *testing.T) {
	db := newTestDB(t)
	m, _ := authority.New(db, user.Config{IDs: testIDs})

	// 1. Create a user with a role
	userCRUD := getHandler(m, "users")
	res, err := userCRUD.Create(user.User{Email: "fallback@test.com", Name: "Fallback User"})
	if err != nil {
		t.Fatal(err)
	}
	u := res.(user.User)

	// Create a Role
	roleCRUD := getHandler(m, "roles")
	roleRes, err := roleCRUD.Create(user.Role{Id: "role_1", Code: "ops", Name: "Operations", Description: "Ops Team"})
	if err != nil {
		t.Fatal(err)
	}
	_ = roleRes

	// Assign role to user
	err = m.AssignRole(u.Id, "role_1")
	if err != nil {
		t.Fatal(err)
	}

	// 2. Invoke opMe which returns ProfileDTO. The user's Area is empty, so it should fallback to Role name.
	reg := &mockOpRegistry{ops: make(map[string]*mockRoute)}
	m.MountOps(reg)

	route := reg.ops[user.OpMe]
	if route == nil {
		t.Fatal("me op not registered")
	}

	ctx := &mock.Context{}
	ctx.SetUserID(u.Id)

	route.handler(ctx)

	if ctx.Status != 0 && ctx.Status != 200 {
		t.Fatalf("route.handler failed with status %d: %s", ctx.Status, string(ctx.ResponseBody()))
	}

	// Read body and decode ProfileDTO
	var profile user.ProfileDTO
	err = json.Decode(ctx.ResponseBody(), &profile)
	if err != nil {
		t.Fatal(err)
	}

	if profile.Area != "Operations" {
		t.Errorf("expected fallback Area to be 'Operations', got '%s'", profile.Area)
	}

	// 3. Now update the user with an explicit Area
	u.Area = "Engineering Department"
	_, err = userCRUD.Update(u)
	if err != nil {
		t.Fatal(err)
	}

	ctx2 := &mock.Context{}
	ctx2.SetUserID(u.Id)

	route.handler(ctx2)

	var profile2 user.ProfileDTO
	err = json.Decode(ctx2.ResponseBody(), &profile2)
	if err != nil {
		t.Fatal(err)
	}

	if profile2.Area != "Engineering Department" {
		t.Errorf("expected explicit Area 'Engineering Department', got '%s'", profile2.Area)
	}
}
