package fix

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func inferPackageManagerTestCommand(root string) (core.CommandCheckConfig, bool) {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return core.CommandCheckConfig{}, false
	}

	var manifest struct {
		Scripts        map[string]string `json:"scripts"`
		PackageManager string            `json:"packageManager"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return core.CommandCheckConfig{}, false
	}
	if strings.TrimSpace(manifest.Scripts["test"]) == "" {
		return core.CommandCheckConfig{}, false
	}

	manager := detectPackageManager(root, manifest.PackageManager)
	name := manager + " test"
	return core.CommandCheckConfig{Name: name, Command: manager, Args: []string{"test"}}, true
}

func detectPackageManager(root string, packageManagerField string) string {
	pm := strings.TrimSpace(packageManagerField)
	switch {
	case strings.HasPrefix(pm, "pnpm@"):
		return "pnpm"
	case strings.HasPrefix(pm, "yarn@"):
		return "yarn"
	case strings.HasPrefix(pm, "bun@"):
		return "bun"
	case strings.HasPrefix(pm, "npm@"):
		return "npm"
	}

	switch {
	case fileExists(filepath.Join(root, "pnpm-lock.yaml")):
		return "pnpm"
	case fileExists(filepath.Join(root, "yarn.lock")):
		return "yarn"
	case fileExists(filepath.Join(root, "bun.lock")), fileExists(filepath.Join(root, "bun.lockb")):
		return "bun"
	default:
		return "npm"
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
