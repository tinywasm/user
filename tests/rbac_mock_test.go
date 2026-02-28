package tests

import (
	"errors"

	"github.com/tinywasm/rbac"
)

// MockExecutor implements rbac.Executor for testing.
type MockExecutor struct {
	ExecFn     func(query string, args ...any) error
	QueryRowFn func(query string, args ...any) rbac.Scanner
	QueryFn    func(query string, args ...any) (rbac.Rows, error)
}

func (m *MockExecutor) Exec(q string, args ...any) error {
	if m.ExecFn == nil {
		return errors.New("unexpected Exec call: " + q)
	}
	return m.ExecFn(q, args...)
}

func (m *MockExecutor) QueryRow(q string, args ...any) rbac.Scanner {
	if m.QueryRowFn == nil {
		panic("unexpected QueryRow call: " + q)
	}
	return m.QueryRowFn(q, args...)
}

func (m *MockExecutor) Query(q string, args ...any) (rbac.Rows, error) {
	if m.QueryFn == nil {
		return nil, errors.New("unexpected Query call: " + q)
	}
	return m.QueryFn(q, args...)
}

// MockScanner implements rbac.Scanner
type MockScanner struct {
	ScanFn func(dest ...any) error
}

func (m *MockScanner) Scan(dest ...any) error {
	if m.ScanFn == nil {
		return errors.New("unexpected Scan call")
	}
	return m.ScanFn(dest...)
}

// MockRows implements rbac.Rows
type MockRows struct {
	NextFn  func() bool
	ScanFn  func(dest ...any) error
	CloseFn func() error
	ErrFn   func() error
}

func (m *MockRows) Next() bool {
	if m.NextFn == nil {
		return false
	}
	return m.NextFn()
}

func (m *MockRows) Scan(dest ...any) error {
	if m.ScanFn == nil {
		return errors.New("unexpected Scan call")
	}
	return m.ScanFn(dest...)
}

func (m *MockRows) Close() error {
	if m.CloseFn != nil {
		return m.CloseFn()
	}
	return nil
}

func (m *MockRows) Err() error {
	if m.ErrFn != nil {
		return m.ErrFn()
	}
	return nil
}
