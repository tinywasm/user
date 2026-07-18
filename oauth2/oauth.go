package oauth2

import (
	"github.com/tinywasm/router"
	"github.com/tinywasm/user"
)

type Authenticator struct {
	providers []user.OAuthProvider
}

func New(providers ...user.OAuthProvider) *Authenticator {
	return &Authenticator{providers: providers}
}

func (a *Authenticator) Name() string {
	return "oauth2"
}

func (a *Authenticator) Mount(r router.Router, module any) {
	m := module.(user.ModuleContext)
	for _, p := range a.providers {
		m.RegisterProvider(p)
	}

	if r == nil {
		return
	}

	for _, p := range a.providers {
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
			ip := m.ExtractClientIP(ctx)
			ua := ctx.GetHeader("User-Agent")
			u, _, err := m.CompleteOAuth(providerName, ctx, ip, ua)
			if err != nil {
				ctx.WriteStatus(401)
				ctx.Write([]byte(err.Error()))
				return
			}

			cfg := m.Config()
			afterLogin := cfg.AfterLoginPath
			if afterLogin == "" {
				afterLogin = user.PathAfterLogin
			}

			var value string
			if cfg.AuthMode == user.AuthModeJWT {
				token, err := m.IssueToken(u.Id, cfg.TokenTTL)
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
				Name:     cfg.CookieName,
				Value:    value,
				HttpOnly: true,
				Secure:   true,
				SameSite: router.SameSiteStrict,
				MaxAge:   cfg.TokenTTL,
				Path:     "/",
			})

			ctx.SetHeader("Location", afterLogin)
			ctx.WriteStatus(302)
		}).Public()
	}
}
