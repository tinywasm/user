package user

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/form"
	"github.com/tinywasm/form/input"
)

var uiModules []any

func init() {
	form.RegisterInput(
		input.Password("", "current"),
		input.Password("", "new"),
		input.Password("", "confirm"),
	)
	uiModules = []any{
		&loginModule{form: mustForm("login", &LoginData{})},
		&registerModule{form: mustForm("register", &RegisterData{})},
		&profileModule{
			form:         mustForm("profile", &ProfileData{}),
			passwordForm: mustForm("password", &PasswordData{}),
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
