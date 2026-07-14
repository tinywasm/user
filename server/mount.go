package userserver

import (

	"github.com/tinywasm/json"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/router"
	"github.com/tinywasm/user"
)

// Module is an APIModule: it mounts its own routes and carries its own identity.
// The assertion is the contract — without it the missing half of the interface only
// surfaced at the consumer, who then wrote a local wrapper to paper over it. A
// library that says "Mount me like any other APIModule" must fail to compile when it
// isn't one.
var _ router.APIModule = (*Module)(nil)

// ModelName is the module's identity (model.ModuleNaming), used as the RBAC resource
// and as the key by which a host registers it.
func (m *Module) ModelName() string { return "user" }

// MountAPI publishes the authentication flows on the host router. The module
// owns its routes; consumers just Mount it like any other APIModule.
func (m *Module) MountAPI(r router.Router) {
	afterLogin := m.config.AfterLoginPath
	if afterLogin == "" {
		afterLogin = user.PathAfterLogin
	}

	r.Post(user.PathLogin, func(ctx router.Context) {
		ip := extractClientIP(ctx, m.config.TrustProxy)
		if m.config.RateLimit != nil {
			if err := m.config.RateLimit(ip); err != nil {
				ctx.WriteStatus(429)
				ctx.Write([]byte(err.Error()))
				return
			}
		}

		data := &user.LoginData{}
		if err := json.Decode(string(ctx.Body()), data); err != nil {
			ctx.WriteStatus(400)
			ctx.Write([]byte(err.Error()))
			return
		}

		u, err := m.Login(data.Email, data.Password)
		if err != nil {
			m.notify(user.SecurityEvent{
				Type:   user.EventAccessDenied,
				IP:     extractClientIP(ctx, m.config.TrustProxy),
				UserID: data.Email,
			})
			ctx.WriteStatus(401)
			ctx.Write([]byte(err.Error()))
			return
		}

		var value string
		if m.config.AuthMode == user.AuthModeJWT {
			token, err := m.issueToken(u.Id, m.config.TokenTTL)
			if err != nil {
				ctx.WriteStatus(500)
				ctx.Write([]byte(err.Error()))
				return
			}
			value = token
		} else {
			ua := ctx.GetHeader("User-Agent")
			sess, err := m.CreateSession(u.Id, extractClientIP(ctx, m.config.TrustProxy), ua)
			if err != nil {
				ctx.WriteStatus(500)
				ctx.Write([]byte(err.Error()))
				return
			}
			value = sess.Id
		}

		ctx.SetCookie(router.Cookie{
			Name:     m.config.CookieName,
			Value:    value,
			HttpOnly: true,
			Secure:   true,
			SameSite: router.SameSiteStrict,
			MaxAge:   m.config.TokenTTL,
			Path:     "/",
		})

		ctx.SetHeader("Location", afterLogin)
		ctx.WriteStatus(302)
	}).Public()

	r.Post(user.PathLogout, func(ctx router.Context) {
		cookie, ok := ctx.Cookie(m.config.CookieName)
		if ok {
			if m.config.AuthMode == user.AuthModeCookie {
				m.cache.delete(cookie.Value)
				qb := m.db.Query(&user.Session{}).Where(user.Session_.Id).Eq(cookie.Value)
				if sess, err := user.ReadOneSession(qb, &user.Session{}); err == nil {
					m.db.Delete(sess, orm.Eq(user.Session_.Id, sess.Id))
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
		ctx.SetHeader("Location", user.PathLogin)
		ctx.WriteStatus(302)
	}).Public()

	for _, p := range m.registeredProviders() {
		providerName := p.Name()
		r.Get("/oauth/"+providerName, func(ctx router.Context) {
			url, err := m.BeginOAuth(providerName)
			if err != nil {
				ctx.WriteStatus(500)
				ctx.Write([]byte(err.Error()))
				return
			}
			ctx.SetHeader("Location", url)
			ctx.WriteStatus(302)
		}).Public()

		r.Get("/oauth/callback/"+providerName, func(ctx router.Context) {
			ip := extractClientIP(ctx, m.config.TrustProxy)
			ua := ctx.GetHeader("User-Agent")
			u, _, err := m.CompleteOAuth(providerName, ctx, ip, ua)
			if err != nil {
				ctx.WriteStatus(401)
				ctx.Write([]byte(err.Error()))
				return
			}

			var value string
			if m.config.AuthMode == user.AuthModeJWT {
				token, err := m.issueToken(u.Id, m.config.TokenTTL)
				if err != nil {
					ctx.WriteStatus(500)
					ctx.Write([]byte(err.Error()))
					return
				}
				value = token
			} else {
				sess, err := m.CreateSession(u.Id, ip, ua)
				if err != nil {
					ctx.WriteStatus(500)
					ctx.Write([]byte(err.Error()))
					return
				}
				value = sess.Id
			}

			ctx.SetCookie(router.Cookie{
				Name:     m.config.CookieName,
				Value:    value,
				HttpOnly: true,
				Secure:   true,
				SameSite: router.SameSiteStrict,
				MaxAge:   m.config.TokenTTL,
				Path:     "/",
			})

			ctx.SetHeader("Location", afterLogin)
			ctx.WriteStatus(302)
		}).Public()
	}
}
