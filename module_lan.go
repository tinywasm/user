package user

type lanModule struct {
	m *Module
}

func (m *lanModule) HandlerName() string { return "lan" }
func (m *lanModule) ModuleTitle() string { return "LAN Auth" }
