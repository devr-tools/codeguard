package cli

import (
	"flag"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// scanRunFlags bundles the flag values shared by commands that execute a scan
// against a config: the config path, scan mode, diff base ref, and profile.
type scanRunFlags struct {
	configPath *string
	mode       *string
	baseRef    *string
	profile    *string
}

func registerScanRunFlags(fs *flag.FlagSet) scanRunFlags {
	return scanRunFlags{
		configPath: fs.String("config", service.DefaultConfigPath(), "config file or directory path"),
		mode:       fs.String("mode", string(service.ScanModeFull), "scan mode: full or diff"),
		baseRef:    fs.String("base-ref", "main", "base branch/ref for diff mode"),
		profile:    fs.String("profile", "", "optional policy profile override"),
	}
}
