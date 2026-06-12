package support

import (
	"os"
	"path/filepath"
	"strings"
)

// GoModulePath reads the module path declared in dir/go.mod, or returns ""
// when the file is missing or has no module directive.
func GoModulePath(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}
