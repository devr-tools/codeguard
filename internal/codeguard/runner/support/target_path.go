package support

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// ApplyTargetPath narrows configured targets to an operator-supplied scan
// folder. The path is a runtime option rather than config, so it may point at
// any local directory the caller intentionally chose.
func ApplyTargetPath(cfg *core.Config, targetPath string) error {
	initializeTargetLogicalPaths(cfg)
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return nil
	}
	scanPath, err := absoluteDir(targetPath)
	if err != nil {
		return fmt.Errorf("scan path: %w", err)
	}

	matched := make([]core.TargetConfig, 0, len(cfg.Targets))
	for _, target := range cfg.Targets {
		targetAbs, err := filepath.Abs(target.Path)
		if err != nil {
			return fmt.Errorf("target %q path: %w", target.Name, err)
		}
		targetAbs = filepath.Clean(targetAbs)

		switch {
		case samePath(scanPath, targetAbs), isSubpath(scanPath, targetAbs):
			rel, relErr := filepath.Rel(targetAbs, scanPath)
			if relErr != nil {
				return fmt.Errorf("scan path relative to target %q: %w", target.Name, relErr)
			}
			target.LogicalPath = joinLogicalPath(target.LogicalPath, rel)
			target.Path = scanPath
			matched = append(matched, target)
		case isSubpath(targetAbs, scanPath):
			matched = append(matched, target)
		}
	}

	if len(matched) == 0 {
		if len(cfg.Targets) == 1 {
			target := cfg.Targets[0]
			target.Path = scanPath
			cfg.Targets = []core.TargetConfig{target}
			return nil
		}
		return fmt.Errorf("%q is outside all configured targets", scanPath)
	}

	cfg.Targets = matched
	return nil
}

func initializeTargetLogicalPaths(cfg *core.Config) {
	for i := range cfg.Targets {
		if cfg.Targets[i].LogicalPath != "" {
			continue
		}
		path := filepath.Clean(cfg.Targets[i].Path)
		if !filepath.IsAbs(path) {
			cfg.Targets[i].LogicalPath = filepath.ToSlash(path)
			continue
		}
		switch base := filepath.Base(path); base {
		case "vendor", "node_modules", "cdk.out":
			cfg.Targets[i].LogicalPath = base
		}
	}
}

func joinLogicalPath(base, rel string) string {
	base = filepath.ToSlash(filepath.Clean(base))
	rel = filepath.ToSlash(filepath.Clean(rel))
	if base == "." || base == "" {
		return rel
	}
	if rel == "." || rel == "" {
		return base
	}
	return base + "/" + rel
}

func absoluteDir(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%q is not a directory", abs)
	}
	return abs, nil
}

func samePath(a string, b string) bool {
	rel, err := filepath.Rel(a, b)
	return err == nil && rel == "."
}

func isSubpath(path string, base string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
