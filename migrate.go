//go:build !wasm

package user

func runMigrations(exec Executor) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE,
			name TEXT,
			phone TEXT,
			status TEXT DEFAULT 'active',
			created_at INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS user_sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			expires_at INTEGER,
			ip TEXT,
			user_agent TEXT,
			created_at INTEGER,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS user_identities (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			provider TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			email TEXT,
			created_at INTEGER,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(provider, provider_id)
		)`,
		`CREATE TABLE IF NOT EXISTS user_oauth_states (
			state TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			expires_at INTEGER,
			created_at INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS user_lan_ips (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			ip TEXT NOT NULL,
			label TEXT,
			created_at INTEGER,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id, ip)
		)`,
	}

	for _, q := range queries {
		if err := exec.Exec(q); err != nil {
			return err
		}
	}
	return nil
}
