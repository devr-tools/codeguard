package security_test

import (
	"context"
	"errors"
	"testing"

	govulncheckrunner "github.com/devr-tools/codeguard/internal/codeguard/runner/govulncheck"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
	"github.com/devr-tools/codeguard/internal/codeguard/trust"
)

// A config-supplied override of govulncheck_command must not execute under the
// default (deny) trust policy: it is an arbitrary binary from a possibly
// untrusted pull request. This is the regression guard for the RCE that existed
// when the govulncheck runner exec'd the config command without the trust gate.
func TestGovulncheckOverrideRefusedByDefault(t *testing.T) {
	trust.Set(trust.Policy{})
	t.Cleanup(trust.ResetFromEnv)

	_, err := govulncheckrunner.Run(context.Background(), t.TempDir(), "/bin/sh", runnersupport.Context{})
	if err == nil {
		t.Fatal("expected config-supplied govulncheck command to be refused under the default policy")
	}
	var disabled trust.ErrConfigCommandsDisabled
	if !errors.As(err, &disabled) {
		t.Fatalf("expected ErrConfigCommandsDisabled, got %T: %v", err, err)
	}
	if disabled.Command != "/bin/sh" {
		t.Fatalf("error did not capture the refused command, got %q", disabled.Command)
	}
}

// The built-in default binary (empty or "govulncheck") must remain exempt so the
// default "auto" mode keeps working without --allow-config-commands. It may fail
// for other reasons (binary absent), but never with the trust-gate error.
func TestGovulncheckDefaultCommandNotGated(t *testing.T) {
	trust.Set(trust.Policy{})
	t.Cleanup(trust.ResetFromEnv)

	for _, cmd := range []string{"", "govulncheck"} {
		_, err := govulncheckrunner.Run(context.Background(), t.TempDir(), cmd, runnersupport.Context{})
		var disabled trust.ErrConfigCommandsDisabled
		if errors.As(err, &disabled) {
			t.Fatalf("default command %q should not be gated by the trust policy", cmd)
		}
	}
}

// With the operator opt-in, a config-supplied command passes the gate (it may
// still fail to execute, but not with the trust-gate error).
func TestGovulncheckOverrideAllowedWhenOptedIn(t *testing.T) {
	trust.Set(trust.Policy{AllowConfigCommands: true})
	t.Cleanup(trust.ResetFromEnv)

	_, err := govulncheckrunner.Run(context.Background(), t.TempDir(), "/bin/sh", runnersupport.Context{})
	var disabled trust.ErrConfigCommandsDisabled
	if errors.As(err, &disabled) {
		t.Fatal("config command should be permitted once AllowConfigCommands is set")
	}
}
