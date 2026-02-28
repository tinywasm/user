package tests

import (
	"testing"

	"github.com/tinywasm/rbac"
)

func TestGlobalWrappers(t *testing.T) {
	mock := &MockExecutor{
		ExecFn: func(q string, args ...any) error { return nil },
		QueryFn: func(q string, args ...any) (rbac.Rows, error) {
			return &MockRows{}, nil
		},
		QueryRowFn: func(q string, args ...any) rbac.Scanner {
			return &MockScanner{
				ScanFn: func(dest ...any) error {
					// Dummy scan
					return nil
				},
			}
		},
	}

	// SetLog
	rbac.SetLog(func(msg ...any) {})

	// Init
	if err := rbac.Init(mock); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Call globals
	_ = rbac.CreateRole("id", 'x', "Name", "Desc")
	_, _ = rbac.GetRole("id")
	_, _ = rbac.GetRoleByCode('x')
	_, _ = rbac.ListRoles()
	_ = rbac.DeleteRole("id")

	_ = rbac.CreatePermission("pid", "name", "res", 'x')
	_, _ = rbac.GetPermission("pid")
	_, _ = rbac.ListPermissions()
	_ = rbac.DeletePermission("pid")

	_ = rbac.AssignRole("uid", "rid")
	_ = rbac.RevokeRole("uid", "rid")
	_ = rbac.AssignPermission("rid", "pid")
	_ = rbac.RevokePermission("rid", "pid")

	_, _ = rbac.GetUserRoles("uid")
	_, _ = rbac.GetUserRoleCodes("uid")
	_, _ = rbac.HasPermission("uid", "res", 'x')

	_ = rbac.Register()
}
