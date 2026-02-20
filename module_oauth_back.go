//go:build !wasm

package user

func (m *oauthModule) RenderHTML() string {
	return `<div>OAuth Callback Processing...</div>`
}

// Additional backend logic would go here
