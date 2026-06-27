package userui

type oauthModule struct {
	m any
}

func (m *oauthModule) HandlerName() string { return "oauth/callback" }
func (m *oauthModule) ModuleTitle() string { return "OAuth Callback" }
