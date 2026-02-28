package tests

import (
	"database/sql"
	"testing"

	"github.com/tinywasm/rbac"
)

func TestIntegration_FullFlow(t *testing.T) {
	s := newSQLiteStore(t)

	// 1. Create Roles
	if err := s.CreateRole("rid1", 'a', "Admin", "Desc"); err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}
	if err := s.CreateRole("rid2", 'e', "Editor", "Desc"); err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}

	// 2. Register Handlers
	h := &mockHandler{
		name: "invoice",
		roles: map[byte][]byte{
			'r': {'a', 'e'},
			'w': {'a'},
		},
	}
	if err := s.Register(h); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 3. Assign Roles
	if err := s.AssignRole("uid1", "rid2"); err != nil { // Assign Editor
		t.Fatalf("AssignRole failed: %v", err)
	}

	// 4. Check Permissions
	// Editor can read invoice
	ok, err := s.HasPermission("uid1", "invoice", 'r')
	if err != nil {
		t.Fatalf("HasPermission failed: %v", err)
	}
	if !ok {
		t.Error("Editor should be able to read invoice")
	}

	// Editor cannot write invoice
	ok, err = s.HasPermission("uid1", "invoice", 'w')
	if err != nil {
		t.Fatalf("HasPermission failed: %v", err)
	}
	if ok {
		t.Error("Editor should NOT be able to write invoice")
	}

	// 5. Delete Role (Cascade)
	if err := s.DeleteRole("rid2"); err != nil {
		t.Fatalf("DeleteRole failed: %v", err)
	}

	// 6. Check Permissions
	ok, err = s.HasPermission("uid1", "invoice", 'r')
	if err != nil {
		t.Fatalf("HasPermission failed: %v", err)
	}
	if ok {
		t.Error("User should have lost permission after role deletion")
	}
}

func TestIntegration_Idempotency(t *testing.T) {
	s := newSQLiteStore(t)

	if err := s.CreateRole("rid1", 'a', "Admin", "Desc"); err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}

	h := &mockHandler{
		name: "invoice",
		roles: map[byte][]byte{
			'r': {'a'},
		},
	}

	// First pass
	if err := s.Register(h); err != nil {
		t.Fatalf("Register pass 1 failed: %v", err)
	}

	// Second pass
	if err := s.Register(h); err != nil {
		t.Fatalf("Register pass 2 failed: %v", err)
	}

	// CreateRole again with same code but different ID (should be ignored and use existing)
	if err := s.CreateRole("rid_new", 'a', "Admin", "Desc"); err != nil {
		t.Fatalf("CreateRole pass 2 failed: %v", err)
	}
}

func TestLoadCache_Integration(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}

	adapter := &sqliteAdapter{db: db}
	s1, err := rbac.New(adapter)
	if err != nil {
		t.Fatal(err)
	}

	// Populate using s1
	if err := s1.CreateRole("rid1", 'a', "Admin", "Desc"); err != nil {
		t.Fatal(err)
	}
	if err := s1.CreatePermission("pid1", "p1", "r1", 'x'); err != nil {
		t.Fatal(err)
	}
	if err := s1.AssignRole("uid1", "rid1"); err != nil {
		t.Fatal(err)
	}
	if err := s1.AssignPermission("rid1", "pid1"); err != nil {
		t.Fatal(err)
	}

	// New instance using same DB adapter (or DB)
	s2, err := rbac.New(adapter)
	if err != nil {
		t.Fatal(err)
	}

	// Check s2 cache
	r, err := s2.GetRole("rid1")
	if r == nil {
		t.Error("Role not loaded")
	}

	p, err := s2.GetPermission("pid1")
	if p == nil {
		t.Error("Permission not loaded")
	}

	ok, err := s2.HasPermission("uid1", "r1", 'x')
	if !ok {
		t.Error("Role permission or user role not loaded correctly")
	}
}
