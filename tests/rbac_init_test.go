package tests

import (
	"errors"
	"strings"
	"testing"

	"github.com/tinywasm/rbac"
)

func TestMigrateFailsFirstTable(t *testing.T) {
	mock := &MockExecutor{
		ExecFn: func(q string, args ...any) error {
			if strings.Contains(q, "CREATE TABLE IF NOT EXISTS rbac_roles") {
				return errors.New("ddl error")
			}
			return nil
		},
	}
	_, err := rbac.New(mock)
	if err == nil || err.Error() != "ddl error" {
		t.Errorf("expected ddl error, got %v", err)
	}
}

func TestLoadCacheFails(t *testing.T) {
	// 1. Roles query fails
	mock := &MockExecutor{
		ExecFn: func(q string, args ...any) error { return nil },
		QueryFn: func(q string, args ...any) (rbac.Rows, error) {
			if strings.Contains(q, "SELECT id, code, name, description FROM rbac_roles") {
				return nil, errors.New("query error roles")
			}
			return nil, nil
		},
	}
	if _, err := rbac.New(mock); err == nil || err.Error() != "query error roles" {
		t.Error("expected query error roles")
	}

	// 2. Perms query fails
	mock = &MockExecutor{
		ExecFn: func(q string, args ...any) error { return nil },
		QueryFn: func(q string, args ...any) (rbac.Rows, error) {
			if strings.Contains(q, "SELECT id, name, resource, action FROM rbac_permissions") {
				return nil, errors.New("query error perms")
			}
			return &MockRows{}, nil
		},
	}
	if _, err := rbac.New(mock); err == nil || err.Error() != "query error perms" {
		t.Error("expected query error perms")
	}

	// 3. RolePerms query fails
	mock = &MockExecutor{
		ExecFn: func(q string, args ...any) error { return nil },
		QueryFn: func(q string, args ...any) (rbac.Rows, error) {
			if strings.Contains(q, "FROM rbac_role_permissions") {
				return nil, errors.New("query error role_perms")
			}
			return &MockRows{}, nil
		},
	}
	if _, err := rbac.New(mock); err == nil || err.Error() != "query error role_perms" {
		t.Error("expected query error role_perms")
	}

	// 4. UserRoles query fails
	mock = &MockExecutor{
		ExecFn: func(q string, args ...any) error { return nil },
		QueryFn: func(q string, args ...any) (rbac.Rows, error) {
			if strings.Contains(q, "FROM rbac_user_roles") {
				return nil, errors.New("query error user_roles")
			}
			return &MockRows{}, nil
		},
	}
	if _, err := rbac.New(mock); err == nil || err.Error() != "query error user_roles" {
		t.Error("expected query error user_roles")
	}
}

func TestLoadCache_ScanError(t *testing.T) {
	// Fail Scan on Roles
	mock := &MockExecutor{
		ExecFn: func(q string, args ...any) error { return nil },
		QueryFn: func(q string, args ...any) (rbac.Rows, error) {
			if strings.Contains(q, "SELECT id, code, name, description FROM rbac_roles") {
				return &MockRows{
					NextFn: func() bool { return true }, // One row
					ScanFn: func(dest ...any) error { return errors.New("scan error roles") },
				}, nil
			}
			return &MockRows{}, nil
		},
	}
	if _, err := rbac.New(mock); err == nil || err.Error() != "scan error roles" {
		t.Error("expected scan error roles")
	}

	// Fail Scan on Perms
	mock = &MockExecutor{
		ExecFn: func(q string, args ...any) error { return nil },
		QueryFn: func(q string, args ...any) (rbac.Rows, error) {
			if strings.Contains(q, "SELECT id, name, resource, action FROM rbac_permissions") {
				return &MockRows{
					NextFn: func() bool { return true },
					ScanFn: func(dest ...any) error { return errors.New("scan error perms") },
				}, nil
			}
			return &MockRows{}, nil
		},
	}
	if _, err := rbac.New(mock); err == nil || err.Error() != "scan error perms" {
		t.Error("expected scan error perms")
	}

	// Fail Scan on RolePerms
	mock = &MockExecutor{
		ExecFn: func(q string, args ...any) error { return nil },
		QueryFn: func(q string, args ...any) (rbac.Rows, error) {
			if strings.Contains(q, "FROM rbac_role_permissions") {
				return &MockRows{
					NextFn: func() bool { return true },
					ScanFn: func(dest ...any) error { return errors.New("scan error role_perms") },
				}, nil
			}
			return &MockRows{}, nil
		},
	}
	if _, err := rbac.New(mock); err == nil || err.Error() != "scan error role_perms" {
		t.Error("expected scan error role_perms")
	}

	// Fail Scan on UserRoles
	mock = &MockExecutor{
		ExecFn: func(q string, args ...any) error { return nil },
		QueryFn: func(q string, args ...any) (rbac.Rows, error) {
			if strings.Contains(q, "FROM rbac_user_roles") {
				return &MockRows{
					NextFn: func() bool { return true },
					ScanFn: func(dest ...any) error { return errors.New("scan error user_roles") },
				}, nil
			}
			return &MockRows{}, nil
		},
	}
	if _, err := rbac.New(mock); err == nil || err.Error() != "scan error user_roles" {
		t.Error("expected scan error user_roles")
	}
}

func TestLoadCache_RowsErr(t *testing.T) {
	// Fail Err() on Roles
	mock := &MockExecutor{
		ExecFn: func(q string, args ...any) error { return nil },
		QueryFn: func(q string, args ...any) (rbac.Rows, error) {
			if strings.Contains(q, "SELECT id, code, name, description FROM rbac_roles") {
				return &MockRows{
					NextFn: func() bool { return false },
					ErrFn: func() error { return errors.New("rows err roles") },
				}, nil
			}
			return &MockRows{}, nil
		},
	}
	if _, err := rbac.New(mock); err == nil || err.Error() != "rows err roles" {
		t.Error("expected rows err roles")
	}
	// Can repeat for others, but coverage should be hit.
}
