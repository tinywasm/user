package userui

import (
	"github.com/tinywasm/dom"
	"github.com/tinywasm/html"
)

type lanModule struct {
	m any

	rows *dom.SignalNodes
}

func (m *lanModule) HandlerName() string { return "lan" }
func (m *lanModule) ModuleTitle() string { return "LAN Auth" }

func (m *lanModule) Init(ctx dom.Ctx) {
	m.rows = dom.NewNodes()
}

func (m *lanModule) Render() *dom.Element {
	return html.Table().Child(
		html.Thead().Child(
			html.Tr().Child(
				html.Th().Text("IP"),
				html.Th().Text("Label"),
				html.Th().Text("Created At"),
				html.Th().Text("Action"),
			),
		),
		html.Tbody().BindChildren(m.rows),
	)
}
