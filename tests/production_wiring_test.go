//go:build !wasm

package tests

import (
	"strings"
	"testing"

	twctx "github.com/tinywasm/context"
	"github.com/tinywasm/json"
	"github.com/tinywasm/mcp"
	"github.com/tinywasm/router"
	"github.com/tinywasm/router/mock"
	"github.com/tinywasm/user"
	"github.com/tinywasm/user/server"
	"golang.org/x/crypto/bcrypt"
)

func TestProductionWiring(t *testing.T) {
	userserver.PasswordHashCost = bcrypt.MinCost

	t.Run("Bootstrap", testBootstrap)
	t.Run("MountAPI", testMountAPI)
	t.Run("MeToolPermissions", testMeToolPermissions)
}

func testBootstrap(t *testing.T) {
	db := newTestDB(t)
	m, err := userserver.New(db, user.Config{})
	if err != nil {
		t.Fatal(err)
	}

	email := "admin@test.com"
	pass := "password123"

	// 1. Bootstrap fresh DB
	if err := m.Bootstrap(email, pass); err != nil {
		t.Fatalf("Bootstrap failed: %v", err)
	}

	// 2. Verify admin exists and can log in
	u, err := m.Login(email, pass)
	if err != nil {
		t.Fatalf("Admin login failed: %v", err)
	}

	// 3. Verify wildcard permissions
	ok := m.Can(u.ID, "any_resource", "any_action")
	if !ok {
		t.Errorf("Admin should have wildcard permissions")
	}

	// 4. Idempotency check: second call should be no-op
	if err := m.Bootstrap(email, pass); err != nil {
		t.Fatalf("Bootstrap second call failed: %v", err)
	}

	// 5. Empty credentials check
	db2 := newTestDB(t)
	m2, _ := userserver.New(db2, user.Config{})
	if err := m2.Bootstrap("", ""); err == nil {
		t.Errorf("Bootstrap with empty credentials on empty DB should fail")
	}
}

func testMountAPI(t *testing.T) {
	db := newTestDB(t)
	m, err := userserver.New(db, user.Config{
		CookieName: "test_session",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Seed a user
	email := "user@test.com"
	pass := "password123"
	if err := m.Bootstrap(email, pass); err != nil {
		t.Fatal(err)
	}

	r := &mock.Router{}
	m.MountAPI(r)

	// 1. GET /login
	ctxGet := &mock.Context{InMethod: "GET", InPath: user.PathLogin}
	r.Invoke("GET", user.PathLogin, ctxGet)
	if ctxGet.Status != 0 && ctxGet.Status != 200 {
		t.Errorf("GET /login status: %d", ctxGet.Status)
	}

	// 2. POST /login (success)
	ctxPost := &mock.Context{InMethod: "POST", InPath: user.PathLogin}
	loginData := &user.LoginData{Email: email, Password: pass}
	var postBody string
	json.Encode(loginData, &postBody)
	ctxPost.InBody = []byte(postBody)

	r.Invoke("POST", user.PathLogin, ctxPost)
	if ctxPost.Status != 302 {
		t.Errorf("POST /login (success) status: %d", ctxPost.Status)
	}
	if ctxPost.GetHeader("Location") != user.PathAfterLogin {
		t.Errorf("POST /login (success) redirect: %s", ctxPost.GetHeader("Location"))
	}
	c, ok := ctxPost.Cookie("test_session")
	if !ok || c.Value == "" {
		t.Errorf("POST /login (success) cookie missing or empty")
	}

	// 3. POST /login (failure)
	ctxFail := &mock.Context{InMethod: "POST", InPath: user.PathLogin}
	loginDataFail := &user.LoginData{Email: email, Password: "wrong"}
	var postBodyFail string
	json.Encode(loginDataFail, &postBodyFail)
	ctxFail.InBody = []byte(postBodyFail)

	r.Invoke("POST", user.PathLogin, ctxFail)
	// Should stay on login page or re-render (not redirect)
	if ctxFail.Status == 302 {
		t.Errorf("POST /login (failure) should not redirect")
	}
	if !strings.Contains(string(ctxFail.ResponseBody()), "access denied") {
		t.Errorf("POST /login (failure) missing error message: %s", string(ctxFail.ResponseBody()))
	}

	// 4. POST /logout
	sessID := c.Value

	ctxLogout := &mock.Context{InMethod: "POST", InPath: user.PathLogout}
	ctxLogout.SetCookie(router.Cookie{Name: "test_session", Value: sessID})
	r.Invoke("POST", user.PathLogout, ctxLogout)
	if ctxLogout.Status != 302 {
		t.Errorf("POST /logout status: %d", ctxLogout.Status)
	}
	if ctxLogout.GetHeader("Location") != user.PathLogin {
		t.Errorf("POST /logout redirect: %s", ctxLogout.GetHeader("Location"))
	}
	logoutCookie, ok := ctxLogout.Cookie("test_session")
	if !ok || logoutCookie.Value != "" {
		t.Errorf("POST /logout cookie not cleared: %+v", logoutCookie)
	}
}

func testMeToolPermissions(t *testing.T) {
	db := newTestDB(t)
	m, _ := userserver.New(db, user.Config{})

	email := "tools@test.com"
	pass := "password123"

	// Create user via Bootstrap
	if err := m.Bootstrap(email, pass); err != nil {
		t.Fatal(err)
	}

	uObj, err := m.ExportGetUserByEmail(email)
	if err != nil {
		t.Fatal(err)
	}

	tools := m.Tools()
	var meTool *mcp.Tool
	for _, tool := range tools {
		if tool.Name == "me" {
			meTool = &tool
			break
		}
	}
	if meTool == nil {
		t.Fatal("me tool not found")
	}

	ctx := twctx.Background()
	ctx.Set(mcp.CtxKeyUserID, uObj.ID)

	res, err := meTool.Execute(ctx, mcp.Request{})
	if err != nil {
		t.Fatalf("me tool execution failed: %v", err)
	}

	var profile user.ProfileDTO
	if err := json.Decode(res.Content, &profile); err != nil {
		t.Fatalf("failed to decode profile: %v", err)
	}

	// From Bootstrap we expect wildcard perm
	if len(profile.Permissions) != 1 || profile.Permissions[0] != "*:*" {
		t.Errorf("expected permission '*:*', got %v", profile.Permissions)
	}
}
