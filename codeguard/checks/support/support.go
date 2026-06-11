package support

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/codeguard/core"
)

type TargetState struct {
	Target core.TargetConfig
	Info   os.FileInfo
	Err    error
}

func CollectTargetStates(cfg core.Config) []TargetState {
	states := make([]TargetState, 0, len(cfg.Targets))
	for _, target := range cfg.Targets {
		info, err := os.Stat(target.Path)
		states = append(states, TargetState{
			Target: target,
			Info:   info,
			Err:    err,
		})
	}
	return states
}

func IsGoTarget(target core.TargetConfig) bool {
	switch strings.ToLower(strings.TrimSpace(target.Language)) {
	case "go", "golang":
		return true
	default:
		return false
	}
}

func GoFiles(targetPath string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(targetPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDir(d.Name()) && path != targetPath {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			files = append(files, filepath.Clean(path))
		}
		return nil
	})
	return files, err
}

func ScopedGoFiles(targetPath string, scope core.ScanScope) ([]string, error) {
	files, err := GoFiles(targetPath)
	if err != nil {
		return nil, err
	}
	return filterPathsForScope(files, scope), nil
}

func CandidateTextFiles(targetPath string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(targetPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDir(d.Name()) && path != targetPath {
				return filepath.SkipDir
			}
			return nil
		}
		if isCandidateTextFile(path) {
			files = append(files, filepath.Clean(path))
		}
		return nil
	})
	return files, err
}

func ScopedCandidateTextFiles(targetPath string, scope core.ScanScope) ([]string, error) {
	files, err := CandidateTextFiles(targetPath)
	if err != nil {
		return nil, err
	}
	return filterPathsForScope(files, scope), nil
}

func filterPathsForScope(paths []string, scope core.ScanScope) []string {
	if scope.Mode != core.ScanModeDiff || len(scope.ChangedFiles) == 0 {
		return paths
	}
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		clean := filepath.Clean(path)
		if _, ok := scope.ChangedFiles[clean]; ok {
			filtered = append(filtered, clean)
		}
	}
	return filtered
}

func skipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "dist", ".idea", ".vscode":
		return true
	default:
		return strings.HasPrefix(name, ".") && name != "."
	}
}

func isCandidateTextFile(path string) bool {
	switch filepath.Ext(path) {
	case ".go", ".json", ".yaml", ".yml", ".toml", ".env", ".txt", ".md", ".sh", ".tf", ".tmpl", ".prompt":
		return true
	}
	base := filepath.Base(path)
	switch base {
	case ".env", ".env.local", ".env.development", ".env.production":
		return true
	default:
		return false
	}
}
