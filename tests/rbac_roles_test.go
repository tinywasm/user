package tests

import (
	"strings"
	"testing"

	"github.com/tinywasm/rbac"
)

func TestCreateRole(t *testing.T) {
	s, mock := newMockStore(t)

	mock.ExecFn = func(q string, args ...any) error {
		if strings.Contains(q, "INSERT INTO rbac_roles") {
			return nil
		}
		return nil
	}
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				*dest[0].(*string) = "id1"
				*dest[1].(*string) = "a"
				*dest[2].(*string) = "Admin"
				*dest[3].(*string) = "Desc"
				return nil
			},
		}
	}

	err := s.CreateRole("id1", 'a', "Admin", "Desc")
	if err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}

	r, _ := s.GetRole("id1")
	if err != nil {
		t.Fatalf("GetRole failed: %v", err)
	}
	if r == nil || r.Code != 'a' {
		t.Errorf("Role not found in cache or incorrect: %v", r)
	}
}

func TestDeleteRole(t *testing.T) {
	s, mock := newMockStore(t)

	// Pre-populate cache via CreateRole
	mock.ExecFn = func(q string, args ...any) error { return nil }
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				*dest[0].(*string) = "id1"
				*dest[1].(*string) = "a"
				*dest[2].(*string) = "Admin"
				*dest[3].(*string) = "Desc"
				return nil
			},
		}
	}
	s.CreateRole("id1", 'a', "Admin", "Desc")

	// Now DeleteRole
	execCalled := false
	mock.ExecFn = func(q string, args ...any) error {
		if strings.Contains(q, "DELETE FROM rbac_roles") {
			execCalled = true
			return nil
		}
		return nil
	}

	if err := s.DeleteRole("id1"); err != nil {
		t.Fatalf("DeleteRole failed: %v", err)
	}
	if !execCalled {
		t.Error("Exec not called for DeleteRole")
	}

	r, _ := s.GetRole("id1")
	if r != nil {
		t.Errorf("Role still exists in cache after delete: %v", r)
	}
}
