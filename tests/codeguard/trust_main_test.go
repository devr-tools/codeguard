package codeguard_test

import (
	"os"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/trust"
)

// TestMain enables the trust opt-ins for this package. These tests exercise
// command-driven AI providers and local (127.0.0.1) AI endpoints, which are
// gated off by default by the trust policy. The secure default itself is
// covered by dedicated tests in the trust, runner/support, and config packages.
func TestMain(m *testing.M) {
	trust.Set(trust.Policy{AllowConfigCommands: true, AllowConfigAIEndpoints: true})
	os.Exit(m.Run())
}
