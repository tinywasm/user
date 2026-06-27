package userui

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/form"
	"github.com/tinywasm/form/input"
	"github.com/tinywasm/user"
)

var uiModules []any

func init() {
	form.RegisterInput(
		input.Password(),
		input.Password(),
		input.Password(),
	)
	uiModules = []any{
		&loginModule{form: mustForm("login", &user.LoginData{})},
		&registerModule{form: mustForm("register", &user.RegisterData{})},
		&profileModule{
			form:         mustForm("profile", &user.ProfileData{}),
			passwordForm: mustForm("password", &user.PasswordData{}),
		},
		&lanModule{},
		&oauthModule{},
	}
}

// UIModules returns all standard authentication UI flow handlers.
// Isomorphic: available in both WASM and non-WASM builds.
func UIModules() []any { return uiModules }

func mustForm(parentID string, s fmt.Fielder) *form.Form {
	f, err := form.New(parentID, s)
	if err != nil {
		panic("user: mustForm: " + err.Error())
	}
	return f
}
