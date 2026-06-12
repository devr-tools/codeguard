package quality

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

type packageManifest struct {
	Name             string            `json:"name"`
	Dependencies     map[string]string `json:"dependencies"`
	DevDependencies  map[string]string `json:"devDependencies"`
	PeerDependencies map[string]string `json:"peerDependencies"`
}

func readPackageManifest(root string) (packageManifest, bool) {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return packageManifest{}, false
	}
	var manifest packageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return packageManifest{}, false
	}
	return manifest, true
}

func packageManifestDeps(manifest packageManifest) map[string]struct{} {
	deps := map[string]struct{}{}
	for name := range manifest.Dependencies {
		deps[name] = struct{}{}
	}
	for name := range manifest.DevDependencies {
		deps[name] = struct{}{}
	}
	for name := range manifest.PeerDependencies {
		deps[name] = struct{}{}
	}
	if strings.TrimSpace(manifest.Name) != "" {
		deps[strings.TrimSpace(manifest.Name)] = struct{}{}
	}
	return deps
}

func readWorkspacePackageNames(root string, excludes []string) map[string]struct{} {
	files, err := runnersupport.WalkFiles(root, excludes, func(rel string) bool {
		return filepath.Base(rel) == "package.json"
	})
	if err != nil {
		return map[string]struct{}{}
	}
	names := map[string]struct{}{}
	for _, rel := range files {
		manifest, ok := readPackageManifest(filepath.Join(root, filepath.Dir(rel)))
		if !ok || strings.TrimSpace(manifest.Name) == "" {
			continue
		}
		names[strings.TrimSpace(manifest.Name)] = struct{}{}
	}
	return names
}

func readGitHeadMessage(dir string) string {
	cmd := exec.Command("git", "-C", dir, "log", "-1", "--format=%B")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func envFlagEnabled(keys []string) bool {
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			continue
		}
		switch strings.ToLower(value) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

func hasCommitTrailer(message string, trailers []string) bool {
	lowerMessage := strings.ToLower(message)
	for _, trailer := range trailers {
		if strings.Contains(lowerMessage, strings.ToLower(strings.TrimSpace(trailer))+":") {
			return true
		}
	}
	return false
}

func packageRoot(specifier string) string {
	if strings.HasPrefix(specifier, "@") {
		parts := strings.Split(specifier, "/")
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
	}
	return firstSegment(specifier)
}

func isNodeBuiltinPackage(root string) bool {
	if strings.HasPrefix(root, "node:") {
		return true
	}
	return slices.Contains([]string{
		"assert", "buffer", "child_process", "crypto", "events", "fs", "http",
		"https", "net", "os", "path", "stream", "timers", "url", "util", "zlib",
	}, root)
}
