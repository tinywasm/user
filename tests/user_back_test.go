//go:build !wasm

package tests

import (
	"testing"
)

func TestUserBackend(t *testing.T) {
	RunUserTests(t)
}
