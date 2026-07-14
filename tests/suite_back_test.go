//go:build !wasm

package tests

import (
	"context"
	"testing"

	"github.com/tinywasm/orm"
	"github.com/tinywasm/router"
	"github.com/tinywasm/router/mock"
	"github.com/tinywasm/sqlite"
	"github.com/tinywasm/user"
	"github.com/tinywasm/user/server"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

func newTestDB(t *testing.T) *orm.DB {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	return db
}

func RunUserTests(t *testing.T) {
	userserver.PasswordHashCost = bcrypt.MinCost
	t.Run("TestInit", testInit)
	t.Run("TestCRUD", testCRUD)
	t.Run("TestAuth", testAuth)
	t.Run("TestSessions", testSessions)
	t.Run("TestOAuth", testOAuth)
	t.Run("TestLAN", testLAN)
	t.Run("TestJWTCookieMode", testJWTCookieMode)
}

func testJWTCookieMode(t *testing.T) {
	db := newTestDB(t)
	secret := []byte("test-secret-32-bytes-minimum-len")
	m, err := userserver.New(db, user.Config{
		AuthMode:  user.AuthModeJWT,
		JWTSecret: secret,
	})
	if err != nil {
		t.Fatal(err)
	}

	userCRUD := getHandler(m, "users")
	res, err := userCRUD.Create(user.User{Email: "jwt@test.com", Name: "JWT User"})
	if err != nil {
		t.Fatal(err)
	}
	u := res.(user.User)
	_ = m.SetPassword(u.Id, "password123")
	logged, err := m.Login("jwt@test.com", "password123")
	if err != nil {
		t.Fatal("login failed:", err)
	}

	// Generar JWT como lo haría SetCookie
	token, err := userserver.GenerateJWT(secret, logged.Id, 86400)
	if err != nil {
		t.Fatal(err)
	}

	// Request con JWT en cookie → debe autenticar
	ctx := &mock.Context{}
	ctx.SetCookie(router.Cookie{Name: "session", Value: token})
	var authID string
	m.Authenticate()(func(c router.Context) {
		authID = c.UserID()
	})(ctx)
	if authID != logged.Id {
		t.Errorf("JWT middleware: expected user %s, got %q", logged.Id, authID)
	}

	// Token inválido → anónimo
	ctx2 := &mock.Context{}
	ctx2.SetCookie(router.Cookie{Name: "session", Value: "invalid.jwt.token"})
	authID = ""
	m.Authenticate()(func(c router.Context) {
		authID = c.UserID()
	})(ctx2)
	if authID != "" {
		t.Errorf("want empty userID for invalid JWT, got %q", authID)
	}
}

func testInit(t *testing.T) {
	db := newTestDB(t)
	cfg := user.Config{
		CookieName: "test_session",
		TokenTTL:   3600,
	}
	m, err := userserver.New(db, cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	_ = m // to be used later
}

func getHandler(m *userserver.Module, name string) interface {
	Create(any) (any, error)
	Read(string) (any, error)
	Update(any) (any, error)
	Delete(string) error
} {
	for _, h := range m.Add() {
		if hr, ok := h.(interface{ HandlerName() string }); ok && hr.HandlerName() == name {
			return h.(interface {
				Create(any) (any, error)
				Read(string) (any, error)
				Update(any) (any, error)
				Delete(string) error
			})
		}
	}
	return nil
}

func testCRUD(t *testing.T) {
	db := newTestDB(t)
	m, err := userserver.New(db, user.Config{})
	if err != nil {
		t.Fatal(err)
	}

	userCRUD := getHandler(m, "users")
	if userCRUD == nil {
		t.Fatal("userCRUD handler not found")
	}

	res, err := userCRUD.Create(user.User{Email: "test@example.com", Name: "Test User", Phone: "123456789"})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	u := res.(user.User)
	if u.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got '%s'", u.Email)
	}

	res2, err := userCRUD.Read(u.Id)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	u2 := res2.(user.User)
	if u2.Id != u.Id {
		t.Errorf("expected ID '%s', got '%s'", u.Id, u2.Id)
	}

	u2.Name = "Updated Name"
	res3, err := userCRUD.Update(u2)
	if err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}
	u3 := res3.(user.User)
	if u3.Name != "Updated Name" {
		t.Errorf("expected Name 'Updated Name', got '%s'", u3.Name)
	}
}

func testAuth(t *testing.T) {
	db := newTestDB(t)
	m, err := userserver.New(db, user.Config{})
	if err != nil {
		t.Fatal(err)
	}

	userCRUD := getHandler(m, "users")
	res, err := userCRUD.Create(user.User{Email: "auth@example.com", Name: "Auth User"})
	if err != nil {
		t.Fatal(err)
	}
	u := res.(user.User)

	if err := m.SetPassword(u.Id, "password123"); err != nil {
		t.Fatalf("SetPassword failed: %v", err)
	}

	u2, err := m.Login("auth@example.com", "password123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if u2.Id != u.Id {
		t.Errorf("Login returned wrong user")
	}

	_, err = m.Login("auth@example.com", "wrongpass")
	if err != user.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}

	if err := m.VerifyPassword(u.Id, "password123"); err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
}

func testSessions(t *testing.T) {
	db := newTestDB(t)
	m, err := userserver.New(db, user.Config{TokenTTL: 3600})
	if err != nil {
		t.Fatal(err)
	}

	userCRUD := getHandler(m, "users")
	res, err := userCRUD.Create(user.User{Email: "sess@example.com", Name: "Sess User"})
	if err != nil {
		t.Fatal(err)
	}
	u := res.(user.User)

	sess, err := m.CreateSession(u.Id, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Get Session
	s2, err := m.GetSession(sess.Id)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if s2.UserId != u.Id {
		t.Errorf("Session user ID mismatch")
	}

	// Instant expire via SQL
	if err := db.RawExecutor().Exec("UPDATE session SET expires_at = 0 WHERE id = ?", sess.Id); err != nil {
		t.Fatalf("failed to expire session in DB: %v", err)
	}

	// Re-init to flush memory cache
	m, _ = userserver.New(db, user.Config{TokenTTL: 3600})

	_, err = m.GetSession(sess.Id)
	if err != user.ErrSessionExpired {
		t.Errorf("expected ErrSessionExpired, got %v", err)
	}
}

type MockProvider struct {
	NameVal         string
	ExchangeCodeVal *oauth2.Token
	UserInfoVal     user.OAuthUserInfo
}

func (m *MockProvider) Name() string                    { return m.NameVal }
func (m *MockProvider) AuthCodeURL(state string) string { return "http://mock/" + state }
func (m *MockProvider) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	return m.ExchangeCodeVal, nil
}
func (m *MockProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (user.OAuthUserInfo, error) {
	return m.UserInfoVal, nil
}

func testOAuth(t *testing.T) {
	db := newTestDB(t)
	mockP := &MockProvider{
		NameVal:         "mock",
		ExchangeCodeVal: &oauth2.Token{AccessToken: "mocktoken"},
		UserInfoVal:     user.OAuthUserInfo{ID: "mockid", Email: "mock@example.com", Name: "Mock User"},
	}

	cfg := user.Config{
		OAuthProviders: []user.OAuthProvider{mockP},
	}
	m, err := userserver.New(db, cfg)
	if err != nil {
		t.Fatal(err)
	}

	url, err := m.BeginOAuth("mock")
	if err != nil {
		t.Fatalf("BeginOAuth failed: %v", err)
	}
	if len(url) < 12 {
		t.Fatalf("invalid url: %s", url)
	}
	state := url[12:]

	ctx := &mock.Context{InPath: "/callback?state=" + state + "&code=mockcode"}
	u, isNew, err := m.CompleteOAuth("mock", ctx, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("CompleteOAuth failed: %v", err)
	}
	if !isNew {
		t.Errorf("expected isNew=true")
	}
	if u.Email != "mock@example.com" {
		t.Errorf("expected email mock@example.com, got %s", u.Email)
	}

	url2, _ := m.BeginOAuth("mock")
	state2 := url2[12:]
	ctx2 := &mock.Context{InPath: "/callback?state=" + state2 + "&code=mockcode"}

	u2, isNew2, err := m.CompleteOAuth("mock", ctx2, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("CompleteOAuth 2 failed: %v", err)
	}
	if isNew2 {
		t.Errorf("expected isNew=false")
	}
	if u2.Id != u.Id {
		t.Errorf("expected same user ID")
	}
}

func testLAN(t *testing.T) {
	db := newTestDB(t)
	m, err := userserver.New(db, user.Config{TrustProxy: true})
	if err != nil {
		t.Fatal(err)
	}

	userCRUD := getHandler(m, "users")
	res, err := userCRUD.Create(user.User{Email: "lan@example.com", Name: "LAN User"})
	if err != nil {
		t.Fatal(err)
	}
	u := res.(user.User)

	if err := m.RegisterLAN(u.Id, "12345678-5"); err != nil {
		t.Fatalf("RegisterLAN failed: %v", err)
	}

	if err := m.AssignLANIP(u.Id, "192.168.1.10", "Home"); err != nil {
		t.Fatalf("AssignLANIP failed: %v", err)
	}

	ctx := &mock.Context{}
	ctx.SetValue("RemoteAddr", "192.168.1.10:1234")

	u2, err := m.LoginLAN("12345678-5", ctx)
	if err != nil {
		t.Fatalf("LoginLAN failed: %v", err)
	}
	if u2.Id != u.Id {
		t.Errorf("expected same user ID")
	}

	_, err = m.LoginLAN("123", ctx)
	if err != user.ErrInvalidRUT {
		t.Errorf("expected ErrInvalidRUT, got %v", err)
	}

	ctxW := &mock.Context{}
	ctxW.SetValue("RemoteAddr", "10.0.0.1:1234")
	_, err = m.LoginLAN("12345678-5", ctxW)
	if err != user.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials for wrong IP, got %v", err)
	}

	ctxP := &mock.Context{}
	ctxP.SetValue("RemoteAddr", "10.0.0.1:1234")
	ctxP.SetHeader("X-Forwarded-For", "192.168.1.10")
	u3, err := m.LoginLAN("12345678-5", ctxP)
	if err != nil {
		t.Fatalf("LoginLAN with proxy failed: %v", err)
	}
	if u3.Id != u.Id {
		t.Errorf("expected same user ID")
	}

	if err := m.RevokeLANIP(u.Id, "192.168.1.10"); err != nil {
		t.Fatalf("RevokeLANIP failed: %v", err)
	}

	_, err = m.LoginLAN("12345678-5", ctxP)
	if err != user.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials after revoke, got %v", err)
	}

	if err := m.UnregisterLAN(u.Id); err != nil {
		t.Fatalf("UnregisterLAN failed: %v", err)
	}

	_, err = m.LoginLAN("12345678-5", ctxP)
	if err != user.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials after unregister, got %v", err)
	}
}

func setupModule(t *testing.T) *userserver.Module {
	db := newTestDB(t)
	m, _ := userserver.New(db, user.Config{})
	return m
}
