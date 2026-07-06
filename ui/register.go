package userui

import "github.com/tinywasm/model"

import (
	"github.com/tinywasm/dom"
	"github.com/tinywasm/form"
)

type registerModule struct {
	m any

	form *form.Form
}

func (m *registerModule) HandlerName() string { return "register" }
func (m *registerModule) ModuleTitle() string { return "Register" }

func (m *registerModule) ValidateData(action byte, data ...any) error {
	if len(data) == 0 {
		return nil
	}
	fielder, ok := data[0].(model.Fielder)
	if !ok {
		return nil
	}
	return m.form.ValidateData(action, fielder)
}

func (m *registerModule) Render() *dom.Element {
	return m.form.Render()
}
