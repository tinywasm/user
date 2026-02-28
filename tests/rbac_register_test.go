package tests

import (
	"strings"
	"testing"

	"github.com/tinywasm/rbac"
)

type mockHandler struct {
	name  string
	roles map[byte][]byte
}

func (h *mockHandler) HandlerName() string { return h.name }
func (h *mockHandler) AllowedRoles(action byte) []byte {
	return h.roles[action]
}

func TestRegister_Seeds(t *testing.T) {
	s, mock := newMockStore(t)

	// 1. Create Role 'a'
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

	// 2. Register Handler
	h := &mockHandler{
		name: "invoice",
		roles: map[byte][]byte{
			'r': {'a'},
		},
	}

	// Mock sequence:
	// Register calls CreatePermission -> Exec INSERT
	// Register calls CreatePermission -> Fetch (QueryRow)
	// Register calls GetRoleByCode -> Cache (no SQL)
	// Register calls AssignPermission -> Exec INSERT
	// Register calls AssignPermission -> Update Cache (needs perm from cache)

	// We need to carefully mock QueryRowFn for CreatePermission.
	// It happens inside Register.

	// Also Register loops over c, r, u, d.
	// Only 'r' has roles.
	// So 1 CreatePermission call for 'r'.

	permsCreated := 0
	assignmentsCreated := 0

	mock.ExecFn = func(q string, args ...any) error {
		if strings.Contains(q, "INSERT INTO rbac_permissions") {
			permsCreated++
			return nil
		}
		if strings.Contains(q, "INSERT INTO rbac_role_permissions") {
			assignmentsCreated++
			return nil
		}
		return nil
	}

	mock.QueryRowFn = func(q string, args ...any) rbac.Scanner {
		return &MockScanner{
			ScanFn: func(dest ...any) error {
				// This is for CreatePermission fetch
				*dest[0].(*string) = "pid1"
				*dest[1].(*string) = "invoice:r"
				*dest[2].(*string) = "invoice"
				*dest[3].(*string) = "r"
				return nil
			},
		}
	}

	if err := s.Register(h); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if permsCreated != 1 {
		t.Errorf("Expected 1 permission created, got %d", permsCreated)
	}
	if assignmentsCreated != 1 {
		t.Errorf("Expected 1 assignment created, got %d", assignmentsCreated)
	}
}
