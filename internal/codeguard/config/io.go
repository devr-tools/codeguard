package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"gopkg.in/yaml.v3"
)

var (
	defaultConfigNames   = []string{"codeguard.yaml", "codeguard.yml", "codeguard.json"}
	directoryConfigNames = []string{"codeguard.yaml", "codeguard.yml", "codeguard.json", "config.yaml", "config.yml", "config.json"}
	defaultConfigDirs    = []string{".", ".codeguard"}
)

func DefaultConfigPath() string {
	return defaultConfigNames[0]
}

func LoadFile(path string) (core.Config, error) {
	resolvedPath, err := resolveConfigPath(path)
	if err != nil {
		return core.Config{}, err
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return core.Config{}, err
	}

	var cfg core.Config
	if err := unmarshalConfig(data, resolvedPath, &cfg); err != nil {
		return core.Config{}, err
	}
	ApplyDefaults(&cfg)
	if err := Validate(cfg); err != nil {
		return core.Config{}, err
	}
	return cfg, nil
}

func WriteFile(path string, cfg core.Config) error {
	ApplyDefaults(&cfg)
	if err := Validate(cfg); err != nil {
		return err
	}

	data, err := marshalConfig(path, cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func resolveConfigPath(path string) (string, error) {
	trimmedPath := strings.TrimSpace(path)
	if shouldSearchDefaultConfigs(trimmedPath) {
		if resolved, ok := findConfigInDirs(defaultConfigDirs, defaultConfigNames); ok {
			return resolved, nil
		}
		return trimmedPath, nil
	}

	info, err := os.Stat(trimmedPath)
	if err == nil && info.IsDir() {
		if resolved, ok := findConfigInDirs([]string{trimmedPath}, directoryConfigNames); ok {
			return resolved, nil
		}
		return "", fmt.Errorf("no config file found in %s", trimmedPath)
	}
	if err == nil {
		return trimmedPath, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return trimmedPath, nil
	}
	return "", err
}

func shouldSearchDefaultConfigs(path string) bool {
	if path == "" {
		return true
	}
	if filepath.Dir(path) != "." {
		return false
	}
	for _, name := range defaultConfigNames {
		if path == name {
			return true
		}
	}
	return false
}

func findConfigInDirs(dirs []string, names []string) (string, bool) {
	for _, dir := range dirs {
		for _, name := range names {
			candidate := filepath.Join(dir, name)
			info, err := os.Stat(candidate)
			if err == nil && !info.IsDir() {
				return candidate, true
			}
		}
	}
	return "", false
}

func marshalConfig(path string, cfg core.Config) ([]byte, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return yaml.Marshal(cfg)
	default:
		return json.MarshalIndent(cfg, "", "  ")
	}
}

func unmarshalConfig(data []byte, path string, cfg *core.Config) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return yaml.Unmarshal(data, cfg)
	default:
		return json.Unmarshal(data, cfg)
	}
}
