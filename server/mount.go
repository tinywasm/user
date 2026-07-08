package userserver

import (
	"github.com/tinywasm/json"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/router"
	"github.com/tinywasm/user"
)

// MountAPI publishes the authentication flows on the host router. The module
// owns its routes; consumers just Mount it like any other APIModule.
func (m *Module) MountAPI(r router.Router) {
	afterLogin := m.config.AfterLoginPath
	if afterLogin == "" {
		afterLogin = user.PathAfterLogin
	}

	r.Get(user.PathLogin, func(ctx router.Context) {
		ctx.SetHeader("Content-Type", "text/html; charset=utf-8")
		ctx.Write([]byte(m.wrapSSR(m.loginUI().RenderHTML())))
	})

	r.Post(user.PathLogin, func(ctx router.Context) {
		data := &user.LoginData{}
		if err := json.Decode(string(ctx.Body()), data); err != nil {
			m.renderLoginError(ctx, err)
			return
		}

		u, err := m.Login(data.Email, data.Password)
		if err != nil {
			m.notify(user.SecurityEvent{
				Type:   user.EventAccessDenied,
				IP:     extractClientIP(ctx, m.config.TrustProxy),
				UserID: data.Email,
			})
			m.renderLoginError(ctx, err)
			return
		}

		if err := m.loginUI().SetCookie(u.ID, ctx); err != nil {
			m.renderLoginError(ctx, err)
			return
		}

		m.redirect(ctx, afterLogin)
	})

	r.Post(user.PathLogout, func(ctx router.Context) {
		cookie, ok := ctx.Cookie(m.config.CookieName)
		if ok {
			if m.config.AuthMode == user.AuthModeCookie {
				m.cache.delete(cookie.Value)
				qb := m.db.Query(&user.Session{}).Where(user.Session_.ID).Eq(cookie.Value)
				if sess, err := user.ReadOneSession(qb, &user.Session{}); err == nil {
					m.db.Delete(sess, orm.Eq(user.Session_.ID, sess.ID))
				}
			}
		}

		ctx.SetCookie(router.Cookie{
			Name:     m.config.CookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		})
		m.redirect(ctx, user.PathLogin)
	})

	// OAuth routes
	for _, p := range m.registeredProviders() {
		providerName := p.Name()
		r.Get("/oauth/"+providerName, func(ctx router.Context) {
			url, err := m.BeginOAuth(providerName)
			if err != nil {
				ctx.WriteStatus(500)
				ctx.Write([]byte(err.Error()))
				return
			}
			m.redirect(ctx, url)
		})

		r.Get("/oauth/callback/"+providerName, func(ctx router.Context) {
			ip := extractClientIP(ctx, m.config.TrustProxy)
			ua := ctx.GetHeader("User-Agent")
			u, _, err := m.CompleteOAuth(providerName, ctx, ip, ua)
			if err != nil {
				m.renderLoginError(ctx, err)
				return
			}

			if err := m.loginUI().SetCookie(u.ID, ctx); err != nil {
				m.renderLoginError(ctx, err)
				return
			}

			m.redirect(ctx, afterLogin)
		})
	}
}

func (m *Module) loginUI() *loginModule {
	for _, mod := range m.UIModules() {
		if l, ok := mod.(*loginModule); ok {
			return l
		}
	}
	panic("userserver: loginModule not found in UIModules")
}

func (m *Module) renderLoginError(ctx router.Context, err error) {
	ctx.SetHeader("Content-Type", "text/html; charset=utf-8")
	html := `<div style="color:red">` + err.Error() + `</div>` + m.loginUI().RenderHTML()
	ctx.Write([]byte(m.wrapSSR(html)))
}

func (m *Module) redirect(ctx router.Context, path string) {
	ctx.SetHeader("Location", path)
	ctx.WriteStatus(302)
}
