//go:build !wasm

package user

import "github.com/tinywasm/orm"

func initSchema(db *orm.DB, mode AuthMode) error {
	models := []orm.Model{
		&User{}, &Role{}, &Permission{},
		&Identity{}, &LANIP{},
		&OAuthState{}, &UserRole{}, &RolePermission{},
	}
	if mode == AuthModeCookie {
		models = append(models, &Session{})
	}
	for _, m := range models {
		if err := db.CreateTable(m); err != nil {
			return err
		}
	}
	return nil
}
