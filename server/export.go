package userserver

import "github.com/tinywasm/user"

func (m *Module) ExportGetUserByEmail(email string) (user.User, error) {
	return getUserByEmail(m.db, m.ucache, email)
}
