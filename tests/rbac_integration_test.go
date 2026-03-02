//go:build !wasm

package tests

import (
	"testing"

	"github.com/tinywasm/user"
)

type mockHandler struct {
	name  string
	roles map[byte][]byte
}

func (h *mockHandler) HandlerName() string { return h.name }
func (h *mockHandler) AllowedRoles(action byte) []byte {
	return h.roles[action]
}

func TestIntegration_FullFlow(t *testing.T) {
	newTestDB(t)
	user.Init(newTestDB(t), user.Config{})

	if err := user.CreateRole("rid1", "a", "Admin", "Desc"); err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}
	if err := user.CreateRole("rid2", "e", "Editor", "Desc"); err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}

	h := &mockHandler{
		name: "invoice",
		roles: map[byte][]byte{
			'r': {'a', 'e'},
			'w': {'a'},
		},
	}
	if err := user.Register(h); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	user.CreateUser("uid1@test.com", "Test", "")
	u, _ := user.GetUserByEmail("uid1@test.com")

	if err := user.AssignRole(u.ID, "rid2"); err != nil {
		t.Fatalf("AssignRole failed: %v", err)
	}

	ok, err := user.HasPermission(u.ID, "invoice", 'r')
	if err != nil {
		t.Fatalf("HasPermission failed: %v", err)
	}
	if !ok {
		t.Error("Editor should be able to read invoice")
	}

	ok, err = user.HasPermission(u.ID, "invoice", 'w')
	if err != nil {
		t.Fatalf("HasPermission failed: %v", err)
	}
	if ok {
		t.Error("Editor should NOT be able to write invoice")
	}

	if err := user.DeleteRole("rid2"); err != nil {
		t.Fatalf("DeleteRole failed: %v", err)
	}

	ok, err = user.HasPermission(u.ID, "invoice", 'r')
	if err != nil {
		t.Fatalf("HasPermission failed: %v", err)
	}
	if ok {
		t.Error("User should have lost permission after role deletion")
	}
}
