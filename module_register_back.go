//go:build !wasm

package user

import "net/http"

func (m *registerModule) RenderHTML() string {
	m.form.SetSSR(true)
	return m.form.RenderHTML()
}

func (m *registerModule) Create(data ...any) (any, error) {
	if len(data) == 0 {
		return nil, ErrInvalidCredentials
	}
	d, ok := data[0].(*RegisterData)
	if !ok {
		return nil, ErrInvalidCredentials
	}
	u, err := CreateUser(d.Email, d.Name, d.Phone)
	if err != nil {
		return nil, err
	}
	if err := SetPassword(u.ID, d.Password); err != nil {
		return nil, err
	}
	return u, nil
}

func (m *registerModule) SetCookie(userID string, w http.ResponseWriter, r *http.Request) error {
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
