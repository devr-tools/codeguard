package support

import (
	"os"
	"path/filepath"
	"strings"
)

const codeguardTypeScriptLibEnv = "CODEGUARD_TYPESCRIPT_LIB_PATH"

var defaultTypeScriptLibCandidates = []string{
	"/Applications/Visual Studio Code.app/Contents/Resources/app/extensions/node_modules/typescript/lib/typescript.js",
}

func discoverTypeScriptLibPath(targetPath string) string {
	if candidate := strings.TrimSpace(os.Getenv(codeguardTypeScriptLibEnv)); isTypeScriptLibPath(candidate) {
		return candidate
	}
	for _, candidate := range typeScriptLibCandidates(targetPath) {
		if isTypeScriptLibPath(candidate) {
			return candidate
		}
	}
	return ""
}

func typeScriptLibCandidates(targetPath string) []string {
	candidates := make([]string, 0, 8)
	for _, dir := range ancestorPaths(targetPath) {
		candidates = append(candidates, filepath.Join(dir, "node_modules", "typescript", "lib", "typescript.js"))
	}
	return append(candidates, defaultTypeScriptLibCandidates...)
}

func ancestorPaths(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	current, err := filepath.Abs(path)
	if err != nil {
		current = path
	}
	paths := make([]string, 0, 6)
	for {
		paths = append(paths, current)
		parent := filepath.Dir(current)
		if parent == current {
			return paths
		}
		current = parent
	}
}

func isTypeScriptLibPath(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path) //nolint:gosec // stat-only existence check during source discovery
	return err == nil && !info.IsDir()
}
