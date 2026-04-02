//go:build !wasm

package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tinywasm/orm"
	"github.com/tinywasm/sqlite"
	"github.com/tinywasm/user"
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
	user.PasswordHashCost = bcrypt.MinCost
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
	m, err := user.New(db, user.Config{
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
	_ = m.SetPassword(u.ID, "password123")
	logged, err := m.Login("jwt@test.com", "password123")
	if err != nil {
		t.Fatal("login failed:", err)
	}

	// Generar JWT como lo haría SetCookie
	token, err := user.GenerateJWT(secret, logged.ID, 86400)
	if err != nil {
		t.Fatal(err)
	}

	// Request con JWT en cookie → debe autenticar
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rec := httptest.NewRecorder()
	var ctxUser *user.User
	m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxUser, _ = m.FromContext(r.Context())
	})).ServeHTTP(rec, req)
	if ctxUser == nil || ctxUser.ID != logged.ID {
		t.Errorf("JWT middleware: expected user %s, got %v", logged.ID, ctxUser)
	}

	// Token inválido → 401
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.AddCookie(&http.Cookie{Name: "session", Value: "invalid.jwt.token"})
	rec2 := httptest.NewRecorder()
	called := false
	m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})).ServeHTTP(rec2, req2)
	if called || rec2.Code != http.StatusUnauthorized {
		t.Errorf("want 401 for invalid JWT, got %d (called=%v)", rec2.Code, called)
	}
}

func testInit(t *testing.T) {
	db := newTestDB(t)
	cfg := user.Config{
		CookieName: "test_session",
		TokenTTL:   3600,
	}
	m, err := user.New(db, cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	_ = m // to be used later
}

func getHandler(m *user.Module, name string) interface {
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
	m, err := user.New(db, user.Config{})
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

	res2, err := userCRUD.Read(u.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	u2 := res2.(user.User)
	if u2.ID != u.ID {
		t.Errorf("expected ID '%s', got '%s'", u.ID, u2.ID)
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
	m, err := user.New(db, user.Config{})
	if err != nil {
		t.Fatal(err)
	}

	userCRUD := getHandler(m, "users")
	res, err := userCRUD.Create(user.User{Email: "auth@example.com", Name: "Auth User"})
	if err != nil {
		t.Fatal(err)
	}
	u := res.(user.User)

	if err := m.SetPassword(u.ID, "password123"); err != nil {
		t.Fatalf("SetPassword failed: %v", err)
	}

	u2, err := m.Login("auth@example.com", "password123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if u2.ID != u.ID {
		t.Errorf("Login returned wrong user")
	}

	_, err = m.Login("auth@example.com", "wrongpass")
	if err != user.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}

	if err := m.VerifyPassword(u.ID, "password123"); err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
}

func testSessions(t *testing.T) {
	db := newTestDB(t)
	m, err := user.New(db, user.Config{TokenTTL: 3600})
	if err != nil {
		t.Fatal(err)
	}

	userCRUD := getHandler(m, "users")
	res, err := userCRUD.Create(user.User{Email: "sess@example.com", Name: "Sess User"})
	if err != nil {
		t.Fatal(err)
	}
	u := res.(user.User)

	sess, err := m.CreateSession(u.ID, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Get Session
	s2, err := m.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if s2.UserID != u.ID {
		t.Errorf("Session user ID mismatch")
	}

	// Instant expire via SQL
	if err := db.RawExecutor().Exec("UPDATE session SET expires_at = 0 WHERE id = ?", sess.ID); err != nil {
		t.Fatalf("failed to expire session in DB: %v", err)
	}

	// Re-init to flush memory cache
	m, _ = user.New(db, user.Config{TokenTTL: 3600})

	_, err = m.GetSession(sess.ID)
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
	m, err := user.New(db, cfg)
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

	req, _ := http.NewRequest("GET", "/callback?state="+state+"&code=mockcode", nil)
	u, isNew, err := m.CompleteOAuth("mock", req, "127.0.0.1", "TestAgent")
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
	req2, _ := http.NewRequest("GET", "/callback?state="+state2+"&code=mockcode", nil)

	u2, isNew2, err := m.CompleteOAuth("mock", req2, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("CompleteOAuth 2 failed: %v", err)
	}
	if isNew2 {
		t.Errorf("expected isNew=false")
	}
	if u2.ID != u.ID {
		t.Errorf("expected same user ID")
	}
}

func testLAN(t *testing.T) {
	db := newTestDB(t)
	m, err := user.New(db, user.Config{TrustProxy: true})
	if err != nil {
		t.Fatal(err)
	}

	userCRUD := getHandler(m, "users")
	res, err := userCRUD.Create(user.User{Email: "lan@example.com", Name: "LAN User"})
	if err != nil {
		t.Fatal(err)
	}
	u := res.(user.User)

	if err := m.RegisterLAN(u.ID, "12345678-5"); err != nil {
		t.Fatalf("RegisterLAN failed: %v", err)
	}

	if err := m.AssignLANIP(u.ID, "192.168.1.10", "Home"); err != nil {
		t.Fatalf("AssignLANIP failed: %v", err)
	}

	req, _ := http.NewRequest("POST", "/lan", nil)
	req.RemoteAddr = "192.168.1.10:1234"

	u2, err := m.LoginLAN("12345678-5", req)
	if err != nil {
		t.Fatalf("LoginLAN failed: %v", err)
	}
	if u2.ID != u.ID {
		t.Errorf("expected same user ID")
	}

	_, err = m.LoginLAN("123", req)
	if err != user.ErrInvalidRUT {
		t.Errorf("expected ErrInvalidRUT, got %v", err)
	}

	req.RemoteAddr = "10.0.0.1:1234"
	_, err = m.LoginLAN("12345678-5", req)
	if err != user.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials for wrong IP, got %v", err)
	}

	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "192.168.1.10")
	u3, err := m.LoginLAN("12345678-5", req)
	if err != nil {
		t.Fatalf("LoginLAN with proxy failed: %v", err)
	}
	if u3.ID != u.ID {
		t.Errorf("expected same user ID")
	}

	if err := m.RevokeLANIP(u.ID, "192.168.1.10"); err != nil {
		t.Fatalf("RevokeLANIP failed: %v", err)
	}

	_, err = m.LoginLAN("12345678-5", req)
	if err != user.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials after revoke, got %v", err)
	}

	if err := m.UnregisterLAN(u.ID); err != nil {
		t.Fatalf("UnregisterLAN failed: %v", err)
	}

	_, err = m.LoginLAN("12345678-5", req)
	if err != user.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials after unregister, got %v", err)
	}
}
