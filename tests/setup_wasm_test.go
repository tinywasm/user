//go:build wasm

package tests

import (
	"testing"
	"github.com/tinywasm/user/ui"
)

type wasmModule struct{}
func (m wasmModule) UIModules() []any { return userui.UIModules() }

func setupModule(t *testing.T) wasmModule {
    return wasmModule{}
}
