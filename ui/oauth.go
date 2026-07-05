package userui

import (
	"github.com/tinywasm/dom"
	"github.com/tinywasm/html"
)

type oauthModule struct {
	m any
}

func (m *oauthModule) HandlerName() string { return "oauth/callback" }
func (m *oauthModule) ModuleTitle() string { return "OAuth Callback" }

func (m *oauthModule) Render() *dom.Element {
	return html.Div().Text("OAuth Callback Processing...")
}
