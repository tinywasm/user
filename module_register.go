package user

import "github.com/tinywasm/form"

type registerModule struct {
	m *Module

	form *form.Form
}

func (m *registerModule) HandlerName() string { return "register" }
func (m *registerModule) ModuleTitle() string { return "Register" }

func (m *registerModule) ValidateData(action byte, data ...any) error {
	return m.form.ValidateData(action, data...)
}
