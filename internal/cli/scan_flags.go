package cli

import (
	"flag"

	"github.com/devr-tools/codeguard/internal/codeguard/trust"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// scanRunFlags bundles the flag values shared by commands that execute a scan
// against a config: the config path, scan mode, diff base ref, profile, and the
// trust opt-ins that unlock config-driven command execution and custom AI
// endpoints (disabled by default; see internal/codeguard/trust).
type scanRunFlags struct {
	configPath       *string
	mode             *string
	baseRef          *string
	profile          *string
	allowCommands    *bool
	allowAIEndpoints *bool
}

func registerScanRunFlags(fs *flag.FlagSet) scanRunFlags {
	return scanRunFlags{
		configPath:       fs.String("config", service.DefaultConfigPath(), "config file or directory path"),
		mode:             fs.String("mode", string(service.ScanModeFull), "scan mode: full or diff"),
		baseRef:          fs.String("base-ref", "main", "base branch/ref for diff mode"),
		profile:          fs.String("profile", "", "optional policy profile override"),
		allowCommands:    fs.Bool("allow-config-commands", false, "trust the repository config to run shell commands (off by default; only enable for trusted repos)"),
		allowAIEndpoints: fs.Bool("allow-config-ai-endpoints", false, "trust the repository config to set non-allowlisted AI provider base URLs (off by default)"),
	}
}

// applyTrustPolicy merges the trust opt-in flags with any environment defaults
// and installs the resulting policy. A flag only ever widens trust; it never
// disables a capability the environment already enabled.
func (f scanRunFlags) applyTrustPolicy() {
	p := trust.FromEnv()
	if f.allowCommands != nil && *f.allowCommands {
		p.AllowConfigCommands = true
	}
	if f.allowAIEndpoints != nil && *f.allowAIEndpoints {
		p.AllowConfigAIEndpoints = true
	}
	trust.Set(p)
}
