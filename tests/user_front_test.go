//go:build wasm

package tests

import (
	"testing"
)

func TestUserFrontend(t *testing.T) {
	// Frontend-specific tests would go here.
	// Since UI modules were removed from this library's responsibility,
	// frontend tests that depend on them are removed or adapted in the consumer.
}
