package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveDesignRulesPath(configPath, explicit string) (string, bool, error) {
	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		return "", false, err
	}
	configDir := filepath.Dir(absConfig)
	projectRoot := configDir
	discoveryDir := filepath.Join(projectRoot, ".codeguard")
	if filepath.Base(configDir) == ".codeguard" {
		projectRoot = filepath.Dir(configDir)
		discoveryDir = configDir
	}

	if strings.TrimSpace(explicit) != "" {
		return explicitDesignRulesPath(projectRoot, configDir, explicit)
	}

	return discoverDesignRulesPath(projectRoot, discoveryDir)
}

func explicitDesignRulesPath(projectRoot, configDir, explicit string) (string, bool, error) {
	candidate := explicit
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(configDir, candidate)
	}
	contained, err := containedPath(projectRoot, candidate)
	if err != nil {
		return "", false, fmt.Errorf("checks.design_rules_file: %w", err)
	}
	if _, err := os.Stat(contained); err != nil {
		return "", false, fmt.Errorf("checks.design_rules_file %q: %w", explicit, err)
	}
	return contained, true, nil
}

func discoverDesignRulesPath(projectRoot, discoveryDir string) (string, bool, error) {
	for _, name := range defaultDesignRulesNames {
		candidate, found, err := inspectDiscoveredDesignRulesPath(projectRoot, filepath.Join(discoveryDir, name))
		if err != nil || found {
			return candidate, found, err
		}
	}
	return "", false, nil
}

func inspectDiscoveredDesignRulesPath(projectRoot, path string) (string, bool, error) {
	candidate, err := containedPath(projectRoot, path)
	if err != nil {
		return "", false, fmt.Errorf("discover design rules: %w", err)
	}
	info, err := os.Stat(candidate)
	switch {
	case err == nil && !info.IsDir():
		return candidate, true, nil
	case err == nil:
		return "", false, nil
	case os.IsNotExist(err):
		return "", false, nil
	default:
		return "", false, fmt.Errorf("discover design rules file %q: %w", candidate, err)
	}
}
