package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	defaultConfigNames   = []string{"codeguard.yaml", "codeguard.yml", "codeguard.json"}
	directoryConfigNames = []string{"codeguard.yaml", "codeguard.yml", "codeguard.json", "config.yaml", "config.yml", "config.json"}
	defaultConfigDirs    = []string{".", ".codeguard"}
)

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
	baseDir := filepath.Dir(resolvedPath)
	resolveRelativePaths(&cfg, baseDir)
	ApplyDefaults(&cfg)
	if err := containConfigArtifactPaths(&cfg, baseDir); err != nil {
		return core.Config{}, err
	}
	if err := Validate(cfg); err != nil {
		return core.Config{}, err
	}
	return cfg, nil
}

// containConfigArtifactPaths resolves codeguard's config-controlled output
// paths (baseline, scan cache, AI cache) relative to the config directory and
// rejects any path that escapes that directory tree. Because the config file is
// checked into the repository and may be authored by an untrusted contributor,
// this prevents a config from steering codeguard into reading or writing files
// outside the repository (path traversal / arbitrary file write).
func containConfigArtifactPaths(cfg *core.Config, baseDir string) error {
	type artifact struct {
		label string
		path  *string
	}
	for _, a := range []artifact{
		{"baseline.path", &cfg.Baseline.Path},
		{"cache.path", &cfg.Cache.Path},
		{"ai.cache.path", &cfg.AI.Cache.Path},
	} {
		resolved, err := containedPath(baseDir, *a.path)
		if err != nil {
			return fmt.Errorf("%s: %w", a.label, err)
		}
		*a.path = resolved
	}
	return nil
}

// containedPath resolves p (relative to baseDir if not absolute) and returns the
// cleaned path, erroring if it escapes baseDir. An empty path is returned
// unchanged.
func containedPath(baseDir, p string) (string, error) {
	if strings.TrimSpace(p) == "" {
		return p, nil
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	resolved := p
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(absBase, resolved)
	}
	resolved = filepath.Clean(resolved)
	rel, err := filepath.Rel(absBase, resolved)
	if err != nil {
		return "", fmt.Errorf("path %q is not within the config directory", p)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes the config directory %q", p, absBase)
	}
	return resolved, nil
}

func resolveRelativePaths(cfg *core.Config, baseDir string) {
	for i := range cfg.Targets {
		targetPath := strings.TrimSpace(cfg.Targets[i].Path)
		if targetPath == "" || filepath.IsAbs(targetPath) {
			continue
		}
		cfg.Targets[i].Path = filepath.Join(baseDir, targetPath)
	}
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
