package user

import (
	_ "github.com/tinywasm/fmt/dictionary"
	"github.com/tinywasm/form"
	"github.com/tinywasm/form/input"
)

var (
	LoginModule    *loginModule
	RegisterModule *registerModule
	ProfileModule  *profileModule
	LANModule      *lanModule
	OAuthCallback  *oauthModule
)

func init() {
	form.RegisterInput(
		input.Password("", "current"),
		input.Password("", "new"),
		input.Password("", "confirm"),
	)

	LoginModule = &loginModule{form: mustForm("login", &LoginData{})}
	RegisterModule = &registerModule{form: mustForm("register", &RegisterData{})}
	ProfileModule = &profileModule{
		form:         mustForm("profile", &ProfileData{}),
		passwordForm: mustForm("password", &PasswordData{}),
	}
	LANModule = &lanModule{}
	OAuthCallback = &oauthModule{}
}

func mustForm(parentID string, s any) *form.Form {
	f, err := form.New(parentID, s)
	if err != nil {
		panic("user: mustForm: " + err.Error())
	}
	return f
}
