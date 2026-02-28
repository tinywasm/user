package tests

import (
	"testing"

	"github.com/tinywasm/rbac"
)

// newMockStore returns a store initialized with a MockExecutor that succeeds on initialization.
func newMockStore(t *testing.T) (*rbac.Store, *MockExecutor) {
	mock := &MockExecutor{
		ExecFn: func(q string, args ...any) error {
			// Assume success for DDLs
			return nil
		},
		QueryFn: func(q string, args ...any) (rbac.Rows, error) {
			// Assume success for loadCache queries (empty results)
			return &MockRows{}, nil
		},
	}
	s, err := rbac.New(mock)
	if err != nil {
		t.Fatalf("newMockStore failed: %v", err)
	}
	return s, mock
}
