//go:build !wasm

package user

func (m *lanModule) RenderHTML() string {
	return `<table><thead><tr><th>IP</th><th>Label</th><th>Created At</th><th>Action</th></tr></thead><tbody></tbody></table>`
}

func (m *lanModule) Create(data ...any) (any, error) {
	return nil, nil
}

func (m *lanModule) Delete(id string) error {
	return nil
}
