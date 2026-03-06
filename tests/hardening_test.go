//go:build !wasm

package tests

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tinywasm/user"
)

func getLoginMod(m *user.Module) interface {
	SetCookie(string, http.ResponseWriter, *http.Request) error
} {
	for _, mod := range m.UIModules() {
		if h, ok := mod.(interface{ HandlerName() string }); ok && h.HandlerName() == "login" {
			if lm, ok := mod.(interface {
				SetCookie(string, http.ResponseWriter, *http.Request) error
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
		m, _ := user.New(db, user.Config{TokenTTL: 3600})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)

		lm := getLoginMod(m)
		err := lm.SetCookie("user1", rec, req)
		if err != nil {
			t.Fatal(err)
		}

		cookies := rec.Result().Cookies()
		if len(cookies) == 0 {
			t.Fatal("no cookies set")
		}
		c := cookies[0]

		if c.Name != "session" {
			t.Errorf("expected cookie name 'session', got %s", c.Name)
		}
		if !c.HttpOnly {
			t.Errorf("expected HttpOnly=true")
		}
		if !c.Secure {
			t.Errorf("expected Secure=true")
		}
		if c.SameSite != http.SameSiteStrictMode {
			t.Errorf("expected SameSite=Strict, got %v", c.SameSite)
		}
		if c.Path != "/" {
			t.Errorf("expected Path=/, got %s", c.Path)
		}
		if c.MaxAge != 3600 {
			t.Errorf("expected MaxAge=3600, got %d", c.MaxAge)
		}
	})

	t.Run("JWT Mode Cookie", func(t *testing.T) {
		m, _ := user.New(db, user.Config{AuthMode: user.AuthModeJWT, JWTSecret: []byte("sec"), TokenTTL: 7200})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)

		lm := getLoginMod(m)
		_ = lm.SetCookie("user1", rec, req)

		c := rec.Result().Cookies()[0]

		// Assert flags
		if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode {
			t.Errorf("JWT cookie missing security flags")
		}

		// Assert value is JWT
		parts := strings.Split(c.Value, ".")
		if len(parts) != 3 {
			t.Errorf("expected JWT value (3 parts), got %d parts", len(parts))
		}
	})

	t.Run("Custom Cookie Name", func(t *testing.T) {
		m, _ := user.New(db, user.Config{CookieName: "custom_auth"})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)

		lm := getLoginMod(m)
		_ = lm.SetCookie("user1", rec, req)

		c := rec.Result().Cookies()[0]
		if c.Name != "custom_auth" {
			t.Errorf("expected cookie name 'custom_auth', got %s", c.Name)
		}
	})
}

func TestSessionRotation(t *testing.T) {
	db := newTestDB(t)
	m, _ := user.New(db, user.Config{TokenTTL: 3600})

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
		db.RawExecutor().Exec("UPDATE user_sessions SET expires_at = 0 WHERE id = ?", sess3.ID)
		m, _ = user.New(db, user.Config{TokenTTL: 3600}) // Clear cache

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

		reqOld := httptest.NewRequest("GET", "/", nil)
		reqOld.AddCookie(&http.Cookie{Name: "session", Value: sessOld.ID})
		recOld := httptest.NewRecorder()
		m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(recOld, reqOld)
		if recOld.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for old session")
		}

		reqNew := httptest.NewRequest("GET", "/", nil)
		reqNew.AddCookie(&http.Cookie{Name: "session", Value: sessNew.ID})
		recNew := httptest.NewRecorder()
		m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(recNew, reqNew)
		if recNew.Code != http.StatusOK {
			t.Errorf("expected 200 for new session")
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
	m, _ := user.New(db, cfg)

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
