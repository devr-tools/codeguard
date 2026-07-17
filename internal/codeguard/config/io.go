package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// maxConfigFileBytes caps how much of a config file is read into memory,
// guarding against an oversized file exhausting memory.
const maxConfigFileBytes = 32 << 20

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

	f, err := os.Open(resolvedPath) //nolint:gosec // operator-supplied config path; read is size-capped by LimitReader below
	if err != nil {
		return core.Config{}, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxConfigFileBytes))
	if err != nil {
		return core.Config{}, err
	}

	var cfg core.Config
	if err := unmarshalConfig(data, resolvedPath, &cfg); err != nil {
		return core.Config{}, err
	}
	designOverlay, err := loadExternalDesignRules(&cfg, data, resolvedPath)
	if err != nil {
		return core.Config{}, err
	}
	baseDir := filepath.Dir(resolvedPath)
	resolveRelativePaths(&cfg, baseDir)
	ApplyDefaults(&cfg)
	// Applying defaults intentionally treats numeric zero as unset. When an
	// external design policy is used, however, explicitly present inline fields
	// must win even when their value is zero, false, or empty.
	designOverlay.apply(&cfg.Checks.DesignRules)
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
		{"performance_rules.benchmarks.baseline_path", &cfg.Checks.PerformanceRules.Benchmarks.BaselinePath},
		{"performance_rules.build_regression.baseline_path", &cfg.Checks.PerformanceRules.BuildRegression.BaselinePath},
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

	// The lexical check above is not enough: a symlink committed inside the
	// config directory could still redirect a write outside it. Canonicalize
	// both the base and the target's deepest existing ancestor (the target
	// itself is an output path that may not exist yet) and re-check containment.
	realBase, err := canonicalizeExistingPrefix(absBase)
	if err != nil {
		return "", err
	}
	realResolved, err := canonicalizeExistingPrefix(resolved)
	if err != nil {
		return "", err
	}
	realRel, err := filepath.Rel(realBase, realResolved)
	if err != nil || realRel == ".." || strings.HasPrefix(realRel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q resolves outside the config directory %q via a symlink", p, absBase)
	}

	return resolved, nil
}

// canonicalizeExistingPrefix resolves symlinks in path, tolerating a path that
// does not exist yet by resolving the deepest existing ancestor and re-joining
// the remaining (necessarily non-symlink) components.
func canonicalizeExistingPrefix(path string) (string, error) {
	current := filepath.Clean(path)
	remainder := ""
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			return filepath.Join(resolved, remainder), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			// Reached the filesystem root without an existing ancestor.
			return filepath.Join(current, remainder), nil
		}
		remainder = filepath.Join(filepath.Base(current), remainder)
		current = parent
	}
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
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
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
