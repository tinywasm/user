package user

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/form"
)

type registerModule struct {
	m *Module

	form *form.Form
}

func (m *registerModule) HandlerName() string { return "register" }
func (m *registerModule) ModuleTitle() string { return "Register" }

func (m *registerModule) ValidateData(action byte, data ...any) error {
	if len(data) == 0 {
		return nil
	}
	fielder, ok := data[0].(fmt.Fielder)
	if !ok {
		return nil
	}
	return m.form.ValidateData(action, fielder)
}
