package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// promptString and loadConfigWithProfile live here; the command menu is
// rendered by writeMenu in menu.go.

func promptString(reader *bufio.Reader, stdout io.Writer, label string, fallback string) (string, error) {
	if _, err := fmt.Fprintf(stdout, "%s [%s]: ", label, fallback); err != nil {
		return "", err
	}
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return fallback, nil
	}
	return line, nil
}

// loadConfigOrFail wraps loadConfigWithProfile with the shared handler
// convention: on failure it reports "load config: <err>" on stderr and returns
// ok=false so the handler can return exitError.
func loadConfigOrFail(path string, profile string, stderr io.Writer) (service.Config, bool) {
	cfg, err := loadConfigWithProfile(path, profile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
		return service.Config{}, false
	}
	return cfg, true
}

func loadConfigWithProfile(path string, profile string) (service.Config, error) {
	cfg, err := service.LoadConfigFile(path)
	if err != nil {
		return service.Config{}, err
	}
	return applyProfileOverride(cfg, profile)
}

// configlessScanHint is printed (on stderr, so JSON/SARIF output stays clean)
// when a scan runs without a config file.
const configlessScanHint = "note: no codeguard config found; scanning with built-in defaults. Run `codeguard init` to write a config for this repository."

// loadScanConfigWithFallback loads the scan config. When the operator did not
// name a config and none is discoverable, it falls back to the same built-in
// defaults the SDK exposes as DefaultScanConfig, for `scan` and `scan -folder`
// alike, and reports usedDefaults so the caller can print the init hint.
func loadScanConfigWithFallback(path string, profile string, targetPath string, defaultConfigRequested bool) (cfg service.Config, usedDefaults bool, err error) {
	cfg, err = service.LoadConfigFile(path)
	if err == nil {
		cfg, err = applyProfileOverride(cfg, profile)
		return cfg, false, err
	}
	if !defaultConfigRequested || !errors.Is(err, service.ErrConfigNotFound) {
		return service.Config{}, false, err
	}

	// No config file: resolve the repository-aware built-in defaults (what
	// `codeguard init` would write, in memory), NOT codeguard's own starter
	// config, whose CI, release and layering conventions fail any other
	// repository on its first scan.
	root := strings.TrimSpace(targetPath)
	if root == "" {
		root = "."
	}
	cfg, err = service.DefaultScanConfig(root, profile)
	if err != nil {
		return service.Config{}, false, err
	}
	disable := false
	cfg.Cache.Enabled = &disable
	cfg.Cache.Path = ""
	return cfg, true, nil
}

func applyProfileOverride(cfg service.Config, profile string) (service.Config, error) {
	if strings.TrimSpace(profile) != "" {
		cfg.Profile = strings.TrimSpace(profile)
		service.ApplyDefaults(&cfg)
		if err := service.ValidateConfig(cfg); err != nil {
			return service.Config{}, err
		}
	}
	return cfg, nil
}
