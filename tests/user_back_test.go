//go:build !wasm

package tests

import (
	"strings"
	"testing"

	"github.com/tinywasm/user"
)

func TestUserBackend(t *testing.T) {
	RunUserTests(t)
	t.Run("TestModulesSSR", testModulesSSR)
}

func testModulesSSR(t *testing.T) {
	if out := user.LoginModule.RenderHTML(); !strings.Contains(out, "<form") {
		t.Errorf("LoginModule.RenderHTML() should contain <form")
	}
	if out := user.RegisterModule.RenderHTML(); !strings.Contains(out, "<form") {
		t.Errorf("RegisterModule.RenderHTML() should contain <form")
	}
	if out := user.ProfileModule.RenderHTML(); !strings.Contains(out, "<form") {
		t.Errorf("ProfileModule.RenderHTML() should contain <form")
	}
	if out := user.LANModule.RenderHTML(); !strings.Contains(out, "<table") {
		t.Errorf("LANModule.RenderHTML() should contain <table")
	}
}
