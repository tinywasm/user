package tests

import (
	"testing"

	"github.com/tinywasm/user"
)

func RunSharedTests(t *testing.T) {
	t.Run("TestModules", testModules)
}

func testModules(t *testing.T) {
	if user.LoginModule.HandlerName() != "login" {
		t.Errorf("expected handler name login, got %s", user.LoginModule.HandlerName())
	}
	// Check that we can at least access the module and it has basic properties
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
