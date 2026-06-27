package userui

type lanModule struct {
	m any
}

func (m *lanModule) HandlerName() string { return "lan" }
func (m *lanModule) ModuleTitle() string { return "LAN Auth" }

func (m *lanModule) OnMount() {
	// Add/remove IP rows logic
}
