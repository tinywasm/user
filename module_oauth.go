package user

type oauthModule struct{}

func (m *oauthModule) HandlerName() string { return "oauth/callback" }
func (m *oauthModule) ModuleTitle() string { return "OAuth Callback" }
