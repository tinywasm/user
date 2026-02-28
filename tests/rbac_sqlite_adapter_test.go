package tests

import (
	"database/sql"
	"regexp"
	"testing"

	"github.com/tinywasm/rbac"
	_ "modernc.org/sqlite"
)

// sqliteAdapter wraps *sql.DB and converts $N placeholders to ? for SQLite.
type sqliteAdapter struct{ db *sql.DB }

var rePlaceholder = regexp.MustCompile(`\$\d+`)

func toPosParams(q string) string {
	return rePlaceholder.ReplaceAllString(q, "?")
}

func (a *sqliteAdapter) Exec(q string, args ...any) error {
	_, err := a.db.Exec(toPosParams(q), args...)
	return err
}

func (a *sqliteAdapter) QueryRow(q string, args ...any) rbac.Scanner {
	return a.db.QueryRow(toPosParams(q), args...)
}

func (a *sqliteAdapter) Query(q string, args ...any) (rbac.Rows, error) {
	return a.db.Query(toPosParams(q), args...)
}

// newSQLiteStore returns a real store backed by in-memory SQLite.
func newSQLiteStore(t *testing.T) *rbac.Store {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	// Enable Foreign Keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}

	adapter := &sqliteAdapter{db: db}
	s, err := rbac.New(adapter)
	if err != nil {
		t.Fatalf("rbac.New failed: %v", err)
	}
	return s
}
