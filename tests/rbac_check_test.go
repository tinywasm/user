package tests

import (
	"testing"

	"github.com/tinywasm/rbac"
)

func TestHasPermission(t *testing.T) {
	s, mock := newMockStore(t)

	// 1. Create Role
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

	// 2. Create Permission
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				*dest[0].(*string) = "pid1"
				*dest[1].(*string) = "view_invoice"
				*dest[2].(*string) = "invoice"
				*dest[3].(*string) = "r"
				return nil
			},
		}
	}
	s.CreatePermission("pid1", "view_invoice", "invoice", 'r')

	// 3. Assign Permission to Role
	mock.ExecFn = func(q string, args ...any) error { return nil }
	s.AssignPermission("rid1", "pid1")

	// 4. Assign Role to User
	s.AssignRole("uid1", "rid1")

	// 5. Check
	ok, err := s.HasPermission("uid1", "invoice", 'r')
	if err != nil {
		t.Fatalf("HasPermission failed: %v", err)
	}
	if !ok {
		t.Error("HasPermission should be true")
	}

	ok, err = s.HasPermission("uid1", "invoice", 'w')
	if ok {
		t.Error("HasPermission should be false for 'w'")
	}
}
