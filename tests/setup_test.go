package tests

import (
	"testing"

	"github.com/tinywasm/user"
)

func RunSharedTests(t *testing.T) {
	t.Run("TestModules", testModules)
}

func testModules(t *testing.T) {
	modules := user.UIModules()
	expected := []string{"login", "register", "profile", "lan", "oauth/callback"}
	for _, name := range expected {
		found := false
		for _, mod := range modules {
			if h, ok := mod.(interface{ HandlerName() string }); ok && h.HandlerName() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("UIModules: missing handler %q", name)
		}
	}
}
