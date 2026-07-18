package authority

import (
	"github.com/tinywasm/ddl"
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/user"
)

func initSchema(db *orm.DB, mode user.AuthMode) error {
	models := []model.Model{
		&user.User{}, &user.Role{}, &user.Permission{},
		&user.Identity{}, &user.LANIP{},
		&user.OAuthState{}, &user.UserRole{}, &user.RolePermission{},
	}
	if mode == user.AuthModeCookie {
		models = append(models, &user.Session{})
	}
	ddlCompiler, ok := db.RawConn().(ddl.Compiler)
	if !ok {
		// Backend sin capacidad DDL (p. ej. storage/mem en tests): crea tablas
		// perezosamente en el primer Exec — no-op correcto, NO un error.
		return nil
	}
	sorted, err := ddl.TopologicalSort(models)
	if err != nil {
		return err
	}
	return ddl.New(db.RawConn(), ddlCompiler).Sync(sorted...)
}
