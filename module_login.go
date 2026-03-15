package user

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/form"
)

type loginModule struct {
	m *Module

	form *form.Form
}

func (m *loginModule) HandlerName() string { return "login" }
func (m *loginModule) ModuleTitle() string { return "Login" }

func (m *loginModule) ValidateData(action byte, data ...any) error {
	if len(data) == 0 {
		return nil
	}
	fielder, ok := data[0].(fmt.Fielder)
	if !ok {
		return nil
	}
	return m.form.ValidateData(action, fielder)
}
