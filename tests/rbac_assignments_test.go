package tests

import (
	"strings"
	"testing"

	"github.com/tinywasm/rbac"
)

func TestAssignRole(t *testing.T) {
	s, mock := newMockStore(t)

	// Create Role
	mock.ExecFn = func(q string, args ...any) error { return nil }
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				*dest[0].(*string) = "rid1"
				*dest[1].(*string) = "a"
				*dest[2].(*string) = "Admin"
				*dest[3].(*string) = "Desc"
				return nil
			},
		}
	}
	s.CreateRole("rid1", 'a', "Admin", "Desc")

	// Assign Role
	execCalled := false
	mock.ExecFn = func(q string, args ...any) error {
		if strings.Contains(q, "INSERT INTO rbac_user_roles") {
			execCalled = true
			return nil
		}
		return nil
	}

	if err := s.AssignRole("uid1", "rid1"); err != nil {
		t.Fatalf("AssignRole failed: %v", err)
	}
	if !execCalled {
		t.Error("Exec not called for AssignRole")
	}

	roles, err := s.GetUserRoles("uid1")
	if err != nil {
		t.Fatalf("GetUserRoles failed: %v", err)
	}
	if len(roles) != 1 || roles[0].ID != "rid1" {
		t.Errorf("Expected user role rid1, got %v", roles)
	}
}

func TestRevokeRole(t *testing.T) {
	s, mock := newMockStore(t)

	// Populate
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				*dest[0].(*string) = "rid1"
				*dest[1].(*string) = "a"
				*dest[2].(*string) = "Admin"
				*dest[3].(*string) = "Desc"
				return nil
			},
		}
	}
	s.CreateRole("rid1", 'a', "Admin", "Desc")
	s.AssignRole("uid1", "rid1")

	// Revoke
	execCalled := false
	mock.ExecFn = func(q string, args ...any) error {
		if strings.Contains(q, "DELETE FROM rbac_user_roles") {
			execCalled = true
			return nil
		}
		return nil
	}

	if err := s.RevokeRole("uid1", "rid1"); err != nil {
		t.Fatalf("RevokeRole failed: %v", err)
	}
	if !execCalled {
		t.Error("Exec not called for RevokeRole")
	}

	roles, _ := s.GetUserRoles("uid1")
	if len(roles) != 0 {
		t.Errorf("User roles should be empty, got %v", roles)
	}
}
