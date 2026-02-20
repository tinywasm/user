//go:build wasm

package user

func (m *profileModule) OnMount() {
	m.form.OnMount()
	m.passwordForm.OnMount()
}
