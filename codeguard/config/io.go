package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/codeguard/core"
	"gopkg.in/yaml.v3"
)

func LoadConfigFile(path string) (core.Config, error) {
	resolvedPath, err := ResolveConfigPath(path)
	if err != nil {
		return core.Config{}, err
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return core.Config{}, fmt.Errorf("read %s: %w", resolvedPath, err)
	}

	var cfg core.Config
	switch strings.ToLower(filepath.Ext(resolvedPath)) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return core.Config{}, fmt.Errorf("decode %s: %w", resolvedPath, err)
		}
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return core.Config{}, fmt.Errorf("decode %s: %w", resolvedPath, err)
		}
	default:
		if err := json.Unmarshal(data, &cfg); err == nil {
			return cfg, nil
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return core.Config{}, fmt.Errorf("decode %s: %w", resolvedPath, err)
		}
	}
	return cfg, nil
}

func WriteConfigFile(path string, cfg core.Config) error {
	var (
		data []byte
		err  error
	)
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		data, err = yaml.Marshal(cfg)
	case ".json", "":
		data, err = json.MarshalIndent(cfg, "", "  ")
	default:
		data, err = json.MarshalIndent(cfg, "", "  ")
	}
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func DefaultConfigPath() string {
	candidates := configCandidates(".")
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "codeguard.yaml"
}

func ResolveConfigPath(path string) (string, error) {
	if path == "" {
		path = DefaultConfigPath()
	}

	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		for _, candidate := range configCandidates(path) {
			if _, statErr := os.Stat(candidate); statErr == nil {
				return candidate, nil
			}
		}
		return "", fmt.Errorf("no config file found in directory %s", path)
	}

	if err == nil {
		return path, nil
	}

	if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat %s: %w", path, err)
	}

	return path, nil
}

func configCandidates(baseDir string) []string {
	names := []string{
		"codeguard.yaml",
		"codeguard.yml",
		"codeguard.json",
		"config.yaml",
		"config.yml",
		"config.json",
	}

	candidates := make([]string, 0, len(names))
	for _, name := range names {
		candidates = append(candidates, filepath.Join(baseDir, name))
	}
	if baseDir == "." {
		for _, name := range names[:3] {
			candidates = append(candidates, filepath.Join(".codeguard", name))
		}
	}
	return candidates
}
