//go:build !wasm

package tests

import (
	"errors"
	"strings"
	"testing"

	"github.com/tinywasm/router"
	"github.com/tinywasm/user"
	"github.com/tinywasm/user/server"
)

func getLoginMod(m *userserver.Module) interface {
	SetCookie(string, router.Context) error
} {
	for _, mod := range m.UIModules() {
		if h, ok := mod.(interface{ HandlerName() string }); ok && h.HandlerName() == "login" {
			if lm, ok := mod.(interface {
				SetCookie(string, router.Context) error
			}); ok {
				return lm
			}
		}
	}
	return nil
}

func TestCookieSecurity(t *testing.T) {
	db := newTestDB(t)

	t.Run("Cookie Mode Flags", func(t *testing.T) {
		m, _ := userserver.New(db, user.Config{TokenTTL: 3600})
		ctx := newMockContext("GET", "/")

		lm := getLoginMod(m)
		err := lm.SetCookie("user1", ctx)
		if err != nil {
			t.Fatal(err)
		}

		cookie := ctx.GetHeader("Set-Cookie")
		if cookie == "" {
			t.Fatal("no Set-Cookie header set")
		}

		if !strings.Contains(cookie, "session=") {
			t.Errorf("expected cookie name 'session'")
		}
		if !strings.Contains(cookie, "HttpOnly") {
			t.Errorf("expected HttpOnly")
		}
		if !strings.Contains(cookie, "Secure") {
			t.Errorf("expected Secure")
		}
		if !strings.Contains(cookie, "SameSite=Strict") {
			t.Errorf("expected SameSite=Strict")
		}
		if !strings.Contains(cookie, "Path=/") {
			t.Errorf("expected Path=/")
		}
		if !strings.Contains(cookie, "Max-Age=3600") {
			t.Errorf("expected Max-Age=3600")
		}
	})

	t.Run("JWT Mode Cookie", func(t *testing.T) {
		m, _ := userserver.New(db, user.Config{AuthMode: user.AuthModeJWT, JWTSecret: []byte("sec"), TokenTTL: 7200})
		ctx := newMockContext("GET", "/")

		lm := getLoginMod(m)
		_ = lm.SetCookie("user1", ctx)

		cookie := ctx.GetHeader("Set-Cookie")

		// Assert flags
		if !strings.Contains(cookie, "HttpOnly") || !strings.Contains(cookie, "Secure") || !strings.Contains(cookie, "SameSite=Strict") {
			t.Errorf("JWT cookie missing security flags")
		}

		// Assert value is JWT
		val := cookie[strings.Index(cookie, "=")+1 : strings.Index(cookie, ";")]
		parts := strings.Split(val, ".")
		if len(parts) != 3 {
			t.Errorf("expected JWT value (3 parts), got %d parts", len(parts))
		}
	})

	t.Run("Custom Cookie Name", func(t *testing.T) {
		m, _ := userserver.New(db, user.Config{CookieName: "custom_auth"})
		ctx := newMockContext("GET", "/")

		lm := getLoginMod(m)
		_ = lm.SetCookie("user1", ctx)

		cookie := ctx.GetHeader("Set-Cookie")
		if !strings.HasPrefix(cookie, "custom_auth=") {
			t.Errorf("expected cookie name 'custom_auth'")
		}
	})
}

func TestSessionRotation(t *testing.T) {
	db := newTestDB(t)
	m, _ := userserver.New(db, user.Config{TokenTTL: 3600})

	userCRUD := getHandler(m, "users")
	resU, _ := userCRUD.Create(user.User{Email: "rot@example.com", Name: "Rot"})
	u := resU.(user.User)

	t.Run("RotateSession valid", func(t *testing.T) {
		sess1, _ := m.CreateSession(u.ID, "10.0.0.1", "ua1")

		sess2, err := m.RotateSession(sess1.ID, "10.0.0.2", "ua2")
		if err != nil {
			t.Fatalf("RotateSession failed: %v", err)
		}

		if sess2.ID == sess1.ID {
			t.Errorf("expected new session ID")
		}
		if sess2.UserID != u.ID {
			t.Errorf("expected same UserID")
		}
		if sess2.IP != "10.0.0.2" {
			t.Errorf("expected updated IP")
		}

		// Ensure old session is gone
		_, err = m.GetSession(sess1.ID)
		if err != user.ErrNotFound {
			t.Errorf("expected ErrNotFound for old session, got %v", err)
		}
	})

	t.Run("RotateSession expired", func(t *testing.T) {
		sess3, _ := m.CreateSession(u.ID, "10.0.0.1", "ua1")
		// Manually expire
		db.RawExecutor().Exec("UPDATE session SET expires_at = 0 WHERE id = ?", sess3.ID)
		m, _ = userserver.New(db, user.Config{TokenTTL: 3600}) // Clear cache

		_, err := m.RotateSession(sess3.ID, "10.0.0.1", "ua1")
		if err != user.ErrSessionExpired {
			t.Errorf("expected ErrSessionExpired, got %v", err)
		}
	})

	t.Run("RotateSession not found", func(t *testing.T) {
		_, err := m.RotateSession("nope", "10.0.0.1", "ua1")
		if err != user.ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("Middleware Continuity", func(t *testing.T) {
		sessOld, _ := m.CreateSession(u.ID, "10.0.0.1", "ua1")
		sessNew, _ := m.RotateSession(sessOld.ID, "10.0.0.1", "ua1")

		ctxOld := newMockContext("GET", "/")
		ctxOld.SetHeader("Cookie", "session="+sessOld.ID)
		m.Middleware()(func(c router.Context) {})(ctxOld)
		if ctxOld.status != 401 {
			t.Errorf("expected 401 for old session, got %d", ctxOld.status)
		}

		ctxNew := newMockContext("GET", "/")
		ctxNew.SetHeader("Cookie", "session="+sessNew.ID)
		m.Middleware()(func(c router.Context) {})(ctxNew)
		if ctxNew.status != 0 && ctxNew.status != 200 {
			t.Errorf("expected 200/0 for new session, got %d", ctxNew.status)
		}
	})
}

func TestPasswordHook(t *testing.T) {
	db := newTestDB(t)

	errHook := errors.New("custom password hook error")
	cfg := user.Config{
		OnPasswordValidate: func(password string) error {
			if strings.Contains(password, "bad") {
				return errHook
			}
			return nil
		},
	}
	m, _ := userserver.New(db, cfg)

	userCRUD := getHandler(m, "users")
	resU, _ := userCRUD.Create(user.User{Email: "hook@example.com", Name: "Hook"})
	u := resU.(user.User)
	_ = m.SetPassword(u.ID, "goodpass123") // Baseline

	t.Run("Hook rejection", func(t *testing.T) {
		err := m.SetPassword(u.ID, "thisisbadpass")
		if err != errHook {
			t.Errorf("expected custom hook error, got %v", err)
		}

		// Old password should still work
		if err := m.VerifyPassword(u.ID, "goodpass123"); err != nil {
			t.Errorf("expected old password to still be valid")
		}
	})

	t.Run("Hook success", func(t *testing.T) {
		err := m.SetPassword(u.ID, "thisisokpass")
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}

		if err := m.VerifyPassword(u.ID, "thisisokpass"); err != nil {
			t.Errorf("expected new password to work")
		}
	})

	t.Run("Length runs before hook", func(t *testing.T) {
		err := m.SetPassword(u.ID, "bad") // short and "bad"
		if err != user.ErrWeakPassword {
			t.Errorf("expected ErrWeakPassword, got %v", err)
		}
	})
}
