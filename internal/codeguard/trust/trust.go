// Package trust centralizes codeguard's trust-boundary policy.
//
// codeguard frequently runs in CI against pull requests authored by untrusted
// contributors, yet its behavior is driven by configuration (codeguard.yaml and
// rule packs) that is checked into the repository and therefore controllable by
// those same untrusted authors. To avoid turning a code review tool into a
// remote-code-execution or credential-exfiltration vector, potentially
// dangerous, config-driven capabilities are DISABLED BY DEFAULT and must be
// explicitly enabled by the trusted operator.
//
// The trust anchor is the process environment / CLI flags (controlled by the
// workflow author or local developer), never the repository config itself.
package trust

import (
	"os"
	"strings"
	"sync"
)

const (
	// AllowConfigCommandsEnv enables execution of commands supplied by the
	// repository configuration (language/license/autofix commands and the
	// "command" AI provider / nlrule / semantic runtimes).
	AllowConfigCommandsEnv = "CODEGUARD_ALLOW_CONFIG_COMMANDS"
	// AllowConfigAIEndpointsEnv enables AI provider base URLs that are not on
	// the built-in public-provider allowlist, and relaxes the SSRF dial guard
	// so self-hosted/internal endpoints can be reached.
	AllowConfigAIEndpointsEnv = "CODEGUARD_ALLOW_CONFIG_AI_ENDPOINTS"
)

// Policy captures which untrusted, config-driven capabilities are permitted.
type Policy struct {
	// AllowConfigCommands permits execution of config-supplied commands.
	AllowConfigCommands bool
	// AllowConfigAIEndpoints permits non-allowlisted AI provider base URLs.
	AllowConfigAIEndpoints bool
}

var (
	mu      sync.RWMutex
	current = FromEnv()
)

// FromEnv derives a Policy from environment variables. Unset/false by default.
func FromEnv() Policy {
	return Policy{
		AllowConfigCommands:    envEnabled(AllowConfigCommandsEnv),
		AllowConfigAIEndpoints: envEnabled(AllowConfigAIEndpointsEnv),
	}
}

func envEnabled(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// Current returns the active trust policy.
func Current() Policy {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Set replaces the active trust policy. Used by CLI flag wiring and tests.
func Set(p Policy) {
	mu.Lock()
	defer mu.Unlock()
	current = p
}

// ResetFromEnv re-reads the policy from the environment. Used by tests.
func ResetFromEnv() {
	Set(FromEnv())
}

// AllowConfigCommands reports whether config-supplied command execution is
// permitted under the active policy.
func AllowConfigCommands() bool { return Current().AllowConfigCommands }

// AllowConfigAIEndpoints reports whether non-allowlisted AI provider base URLs
// are permitted under the active policy.
func AllowConfigAIEndpoints() bool { return Current().AllowConfigAIEndpoints }
