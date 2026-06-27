package userui

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/form"
	"github.com/tinywasm/user"
)

type profileModule struct {
	m any

	form         *form.Form
	passwordForm *form.Form
}

func (m *profileModule) HandlerName() string { return "profile" }
func (m *profileModule) ModuleTitle() string { return "Profile" }

func (m *profileModule) ValidateData(action byte, data ...any) error {
	if len(data) == 0 {
		return nil
	}

	fielder, ok := data[0].(fmt.Fielder)
	if !ok {
		return nil
	}

	switch data[0].(type) {
	case *user.ProfileData:
		return m.form.ValidateData(action, fielder)
	case *user.PasswordData:
		return m.passwordForm.ValidateData(action, fielder)
	}
	return nil
}

func (m *profileModule) OnMount() {
	m.form.OnMount()
	m.passwordForm.OnMount()
}
