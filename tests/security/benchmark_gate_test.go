package security_test

import (
	"context"
	"errors"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/runner/benchregression"
	"github.com/devr-tools/codeguard/internal/codeguard/trust"
)

// Even though the go executable is fixed, go test compiles and runs code from
// the scanned repository, so repository configuration must not be able to
// trigger it without an explicit operator opt-in.
func TestBenchmarksRefusedByDefault(t *testing.T) {
	trust.Set(trust.Policy{})
	t.Cleanup(trust.ResetFromEnv)

	_, err := benchregression.RunBenchmarks(context.Background(), t.TempDir(), []string{"."})
	if err == nil {
		t.Fatal("expected benchmark execution to be refused under the default policy")
	}
	var disabled trust.ErrConfigCommandsDisabled
	if !errors.As(err, &disabled) {
		t.Fatalf("expected ErrConfigCommandsDisabled, got %T: %v", err, err)
	}
	if disabled.Command != "go test -bench" {
		t.Fatalf("error did not capture the refused command, got %q", disabled.Command)
	}
}

func TestBenchmarksPassTrustGateWhenOptedIn(t *testing.T) {
	trust.Set(trust.Policy{AllowConfigCommands: true})
	t.Cleanup(trust.ResetFromEnv)

	_, err := benchregression.RunBenchmarks(context.Background(), t.TempDir(), []string{"."})
	var disabled trust.ErrConfigCommandsDisabled
	if errors.As(err, &disabled) {
		t.Fatal("benchmark execution should be permitted once AllowConfigCommands is set")
	}
}
