package tests

import (
	"errors"
	"strings"
	"testing"

	"github.com/tinywasm/rbac"
)

func TestMutations_ExecError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExecFn = func(q string, args ...any) error {
		return errors.New("exec error")
	}

	if err := s.CreateRole("id", 'x', "N", "D"); err == nil {
		t.Error("CreateRole expected error")
	}
	if err := s.DeleteRole("id"); err == nil {
		t.Error("DeleteRole expected error")
	}
	if err := s.CreatePermission("id", "n", "r", 'x'); err == nil {
		t.Error("CreatePermission expected error")
	}
	if err := s.DeletePermission("id"); err == nil {
		t.Error("DeletePermission expected error")
	}
	if err := s.AssignRole("u", "r"); err == nil {
		t.Error("AssignRole expected error")
	}
	if err := s.RevokeRole("u", "r"); err == nil {
		t.Error("RevokeRole expected error")
	}
	if err := s.AssignPermission("r", "p"); err == nil {
		t.Error("AssignPermission expected error")
	}
	if err := s.RevokePermission("r", "p"); err == nil {
		t.Error("RevokePermission expected error")
	}
}

func TestCreateRole_FetchError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExecFn = func(q string, args ...any) error { return nil }
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				return errors.New("scan error")
			},
		}
	}

	if err := s.CreateRole("id", 'x', "N", "D"); err == nil {
		t.Error("CreateRole fetch expected error")
	}
}

func TestCheck_EmptyUser(t *testing.T) {
	s, _ := newMockStore(t)
	if _, err := s.GetUserRoles(""); err == nil {
		t.Error("GetUserRoles empty user expected error")
	}
	if _, err := s.GetUserRoleCodes(""); err == nil {
		t.Error("GetUserRoleCodes empty user expected error")
	}
	if _, err := s.HasPermission("", "r", 'x'); err == nil {
		t.Error("HasPermission empty user expected error")
	}
}

func TestRegister_Errors(t *testing.T) {
	s, mock := newMockStore(t)

	// Create Role 'a' first (success)
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
	if err := s.CreateRole("rid1", 'a', "Admin", "Desc"); err != nil {
		t.Fatal(err)
	}

	h := &mockHandler{name: "res", roles: map[byte][]byte{'r': {'a'}}}

	// Case 1: CreatePermission error
	mock.ExecFn = func(q string, args ...any) error {
		if strings.Contains(q, "INSERT INTO rbac_permissions") {
			return errors.New("create perm error")
		}
		return nil
	}
	if err := s.Register(h); err == nil {
		t.Error("Register CreatePermission expected error")
	}

	// Case 2: CreatePermission Success, AssignPermission Error
	mock.ExecFn = func(q string, args ...any) error {
		if strings.Contains(q, "INSERT INTO rbac_role_permissions") {
			return errors.New("assign error")
		}
		return nil
	}
	// Need mock for CreatePermission Fetch
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				*dest[0].(*string) = "pid"
				*dest[1].(*string) = "res:r"
				*dest[2].(*string) = "res"
				*dest[3].(*string) = "r"
				return nil
			},
		}
	}

	if err := s.Register(h); err == nil {
		t.Error("Register AssignPermission expected error")
	}
}
