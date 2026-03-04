//go:build !wasm

package tests

import (
	"strings"
	"testing"

	"github.com/tinywasm/user"
	_ "modernc.org/sqlite"
)

func TestUserBackend(t *testing.T) {
	RunSharedTests(t)
	RunUserTests(t)
	t.Run("TestModulesSSR", testModulesSSR)
}

func testModulesSSR(t *testing.T) {
	modules := user.UIModules()
	for _, mod := range modules {
		if h, ok := mod.(interface{ HandlerName() string }); ok {
			switch h.HandlerName() {
			case "login":
				if out := mod.(interface{ RenderHTML() string }).RenderHTML(); !strings.Contains(out, "<form") {
					t.Errorf("login module should contain <form")
				}
			case "register":
				if out := mod.(interface{ RenderHTML() string }).RenderHTML(); !strings.Contains(out, "<form") {
					t.Errorf("register module should contain <form")
				}
			case "profile":
				if out := mod.(interface{ RenderHTML() string }).RenderHTML(); !strings.Contains(out, "<form") {
					t.Errorf("profile module should contain <form")
				}
			case "lan":
				if out := mod.(interface{ RenderHTML() string }).RenderHTML(); !strings.Contains(out, "<table") {
					t.Errorf("lan module should contain <table")
				}
			}
		}
	}
}
