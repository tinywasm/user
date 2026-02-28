package tests

import (
	"strings"
	"testing"

	"github.com/tinywasm/rbac"
)

func TestCreatePermission(t *testing.T) {
	s, mock := newMockStore(t)

	mock.ExecFn = func(q string, args ...any) error { return nil }
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

	err := s.CreatePermission("pid1", "view_invoice", "invoice", 'r')
	if err != nil {
		t.Fatalf("CreatePermission failed: %v", err)
	}

	p, _ := s.GetPermission("pid1")
	if err != nil {
		t.Fatalf("GetPermission failed: %v", err)
	}
	if p == nil || p.Resource != "invoice" {
		t.Errorf("Permission not found or incorrect: %v", p)
	}
}

func TestDeletePermission(t *testing.T) {
	s, mock := newMockStore(t)

	// Pre-populate
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

	// Delete
	execCalled := false
	mock.ExecFn = func(q string, args ...any) error {
		if strings.Contains(q, "DELETE FROM rbac_permissions") {
			execCalled = true
			return nil
		}
		return nil
	}

	if err := s.DeletePermission("pid1"); err != nil {
		t.Fatalf("DeletePermission failed: %v", err)
	}
	if !execCalled {
		t.Error("Exec not called for DeletePermission")
	}

	p, _ := s.GetPermission("pid1")
	if p != nil {
		t.Errorf("Permission still exists after delete: %v", p)
	}
}
