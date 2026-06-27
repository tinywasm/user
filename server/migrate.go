package userserver

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/user"
)

func initSchema(db *orm.DB, mode user.AuthMode) error {
	models := []fmt.Model{
		&user.User{}, &user.Role{}, &user.Permission{},
		&user.Identity{}, &user.LANIP{},
		&user.OAuthState{}, &user.UserRole{}, &user.RolePermission{},
	}
	if mode == user.AuthModeCookie {
		models = append(models, &user.Session{})
	}
	for _, m := range models {
		if err := db.CreateTable(m); err != nil {
			return err
		}
	}
	return nil
}
