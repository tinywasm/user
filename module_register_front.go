//go:build wasm

package user

func (m *registerModule) OnMount() {
	m.form.OnMount()
}
