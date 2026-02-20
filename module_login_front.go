//go:build wasm

package user

func (m *loginModule) OnMount() {
	m.form.OnMount()
}
