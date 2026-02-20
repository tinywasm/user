package user

import (
	"database/sql"
)

// Executor interface abstracts database operations.
type Executor interface {
	Exec(query string, args ...any) error
	Query(query string, args ...any) (Rows, error)
	QueryRow(query string, args ...any) Scanner
	Prepare(query string) (*sql.Stmt, error)
	Begin() (*sql.Tx, error)
}

// Scanner interface abstracts scanning a row.
type Scanner interface {
	Scan(dest ...any) error
}

// Rows interface abstracts scanning multiple rows.
type Rows interface {
	Scan(dest ...any) error
	Next() bool
	Close() error
	Err() error
}
