package user

import "github.com/tinywasm/form"

type loginModule struct {
	form *form.Form
}

func (m *loginModule) HandlerName() string { return "login" }
func (m *loginModule) ModuleTitle() string { return "Login" }

func (m *loginModule) ValidateData(action byte, data ...any) error {
	return m.form.ValidateData(action, data...)
}
