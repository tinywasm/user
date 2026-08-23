//go:build !wasm

package tests

import (
	"testing"

	"github.com/tinywasm/router/mock"
	"github.com/tinywasm/user"
	"github.com/tinywasm/user/authority"
	"github.com/tinywasm/user/oauth2"
	"github.com/tinywasm/user/oauth2/provider/google"
)

func TestOAuthRoutesMatchExportedPaths(t *testing.T) {
	db := newTestDB(t)
	m, err := authority.New(db, user.Config{IDs: testIDs})
	if err != nil {
		t.Fatal(err)
	}

	gProv := &google.GoogleProvider{}
	m.Enable(oauth2.New(m, m, m, []user.OAuthProvider{gProv}))

	r := &mock.Router{}
	m.MountAPI(r)

	want := map[string]bool{
		user.PathOAuthStart(google.ProviderName):    false,
		user.PathOAuthCallback(google.ProviderName): false,
	}
	for _, info := range r.Routes() {
		if _, ok := want[info.Path]; ok {
			want[info.Path] = true
			if !info.IsPublic() {
				t.Errorf("ruta OAuth %s %s debe ser .Public()", info.Method, info.Path)
			}
		}
	}
	for path, found := range want {
		if !found {
			t.Errorf("la ruta exportada %q no fue registrada por Mount", path)
		}
	}

	if got := user.PathOAuthStart("google"); got != "/oauth/google" {
		t.Errorf("PathOAuthStart cambio: %q", got)
	}
	if got := user.PathOAuthCallback("google"); got != "/oauth/callback/google" {
		t.Errorf("PathOAuthCallback cambio: %q", got)
	}
}
