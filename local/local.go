package local

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/router"
	"github.com/tinywasm/user"

	"golang.org/x/crypto/bcrypt"
)

var PasswordHashCost = bcrypt.DefaultCost
var dummyHashOnce []byte

func getDummyHash(cost int) []byte {
	if len(dummyHashOnce) == 0 {
		dummyHashOnce, _ = bcrypt.GenerateFromPassword([]byte("dummy"), cost)
	}
	return dummyHashOnce
}

type Authenticator struct{}

func New() *Authenticator {
	return &Authenticator{}
}

func (a *Authenticator) Name() string {
	return "local"
}

func (a *Authenticator) Mount(r router.Router, module any) {
	m := module.(user.ModuleContext)
	cfg := m.Config()

	if r == nil {
		return
	}

	afterLogin := cfg.AfterLoginPath
	if afterLogin == "" {
		afterLogin = user.PathAfterLogin
	}

	r.Post(user.PathLogin, func(ctx router.Context) {
		ip := m.ExtractClientIP(ctx)
		data := &user.LoginData{}
		if err := ctx.Decode(data); err != nil {
			ctx.WriteStatus(400)
			ctx.Write([]byte(err.Error()))
			return
		}

		if cfg.RateLimit != nil {
			if err := cfg.RateLimit(ip); err != nil {
				m.Notify(user.SecurityEvent{
					Type:   user.EventRateLimited,
					IP:     ip,
					UserID: data.Email,
				})
				ctx.WriteStatus(429)
				ctx.Write([]byte(err.Error()))
				return
			}
		}

		u, err := m.Login(data.Email, data.Password)
		if err != nil {
			m.Notify(user.SecurityEvent{
				Type:   user.EventAccessDenied,
				IP:     ip,
				UserID: data.Email,
			})
			ctx.WriteStatus(401)
			ctx.Write([]byte(err.Error()))
			return
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
			ua := ctx.GetHeader("User-Agent")
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

	r.Post(user.PathLogout, func(ctx router.Context) {
		cookie, ok := ctx.Cookie(cfg.CookieName)
		if ok {
			if cfg.AuthMode == user.AuthModeCookie {
				m.DeleteSession(cookie.Value)
			}
		}

		ctx.SetCookie(router.Cookie{
			Name:     cfg.CookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		})
		ctx.SetHeader("Location", user.PathLogin)
		ctx.WriteStatus(302)
	}).Public()
}

func (a *Authenticator) VerifyPassword(db *orm.DB, userID, password string, getLocalIdentity func(*orm.DB, string) (user.Identity, error)) error {
	identity, err := getLocalIdentity(db, userID)
	if err != nil {
		return user.ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(identity.ProviderId), []byte(password)); err != nil {
		return user.ErrInvalidCredentials
	}
	return nil
}

func (a *Authenticator) SetPassword(db *orm.DB, ids model.IDGenerator, userID, password string, upsertIdentity func(*orm.DB, model.IDGenerator, string, string, string, string) error) error {
	if len(password) < 8 {
		return user.ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), PasswordHashCost)
	if err != nil {
		return err
	}
	return upsertIdentity(db, ids, userID, "local", string(hash), "")
}

func (a *Authenticator) Login(db *orm.DB, email, password string, getUserByEmail func(*orm.DB, string) (user.User, error), getLocalIdentity func(*orm.DB, string) (user.Identity, error), notify func(user.SecurityEvent)) (user.User, error) {
	u, err := getUserByEmail(db, email)
	if err != nil {
		bcrypt.CompareHashAndPassword(getDummyHash(PasswordHashCost), []byte(password))
		return user.User{}, user.ErrInvalidCredentials
	}
	if u.Status != "active" {
		notify(user.SecurityEvent{Type: user.EventNonActiveAccess, UserID: u.Id})
		bcrypt.CompareHashAndPassword(getDummyHash(PasswordHashCost), []byte(password))
		return user.User{}, user.ErrInvalidCredentials
	}

	identity, err := getLocalIdentity(db, u.Id)
	if err != nil {
		bcrypt.CompareHashAndPassword(getDummyHash(PasswordHashCost), []byte(password))
		return user.User{}, user.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(identity.ProviderId), []byte(password)); err != nil {
		return user.User{}, user.ErrInvalidCredentials
	}
	return u, nil
}
