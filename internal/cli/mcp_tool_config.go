package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func (s *mcpToolService) loadConfig(configPath string, profile string) (service.Config, error) {
	path := strings.TrimSpace(configPath)
	if path == "" {
		path = s.defaultConfigPath
	}
	overrideProfile := strings.TrimSpace(profile)
	if overrideProfile == "" {
		overrideProfile = strings.TrimSpace(s.defaultProfile)
	}
	return loadConfigWithProfile(path, overrideProfile)
}

func confineConfigArg(ctx context.Context, s *mcpToolService, configPath string) (string, error) {
	candidate := strings.TrimSpace(configPath)
	if candidate == "" {
		return "", nil
	}
	return confinePath(allowedRoots(ctx, s), candidate)
}

const rootsFetchTimeout = 10 * time.Second

func allowedRoots(ctx context.Context, s *mcpToolService) []string {
	roots := make([]string, 0, 3)
	if def := strings.TrimSpace(s.defaultConfigPath); def != "" {
		roots = append(roots, configDirOf(def))
	}
	if wd, err := os.Getwd(); err == nil {
		roots = append(roots, wd)
	}
	if caller := clientCallerFrom(ctx); caller != nil && caller.supports("roots") {
		rctx, cancel := context.WithTimeout(ctx, rootsFetchTimeout)
		if clientRoots, err := caller.listRoots(rctx); err == nil {
			for _, root := range clientRoots {
				if p := rootURIToPath(root.URI); p != "" {
					roots = append(roots, p)
				}
			}
		}
		cancel()
	}
	return roots
}

func rootURIToPath(uri string) string {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return ""
	}
	if strings.HasPrefix(uri, "file://") {
		return strings.TrimPrefix(uri, "file://")
	}
	if strings.Contains(uri, "://") {
		return ""
	}
	return uri
}

func configDirOf(path string) string {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return path
	}
	return filepath.Dir(path)
}

func confinePath(allowedRoots []string, candidate string) (string, error) {
	abs := resolvePath(candidate)
	for _, root := range allowedRoots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		rootAbs := resolvePath(root)
		if abs == rootAbs || strings.HasPrefix(abs, rootAbs+string(os.PathSeparator)) {
			return abs, nil
		}
	}
	return "", errConfigPathNotPermitted
}

func resolvePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	abs = filepath.Clean(abs)
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}
