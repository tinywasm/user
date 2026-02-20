//go:build !wasm

package user

import "net/http"

func (m *loginModule) RenderHTML() string {
	m.form.SetSSR(true)
	out := m.form.RenderHTML()
	for _, p := range registeredProviders() {
		out += `<a href="/oauth/` + p.Name() + `">Login with ` + p.Name() + `</a>`
	}
	return out
}

func (m *loginModule) Create(data ...any) (any, error) {
	if len(data) == 0 {
		return nil, ErrInvalidCredentials
	}
	d, ok := data[0].(*LoginData)
	if !ok {
		return nil, ErrInvalidCredentials
	}
	u, err := Login(d.Email, d.Password)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (m *loginModule) SetCookie(userID string, w http.ResponseWriter, r *http.Request) error {
	sess, err := CreateSession(userID, extractClientIP(r, store.config.TrustProxy), r.UserAgent())
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName(),
		Value:    sess.ID,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   store.config.SessionTTL,
		Path:     "/",
	})
	return nil
}
