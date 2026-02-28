package tests

import (
	"errors"
	"testing"

	"github.com/tinywasm/rbac"
)

func TestInstanceSetLog(t *testing.T) {
	s, _ := newMockStore(t)
	s.SetLog(func(msg ...any) {})
}

func TestDeleteBranches(t *testing.T) {
	// Test deleteRole when role does not exist
	s, mock := newMockStore(t)
	mock.ExecFn = func(q string, args ...any) error { return nil }

	if err := s.DeleteRole("nonexistent"); err != nil {
		t.Fatal(err)
	}

	// Test deletePerm when perm does not exist
	if err := s.DeletePermission("nonexistent"); err != nil {
		t.Fatal(err)
	}
}

func TestDeduplication(t *testing.T) {
	s, mock := newMockStore(t)

	// Create Role
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				*dest[0].(*string) = "rid"
				*dest[1].(*string) = "a"
				*dest[2].(*string) = "Admin"
				*dest[3].(*string) = "Desc"
				return nil
			},
		}
	}
	s.CreateRole("rid", 'a', "Admin", "Desc")

	// AssignRole twice
	mock.ExecFn = func(q string, args ...any) error { return nil }
	s.AssignRole("uid", "rid")
	s.AssignRole("uid", "rid")

	roles, _ := s.GetUserRoles("uid")
	if len(roles) != 1 {
		t.Errorf("Expected 1 role, got %d", len(roles))
	}

	// Create Permission
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				*dest[0].(*string) = "pid"
				*dest[1].(*string) = "p"
				*dest[2].(*string) = "r"
				*dest[3].(*string) = "x"
				return nil
			},
		}
	}
	s.CreatePermission("pid", "p", "r", 'x')

	// AssignPermission twice
	s.AssignPermission("rid", "pid")
	s.AssignPermission("rid", "pid")
}

func TestDeletePerm_Cascade(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExecFn = func(q string, args ...any) error { return nil }

	// Create Role
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				*dest[0].(*string) = "rid"
				*dest[1].(*string) = "a"
				*dest[2].(*string) = "Admin"
				*dest[3].(*string) = "Desc"
				return nil
			},
		}
	}
	s.CreateRole("rid", 'a', "Admin", "Desc")

	// Create Perm
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				*dest[0].(*string) = "pid"
				*dest[1].(*string) = "p"
				*dest[2].(*string) = "r"
				*dest[3].(*string) = "x"
				return nil
			},
		}
	}
	s.CreatePermission("pid", "p", "r", 'x')

	// Assign
	s.AssignPermission("rid", "pid")

	// Delete Perm
	if err := s.DeletePermission("pid"); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveRolePermission_NotFound(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExecFn = func(q string, args ...any) error { return nil }

	// Revoke non-existent permission assignment
	s.RevokePermission("rid", "pid")
}

func TestCreatePermission_FetchError(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExecFn = func(q string, args ...any) error { return nil }
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				return errors.New("scan error")
			},
		}
	}

	if err := s.CreatePermission("pid", "n", "r", 'x'); err == nil {
		t.Error("CreatePermission fetch expected error")
	}
}

type incompleteHandler struct{}
func (h *incompleteHandler) HandlerName() string { return "inc" }

func TestRegister_Skips(t *testing.T) {
	s, _ := newMockStore(t)
	h := &incompleteHandler{}
	if err := s.Register(h); err != nil {
		t.Fatal(err)
	}
}

type noNameHandler struct{}
func (h *noNameHandler) AllowedRoles(action byte) []byte { return []byte{'a'} }

func TestRegister_SkipsNoName(t *testing.T) {
	s, _ := newMockStore(t)
	h := &noNameHandler{}
	if err := s.Register(h); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveRolePermission_NotFound_NonEmpty(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExecFn = func(q string, args ...any) error { return nil }

	// Create Role
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				*dest[0].(*string) = "rid"
				*dest[1].(*string) = "a"
				*dest[2].(*string) = "Admin"
				*dest[3].(*string) = "Desc"
				return nil
			},
		}
	}
	s.CreateRole("rid", 'a', "Admin", "Desc")

	// Create Perm
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				*dest[0].(*string) = "pid"
				*dest[1].(*string) = "p"
				*dest[2].(*string) = "r"
				*dest[3].(*string) = "x"
				return nil
			},
		}
	}
	s.CreatePermission("pid", "p", "r", 'x')
	s.AssignPermission("rid", "pid")

	// Revoke different perm
	s.RevokePermission("rid", "pid2")
}

type partialHandler struct{}

func (h *partialHandler) HandlerName() string { return "partial" }
func (h *partialHandler) AllowedRoles(action byte) []byte {
	if action == 'r' {
		return []byte{'a'}
	}
	return nil
}

func TestRegister_Partial(t *testing.T) {
	s, mock := newMockStore(t)
	// Create Role 'a'
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				*dest[0].(*string) = "rid"
				*dest[1].(*string) = "a"
				*dest[2].(*string) = "Admin"
				*dest[3].(*string) = "Desc"
				return nil
			},
		}
	}
	s.CreateRole("rid", 'a', "Admin", "Desc")

	h := &partialHandler{}

	// Mock Exec/Query for Register
	mock.ExecFn = func(q string, args ...any) error { return nil }
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				*dest[0].(*string) = "pid"
				*dest[1].(*string) = "partial:r"
				*dest[2].(*string) = "partial"
				*dest[3].(*string) = "r"
				return nil
			},
		}
	}

	if err := s.Register(h); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteRole_KeepUser(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExecFn = func(q string, args ...any) error { return nil }

	// Create Role 1 & 2
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		code := args[0].(string)
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				if code == "a" {
					*dest[0].(*string) = "rid1"
					*dest[1].(*string) = "a"
				} else {
					*dest[0].(*string) = "rid2"
					*dest[1].(*string) = "b"
				}
				*dest[2].(*string) = "Admin"
				*dest[3].(*string) = "Desc"
				return nil
			},
		}
	}
	s.CreateRole("rid1", 'a', "Admin", "Desc")
	s.CreateRole("rid2", 'b', "Editor", "Desc")

	// Assign both
	s.AssignRole("uid", "rid1")
	s.AssignRole("uid", "rid2")

	// Delete Role 1
	if err := s.DeleteRole("rid1"); err != nil {
		t.Fatal(err)
	}

	// User should still have role 2
	roles, _ := s.GetUserRoles("uid")
	if len(roles) != 1 {
		t.Errorf("Expected 1 role, got %d", len(roles))
	}
	if len(roles) > 0 && roles[0].ID != "rid2" {
		t.Errorf("Expected rid2, got %s", roles[0].ID)
	}
}

func TestDeletePerm_KeepRole(t *testing.T) {
	s, mock := newMockStore(t)
	mock.ExecFn = func(q string, args ...any) error { return nil }

	// Create Role
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				*dest[0].(*string) = "rid"
				*dest[1].(*string) = "a"
				*dest[2].(*string) = "Admin"
				*dest[3].(*string) = "Desc"
				return nil
			},
		}
	}
	s.CreateRole("rid", 'a', "Admin", "Desc")

	// Create Perm 1 & 2
	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		action := args[1].(string)
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				if action == "x" {
					*dest[0].(*string) = "pid1"
					*dest[1].(*string) = "p1"
				} else {
					*dest[0].(*string) = "pid2"
					*dest[1].(*string) = "p2"
				}
				*dest[2].(*string) = "r"
				*dest[3].(*string) = action
				return nil
			},
		}
	}
	s.CreatePermission("pid1", "p1", "r", 'x')
	s.CreatePermission("pid2", "p2", "r", 'y')

	// Assign both
	s.AssignPermission("rid", "pid1")
	s.AssignPermission("rid", "pid2")

	// Delete Perm 1
	if err := s.DeletePermission("pid1"); err != nil {
		t.Fatal(err)
	}

	// Role should still have perm 2
	s.AssignRole("uid", "rid")

	ok, _ := s.HasPermission("uid", "r", 'y')
	if !ok {
		t.Error("Should still have p2")
	}
	ok, _ = s.HasPermission("uid", "r", 'x')
	if ok {
		t.Error("Should not have p1")
	}
}
