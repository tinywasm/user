package user

type oauthModule struct {
	m *Module
}

func (m *oauthModule) HandlerName() string { return "oauth/callback" }
func (m *oauthModule) ModuleTitle() string { return "OAuth Callback" }
