package user

import "github.com/tinywasm/form"

type profileModule struct {
	form         *form.Form
	passwordForm *form.Form
}

func (m *profileModule) HandlerName() string { return "profile" }
func (m *profileModule) ModuleTitle() string { return "Profile" }

func (m *profileModule) ValidateData(action byte, data ...any) error {
	if len(data) == 0 {
		return nil
	}

	switch data[0].(type) {
	case *ProfileData:
		return m.form.ValidateData(action, data...)
	case *PasswordData:
		return m.passwordForm.ValidateData(action, data...)
	}
	return nil
}
