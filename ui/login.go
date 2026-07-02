package userui

import (
	"github.com/tinywasm/dom"
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/form"
)

type loginModule struct {
	m any

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

func (m *loginModule) Render() *dom.Element {
	return m.form.Render()
}
