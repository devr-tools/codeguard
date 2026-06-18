package security_test

import (
	"errors"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/trust"
)

func TestGuardConfigCommandRefusesByDefault(t *testing.T) {
	trust.Set(trust.Policy{})
	t.Cleanup(trust.ResetFromEnv)

	err := trust.GuardConfigCommand("language security command", "/bin/echo")
	if err == nil {
		t.Fatal("expected command execution to be refused under the default policy")
	}
	var disabled trust.ErrConfigCommandsDisabled
	if !errors.As(err, &disabled) {
		t.Fatalf("expected ErrConfigCommandsDisabled, got %T: %v", err, err)
	}
	if disabled.Command != "/bin/echo" {
		t.Fatalf("error did not capture command, got %q", disabled.Command)
	}
}

func TestGuardConfigCommandAllowedWhenOptedIn(t *testing.T) {
	trust.Set(trust.Policy{AllowConfigCommands: true})
	t.Cleanup(trust.ResetFromEnv)

	if err := trust.GuardConfigCommand("ctx", "/bin/echo"); err != nil {
		t.Fatalf("expected command to be permitted, got %v", err)
	}
}

func TestFromEnvParsesTruthyValues(t *testing.T) {
	for _, value := range []string{"1", "true", "TRUE", "yes", "on"} {
		t.Setenv(trust.AllowConfigCommandsEnv, value)
		if !trust.FromEnv().AllowConfigCommands {
			t.Fatalf("value %q should enable config commands", value)
		}
	}
	for _, value := range []string{"", "0", "false", "no", "off", "maybe"} {
		t.Setenv(trust.AllowConfigCommandsEnv, value)
		if trust.FromEnv().AllowConfigCommands {
			t.Fatalf("value %q should not enable config commands", value)
		}
	}
}
