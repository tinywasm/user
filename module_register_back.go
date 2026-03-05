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
	u, err := createUser(m.m.db, d.Email, d.Name, d.Phone)
	if err != nil {
		return nil, err
	}
	if err := m.m.SetPassword(u.ID, d.Password); err != nil {
		return nil, err
	}
	return u, nil
}

func (m *registerModule) SetCookie(userID string, w http.ResponseWriter, r *http.Request) error {
	var value string

	if m.m.config.AuthMode == AuthModeJWT {
		token, err := GenerateJWT(m.m.config.JWTSecret, userID, m.m.config.TokenTTL)
		if err != nil {
			return err
		}
		value = token
	} else {
		sess, err := m.m.CreateSession(userID, extractClientIP(r, m.m.config.TrustProxy), r.UserAgent())
		if err != nil {
			return err
		}
		value = sess.ID
	}

	http.SetCookie(w, &http.Cookie{
		Name:     m.m.config.CookieName,
		Value:    value,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   m.m.config.TokenTTL,
		Path:     "/",
	})
	return nil
}
