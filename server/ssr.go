package userserver

import (
	"net/http"

	"github.com/tinywasm/form"
	"github.com/tinywasm/user"
)

type ssrForm interface {
	SetSSR(bool)
	String() string
}

type loginModule struct {
	m    *Module
	form *form.Form
}

func (m *loginModule) HandlerName() string { return "login" }

func (m *loginModule) RenderHTML() string {
	if m.form != nil {
		m.form.SetSSR(true)
		out := m.form.String()
		for _, p := range m.m.registeredProviders() {
			out += `<a href="/oauth/` + p.Name() + `">Login with ` + p.Name() + `</a>`
		}
		return out
	}
	return ""
}

func (m *loginModule) Create(data ...any) (any, error) {
	if len(data) == 0 {
		return nil, user.ErrInvalidCredentials
	}
	d, ok := data[0].(*user.LoginData)
	if !ok {
		return nil, user.ErrInvalidCredentials
	}
	u, err := m.m.Login(d.Email, d.Password)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (m *loginModule) SetCookie(userID string, w http.ResponseWriter, r *http.Request) error {
	var value string

	if m.m.config.AuthMode == user.AuthModeJWT {
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

type registerModule struct {
	m    *Module
	form *form.Form
}

func (m *registerModule) HandlerName() string { return "register" }

func (m *registerModule) RenderHTML() string {
	if m.form != nil {
		m.form.SetSSR(true)
		return m.form.String()
	}
	return ""
}

func (m *registerModule) Create(data ...any) (any, error) {
	if len(data) == 0 {
		return nil, user.ErrInvalidCredentials
	}
	d, ok := data[0].(*user.RegisterData)
	if !ok {
		return nil, user.ErrInvalidCredentials
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

	if m.m.config.AuthMode == user.AuthModeJWT {
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

type profileModule struct {
	m            *Module
	form         *form.Form
	passwordForm *form.Form
}

func (m *profileModule) HandlerName() string { return "profile" }

func (m *profileModule) RenderHTML() string {
	out := ""
	if m.form != nil {
		m.form.SetSSR(true)
		out += m.form.String()
	}
	if m.passwordForm != nil {
		out += "<hr>"
		m.passwordForm.SetSSR(true)
		out += m.passwordForm.String()
	}
	return out
}

func (m *profileModule) Update(id string, data ...any) error {
	if len(data) == 0 {
		return nil
	}

	switch d := data[0].(type) {
	case *user.ProfileData:
		return updateUser(m.m.db, m.m.ucache, id, d.Name, d.Phone)
	case *user.PasswordData:
		if d.New != d.Confirm {
			return user.ErrInvalidCredentials
		}
		if err := m.m.VerifyPassword(id, d.Current); err != nil {
			return err
		}
		return m.m.SetPassword(id, d.New)
	}
	return nil
}

type oauthModule struct {
	m *Module
}

func (m *oauthModule) HandlerName() string { return "oauth/callback" }

func (m *oauthModule) RenderHTML() string {
	return `<div>OAuth Callback Processing...</div>`
}

type lanModule struct {
	m *Module
}

func (m *lanModule) HandlerName() string { return "lan" }

func (m *lanModule) RenderHTML() string {
	return `<table><thead><tr><th>IP</th><th>Label</th><th>Created At</th><th>Action</th></tr></thead><tbody></tbody></table>`
}

func (m *lanModule) Create(data ...any) (any, error) {
	return nil, nil
}

func (m *lanModule) Delete(id string) error {
	return nil
}
