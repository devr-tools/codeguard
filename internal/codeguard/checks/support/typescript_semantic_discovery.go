package support

import (
	"os"
	"strings"
)

const codeguardTypeScriptLibEnv = "CODEGUARD_TYPESCRIPT_LIB_PATH"

var defaultTypeScriptLibCandidates = []string{
	"/Applications/Visual Studio Code.app/Contents/Resources/app/extensions/node_modules/typescript/lib/typescript.js",
}

func discoverTypeScriptLibPath(_ string) string {
	if candidate := strings.TrimSpace(os.Getenv(codeguardTypeScriptLibEnv)); isTypeScriptLibPath(candidate) {
		return candidate
	}
	for _, candidate := range defaultTypeScriptLibCandidates {
		if isTypeScriptLibPath(candidate) {
			return candidate
		}
	}
	return ""
}

func isTypeScriptLibPath(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path) //nolint:gosec // stat-only existence check during source discovery
	return err == nil && !info.IsDir()
}
