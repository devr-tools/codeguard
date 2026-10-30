package initconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/config"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// Options controls how a detected Layout becomes a config.
type Options struct {
	// Profile is an optional built-in policy profile name.
	Profile string
	// TargetPath is the path every target is written with. Empty means ".".
	TargetPath string
	// Name is the config name. Empty leaves the built-in default.
	Name string
}

// Build turns a detected layout into the minimal config `codeguard init`
// writes: the profile, one target per detected language, and the section
// switches. Everything else is deliberately left unset so it resolves from
// the built-in defaults when the file is loaded.
//
// The returned config is NOT defaulted; call config.ApplyDefaults to resolve
// it. It has been checked to validate once defaulted.
func Build(layout Layout, opts Options) (core.Config, error) {
	profile := strings.TrimSpace(opts.Profile)
	if profile != "" {
		// Reuses the profile catalog's unknown-profile error.
		if _, err := config.DefaultScanConfig(profile); err != nil {
			return core.Config{}, err
		}
	}
	cfg := core.Config{
		Name:    strings.TrimSpace(opts.Name),
		Profile: profile,
		Targets: buildTargets(layout, opts.TargetPath),
		Checks:  buildChecks(layout),
	}
	resolved := cfg
	resolved.Targets = append([]core.TargetConfig(nil), cfg.Targets...)
	config.ApplyDefaults(&resolved)
	if err := config.Validate(resolved); err != nil {
		return core.Config{}, err
	}
	return cfg, nil
}

func buildTargets(layout Layout, targetPath string) []core.TargetConfig {
	if strings.TrimSpace(targetPath) == "" {
		targetPath = "."
	}
	if len(layout.Languages) == 0 {
		// Nothing recognizable: one language-less target, which the
		// per-language dispatch treats as Go. Same shape as the conservative
		// config-less default.
		return []core.TargetConfig{{Name: "repository", Path: targetPath}}
	}
	targets := make([]core.TargetConfig, 0, len(layout.Languages))
	for _, lang := range layout.Languages {
		// Every target shares the repository root. Targets are
		// single-language for analysis, so a Python + TypeScript repository
		// needs two; keeping both at the root keeps finding paths
		// repository-relative and lets identical language-agnostic findings
		// (secrets, supply chain, CI) collapse instead of being reported once
		// per target.
		targets = append(targets, core.TargetConfig{
			Name:        lang.Language,
			Path:        targetPath,
			Language:    lang.Language,
			Entrypoints: append([]string(nil), lang.Entrypoints...),
		})
	}
	return targets
}

// buildChecks enables the recommended sections, plus ci only when the
// repository has GitHub Actions workflows to check and supply_chain only when
// it declares package manifests to read. performance is written as an
// explicit false so the scan's "this config predates the performance section"
// note does not fire on a config generated today.
func buildChecks(layout Layout) core.CheckConfig {
	off := false
	return core.CheckConfig{
		Quality:     true,
		Performance: &off,
		Design:      true,
		Security:    true,
		Prompts:     true,
		CI:          len(layout.Workflows) > 0,
		SupplyChain: len(layout.Manifests) > 0,
	}
}

// DefaultScanConfig returns the configuration a scan uses when no config file
// exists: the config `codeguard init` would write for the repository at root,
// resolved in memory, with every target pointed at root. A root that does not
// exist still yields a config, with a single target at root, so the scan
// reports the missing target the same way it would for a config file.
func DefaultScanConfig(root string, profile string) (core.Config, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	layout, err := Detect(root)
	if err != nil && !isMissingRoot(err, root) {
		return core.Config{}, err
	}
	cfg, err := Build(layout, Options{Profile: profile, TargetPath: root})
	if err != nil {
		return core.Config{}, err
	}
	config.ApplyDefaults(&cfg)
	if err = config.Validate(cfg); err != nil {
		return core.Config{}, err
	}
	return cfg, nil
}

func isMissingRoot(err error, root string) bool {
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	info, statErr := os.Stat(root)
	return statErr == nil && !info.IsDir()
}

// TargetPathForConfig returns the path a config written at configPath should
// use to name root. Target paths in a config file resolve against the file's
// own directory, so a config at .codeguard/codeguard.yaml names the
// repository root as "..". When the config lives outside the repository
// entirely, the root is written as an absolute path.
func TargetPathForConfig(configPath string, root string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absConfigDir, err := filepath.Abs(filepath.Dir(configPath))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absConfigDir, absRoot)
	if err != nil {
		return filepath.ToSlash(absRoot), nil
	}
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "../") && rel != ".." {
		return rel, nil
	}
	// Only ".." segments: the config sits inside the repository.
	if strings.Trim(strings.ReplaceAll(rel, "..", ""), "/") == "" {
		return rel, nil
	}
	return filepath.ToSlash(absRoot), nil
}
