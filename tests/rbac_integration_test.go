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
	db := newTestDB(t)
	m, err := user.New(db, user.Config{})
	if err != nil {
		t.Fatal(err)
	}

	if err := m.CreateRole("rid1", "a", "Admin", "Desc"); err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}
	if err := m.CreateRole("rid2", "e", "Editor", "Desc"); err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}

	h := &mockHandler{
		name: "invoice",
		roles: map[byte][]byte{
			'r': {'a', 'e'},
			'w': {'a'},
		},
	}
	if err := m.Register(h); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	userCRUD := getHandler(m, "users")
	res, err := userCRUD.Create(user.User{Email: "uid1@test.com", Name: "Test"})
	if err != nil {
		t.Fatal(err)
	}
	u := res.(user.User)

	if err := m.AssignRole(u.ID, "rid2"); err != nil {
		t.Fatalf("AssignRole failed: %v", err)
	}

	ok, err := m.HasPermission(u.ID, "invoice", 'r')
	if err != nil {
		t.Fatalf("HasPermission failed: %v", err)
	}
	if !ok {
		t.Error("Editor should be able to read invoice")
	}

	ok, err = m.HasPermission(u.ID, "invoice", 'w')
	if err != nil {
		t.Fatalf("HasPermission failed: %v", err)
	}
	if ok {
		t.Error("Editor should NOT be able to write invoice")
	}

	if err := m.DeleteRole("rid2"); err != nil {
		t.Fatalf("DeleteRole failed: %v", err)
	}

	ok, err = m.HasPermission(u.ID, "invoice", 'r')
	if err != nil {
		t.Fatalf("HasPermission failed: %v", err)
	}
	if ok {
		t.Error("User should have lost permission after role deletion")
	}
}
