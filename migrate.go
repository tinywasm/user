//go:build !wasm

package user

import "github.com/tinywasm/orm"

func initSchema(db *orm.DB) error {
	models := []orm.Model{
		&User{}, &Role{}, &Permission{},
		&Identity{}, &Session{}, &LANIP{},
		&OAuthState{}, &UserRole{}, &RolePermission{},
	}
	for _, m := range models {
		if err := db.CreateTable(m); err != nil {
			return err
		}
	}
	return nil
}
