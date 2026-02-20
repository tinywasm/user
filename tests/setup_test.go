package tests

import (
	"context"
	"database/sql"
	"net/http"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tinywasm/user"
	"golang.org/x/oauth2"
)

type TestExecutor struct {
	*sql.DB
}

func (e *TestExecutor) Exec(query string, args ...any) error {
	_, err := e.DB.Exec(query, args...)
	return err
}

func (e *TestExecutor) Query(query string, args ...any) (user.Rows, error) {
	return e.DB.Query(query, args...)
}

func (e *TestExecutor) QueryRow(query string, args ...any) user.Scanner {
	return e.DB.QueryRow(query, args...)
}

func newTestDB(t *testing.T) *TestExecutor {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	return &TestExecutor{db}
}

func RunUserTests(t *testing.T) {
	t.Run("TestInit", testInit)
	t.Run("TestCRUD", testCRUD)
	t.Run("TestAuth", testAuth)
	t.Run("TestSessions", testSessions)
	t.Run("TestOAuth", testOAuth)
	t.Run("TestLAN", testLAN)
	t.Run("TestModules", testModules)
}

func testInit(t *testing.T) {
	db := newTestDB(t)
	cfg := user.Config{
		SessionCookieName: "test_session",
		SessionTTL:        3600,
	}
	if err := user.Init(db, cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if user.SessionCookieName() != "test_session" {
		t.Errorf("expected session cookie name 'test_session', got '%s'", user.SessionCookieName())
	}
}

func testCRUD(t *testing.T) {
	db := newTestDB(t)
	user.Init(db, user.Config{})

	// Test CreateUser
	u, err := user.CreateUser("test@example.com", "Test User", "123456789")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if u.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got '%s'", u.Email)
	}

	// Test GetUser
	u2, err := user.GetUser(u.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if u2.ID != u.ID {
		t.Errorf("expected ID '%s', got '%s'", u.ID, u2.ID)
	}

	// Test UpdateUser
	if err := user.UpdateUser(u.ID, "Updated Name", "987654321"); err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}
	u3, err := user.GetUser(u.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if u3.Name != "Updated Name" {
		t.Errorf("expected Name 'Updated Name', got '%s'", u3.Name)
	}

	// Test Suspend/Reactivate
	if err := user.SuspendUser(u.ID); err != nil {
		t.Fatalf("SuspendUser failed: %v", err)
	}
	u4, err := user.GetUser(u.ID)
	if u4.Status != "suspended" {
		t.Errorf("expected Status 'suspended', got '%s'", u4.Status)
	}

	if err := user.ReactivateUser(u.ID); err != nil {
		t.Fatalf("ReactivateUser failed: %v", err)
	}
	u5, err := user.GetUser(u.ID)
	if u5.Status != "active" {
		t.Errorf("expected Status 'active', got '%s'", u5.Status)
	}
}

func testAuth(t *testing.T) {
	db := newTestDB(t)
	user.Init(db, user.Config{})

	u, err := user.CreateUser("auth@example.com", "Auth User", "")
	if err != nil {
		t.Fatal(err)
	}

	// Set Password
	if err := user.SetPassword(u.ID, "password123"); err != nil {
		t.Fatalf("SetPassword failed: %v", err)
	}

	// Login
	u2, err := user.Login("auth@example.com", "password123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if u2.ID != u.ID {
		t.Errorf("Login returned wrong user")
	}

	// Wrong password
	_, err = user.Login("auth@example.com", "wrongpass")
	if err != user.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}

	// Verify Password
	if err := user.VerifyPassword(u.ID, "password123"); err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
}

func testSessions(t *testing.T) {
	db := newTestDB(t)
	user.Init(db, user.Config{SessionTTL: 1}) // 1 sec TTL

	u, err := user.CreateUser("sess@example.com", "Sess User", "")
	if err != nil {
		t.Fatal(err)
	}

	sess, err := user.CreateSession(u.ID, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Get Session
	s2, err := user.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if s2.UserID != u.ID {
		t.Errorf("Session user ID mismatch")
	}

	// Expire
	time.Sleep(2 * time.Second)
	_, err = user.GetSession(sess.ID)
	if err != user.ErrSessionExpired {
		t.Errorf("expected ErrSessionExpired, got %v", err)
	}
}

type MockProvider struct {
	NameVal         string
	ExchangeCodeVal *oauth2.Token
	UserInfoVal     user.OAuthUserInfo
}

func (m *MockProvider) Name() string { return m.NameVal }
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
	user.Init(db, cfg)

	// Begin OAuth
	url, err := user.BeginOAuth("mock")
	if err != nil {
		t.Fatalf("BeginOAuth failed: %v", err)
	}
	// Extract state from URL "http://mock/STATE"
	if len(url) < 12 {
		t.Fatalf("invalid url: %s", url)
	}
	state := url[12:]

	// Complete OAuth
	req, _ := http.NewRequest("GET", "/callback?state="+state+"&code=mockcode", nil)
	u, isNew, err := user.CompleteOAuth("mock", req, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("CompleteOAuth failed: %v", err)
	}
	if !isNew {
		t.Errorf("expected isNew=true")
	}
	if u.Email != "mock@example.com" {
		t.Errorf("expected email mock@example.com, got %s", u.Email)
	}

	// Second login should not be new
	url2, _ := user.BeginOAuth("mock")
	state2 := url2[12:]
	req2, _ := http.NewRequest("GET", "/callback?state="+state2+"&code=mockcode", nil)

	u2, isNew2, err := user.CompleteOAuth("mock", req2, "127.0.0.1", "TestAgent")
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
	user.Init(db, user.Config{TrustProxy: true})

	u, err := user.CreateUser("lan@example.com", "LAN User", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := user.RegisterLAN(u.ID, "12345678-5"); err != nil {
		t.Fatalf("RegisterLAN failed: %v", err)
	}

	if err := user.AssignLANIP(u.ID, "192.168.1.10", "Home"); err != nil {
		t.Fatalf("AssignLANIP failed: %v", err)
	}

	req, _ := http.NewRequest("POST", "/lan", nil)
	req.RemoteAddr = "192.168.1.10:1234"

	u2, err := user.LoginLAN("12345678-5", req)
	if err != nil {
		t.Fatalf("LoginLAN failed: %v", err)
	}
	if u2.ID != u.ID {
		t.Errorf("expected same user ID")
	}

	_, err = user.LoginLAN("123", req)
	if err != user.ErrInvalidRUT {
		t.Errorf("expected ErrInvalidRUT, got %v", err)
	}

	req.RemoteAddr = "10.0.0.1:1234"
	_, err = user.LoginLAN("12345678-5", req)
	if err != user.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials for wrong IP, got %v", err)
	}

	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "192.168.1.10")
	u3, err := user.LoginLAN("12345678-5", req)
	if err != nil {
		t.Fatalf("LoginLAN with proxy failed: %v", err)
	}
	if u3.ID != u.ID {
		t.Errorf("expected same user ID")
	}

	if err := user.RevokeLANIP(u.ID, "192.168.1.10"); err != nil {
		t.Fatalf("RevokeLANIP failed: %v", err)
	}

	_, err = user.LoginLAN("12345678-5", req)
	if err != user.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials after revoke, got %v", err)
	}

	if err := user.UnregisterLAN(u.ID); err != nil {
		t.Fatalf("UnregisterLAN failed: %v", err)
	}

	_, err = user.LoginLAN("12345678-5", req)
	if err != user.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials after unregister, got %v", err)
	}
}

func testModules(t *testing.T) {
	if user.LoginModule.HandlerName() != "login" {
		t.Errorf("expected handler name login, got %s", user.LoginModule.HandlerName())
	}
	data := user.LoginData{Email: "test@example.com", Password: "password123"}
	if err := user.LoginModule.ValidateData(0, &data); err != nil {
		t.Errorf("LoginModule.ValidateData failed: %v", err)
	}

	if user.RegisterModule.HandlerName() != "register" {
		t.Errorf("expected handler name register, got %s", user.RegisterModule.HandlerName())
	}

	if user.ProfileModule.HandlerName() != "profile" {
		t.Errorf("expected handler name profile, got %s", user.ProfileModule.HandlerName())
	}

	if user.LANModule.HandlerName() != "lan" {
		t.Errorf("expected handler name lan, got %s", user.LANModule.HandlerName())
	}

	if user.OAuthCallback.HandlerName() != "oauth/callback" {
		t.Errorf("expected handler name oauth/callback, got %s", user.OAuthCallback.HandlerName())
	}
}
