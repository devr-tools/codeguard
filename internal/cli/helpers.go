package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/config"
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

func loadScanConfigWithFallback(path string, profile string, targetPath string, defaultConfigRequested bool) (service.Config, error) {
	cfg, err := service.LoadConfigFile(path)
	if err == nil {
		return applyProfileOverride(cfg, profile)
	}
	if !defaultConfigRequested || strings.TrimSpace(targetPath) == "" || !errors.Is(err, service.ErrConfigNotFound) {
		return service.Config{}, err
	}

	// No config file: resolve the built-in defaults, NOT the `codeguard init`
	// starter config. The starter states codeguard's own conventions, so using
	// it here made a first run on an unseen repository fail on this repo's
	// workflow filenames, `go test ./...` marker, tests/ layout and cmd//pkg
	// layering.
	cfg, err = config.DefaultScanConfig(profile)
	if err != nil {
		return service.Config{}, err
	}
	disable := false
	cfg.Cache.Enabled = &disable
	cfg.Cache.Path = ""
	return cfg, nil
}

// starterConfigForProfile returns the rich, opinionated config `codeguard
// init` writes into a repository. It is only correct for init: the user reads
// and edits the result. A scan must never fall back to it — see
// config.DefaultScanConfig.
func starterConfigForProfile(profile string) (service.Config, error) {
	if strings.TrimSpace(profile) == "" {
		return service.ExampleConfig(), nil
	}
	return service.ExampleConfigForProfile(profile)
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
