//go:build !wasm

package tests

import (
	"strings"
	"testing"

	twctx "github.com/tinywasm/context"
	"github.com/tinywasm/form"
	"github.com/tinywasm/json"
	"github.com/tinywasm/mcp"
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
	"github.com/tinywasm/router/mock"
	"github.com/tinywasm/user"
	"github.com/tinywasm/user/server"
	"golang.org/x/crypto/bcrypt"
)

func TestProductionWiring(t *testing.T) {
	userserver.PasswordHashCost = bcrypt.MinCost

	t.Run("Widgets", testWidgets)
	t.Run("ConsumerViewSSR", testConsumerViewSSR)
	t.Run("Bootstrap", testBootstrap)
	t.Run("MountAPI", testMountAPI)
	t.Run("MeToolPermissions", testMeToolPermissions)
}

// testConsumerViewSSR plays the role of a consumer app building its own
// login page over user.LoginData and posting to user.PathLogin: the
// rendered HTML must expose the field names the handler expects.
func testConsumerViewSSR(t *testing.T) {
	f, err := form.New("login", &user.LoginData{})
	if err != nil {
		t.Fatalf("form.New failed: %v", err)
	}
	f.SetSSR(true)

	html := f.String()

	if !strings.Contains(html, "name='email'") {
		t.Errorf("consumer-view HTML missing email field: %s", html)
	}
	if !strings.Contains(html, "name='password'") {
		t.Errorf("consumer-view HTML missing password field: %s", html)
	}
}

func testWidgets(t *testing.T) {
	cases := []struct {
		name     string
		data     model.Fielder
		expected int
	}{
		{"LoginData", &user.LoginData{}, 2},
		{"RegisterData", &user.RegisterData{}, 4},
		{"ProfileData", &user.ProfileData{}, 2},
		{"PasswordData", &user.PasswordData{}, 3},
	}

	for _, tc := range cases {
		_, err := form.New("test", tc.data)
		if err != nil {
			t.Fatalf("%s: form.New failed: %v", tc.name, err)
		}
		schema := tc.data.Schema()
		count := 0
		for _, field := range schema {
			if field.Widget != nil {
				count++
			}
		}
		if count != tc.expected {
			t.Errorf("%s: expected %d widgets, got %d", tc.name, tc.expected, count)
		}
	}
}

func testBootstrap(t *testing.T) {
	db := newTestDB(t)
	m, err := userserver.New(db, user.Config{})
	if err != nil {
		t.Fatal(err)
	}

	email := "admin@test.com"
	pass := "password123"

	if err := m.Bootstrap(email, pass); err != nil {
		t.Fatalf("Bootstrap failed: %v", err)
	}

	u, err := m.Login(email, pass)
	if err != nil {
		t.Fatalf("Admin login failed: %v", err)
	}

	ok := m.Can(u.ID, "any_resource", "any_action")
	if !ok {
		t.Errorf("Admin should have wildcard permissions")
	}

	if err := m.Bootstrap(email, pass); err != nil {
		t.Fatalf("Bootstrap second call failed: %v", err)
	}

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

	email := "user@test.com"
	pass := "password123"
	if err := m.Bootstrap(email, pass); err != nil {
		t.Fatal(err)
	}

	r := &mock.Router{}
	m.MountAPI(r)

	// 1. POST /login (success) - urlencoded
	ctxPost := &mock.Context{
		InMethod: "POST",
		InPath:   user.PathLogin,
		InBody:   []byte("email=" + email + "&password=" + pass),
	}
	ctxPost.SetHeader("Content-Type", "application/x-www-form-urlencoded")

	r.Invoke("POST", user.PathLogin, ctxPost)
	if ctxPost.Status != 302 {
		t.Errorf("POST /login (success) status: %d, body: %s", ctxPost.Status, string(ctxPost.ResponseBody()))
	}
	if ctxPost.GetHeader("Location") != user.PathAfterLogin {
		t.Errorf("POST /login (success) redirect: %s", ctxPost.GetHeader("Location"))
	}
	c, ok := ctxPost.Cookie("test_session")
	if !ok || c.Value == "" {
		t.Errorf("POST /login (success) cookie missing or empty")
	}

	// 2. POST /login (failure) - urlencoded
	ctxFail := &mock.Context{
		InMethod: "POST",
		InPath:   user.PathLogin,
		InBody:   []byte("email=" + email + "&password=wrong"),
	}
	ctxFail.SetHeader("Content-Type", "application/x-www-form-urlencoded")

	r.Invoke("POST", user.PathLogin, ctxFail)
	if ctxFail.Status != 401 {
		t.Errorf("POST /login (failure) status expected 401, got: %d", ctxFail.Status)
	}
	if !strings.Contains(string(ctxFail.ResponseBody()), "access denied") {
		t.Errorf("POST /login (failure) missing error message: %s", string(ctxFail.ResponseBody()))
	}

	// 3. POST /login (success) - JSON
	ctxJson := &mock.Context{
		InMethod: "POST",
		InPath:   user.PathLogin,
	}
	ctxJson.SetHeader("Content-Type", "application/json")
	loginData := &user.LoginData{Email: email, Password: pass}
	var postBody string
	json.Encode(loginData, &postBody)
	ctxJson.InBody = []byte(postBody)

	r.Invoke("POST", user.PathLogin, ctxJson)
	if ctxJson.Status != 302 {
		t.Errorf("POST /login (JSON) status: %d", ctxJson.Status)
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

	if err := m.Bootstrap(email, pass); err != nil {
		t.Fatal(err)
	}

	uObj, err := m.GetUserByEmail(email)
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

	if len(profile.Permissions) != 1 || profile.Permissions[0] != "*:*" {
		t.Errorf("expected permission '*:*', got %v", profile.Permissions)
	}
}
