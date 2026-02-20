package user

type lanModule struct{}

func (m *lanModule) HandlerName() string { return "lan" }
func (m *lanModule) ModuleTitle() string { return "LAN Auth" }
