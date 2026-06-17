package support

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func parsePackageLockState(data []byte) (lockfileState, bool) {
	var doc struct {
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
		Dependencies map[string]json.RawMessage `json:"dependencies"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return lockfileState{}, false
	}
	state := newLockfileState()
	for pkgPath, pkg := range doc.Packages {
		name := packageLockEntryName(pkgPath)
		if name != "" {
			addLockfilePackage(state, name, strings.TrimSpace(pkg.Version))
		}
	}
	for name, raw := range doc.Dependencies {
		var dep struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(raw, &dep); err == nil {
			addLockfilePackage(state, name, strings.TrimSpace(dep.Version))
		}
	}
	return state, true
}

func parsePNPMLockState(data []byte) (lockfileState, bool) {
	var doc struct {
		Packages map[string]any `yaml:"packages"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return lockfileState{}, false
	}
	state := newLockfileState()
	for key := range doc.Packages {
		name, version := parsePNPMPackageKey(key)
		if name != "" {
			addLockfilePackage(state, name, version)
		}
	}
	return state, true
}

func parseYarnLockState(data []byte) lockfileState {
	state := newLockfileState()
	lines := strings.Split(string(data), "\n")
	currentSelectors := make([]string, 0)
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		switch {
		case line == "":
			currentSelectors = nil
		case !strings.HasPrefix(rawLine, " ") && strings.HasSuffix(line, ":"):
			currentSelectors = parseYarnSelectors(strings.TrimSuffix(line, ":"))
		case len(currentSelectors) > 0 && strings.HasPrefix(line, "version "):
			version := firstQuotedValue(line)
			for _, selector := range currentSelectors {
				name, req := parseYarnSelector(selector)
				if name == "" {
					continue
				}
				addLockfilePackage(state, name, version)
				addLockfileSelector(state, name, req)
			}
		}
	}
	return state
}

func packageLockEntryName(entry string) string {
	entry = filepath.ToSlash(strings.TrimSpace(entry))
	if entry == "" || entry == "node_modules" {
		return ""
	}
	if idx := strings.LastIndex(entry, "node_modules/"); idx >= 0 {
		return strings.TrimPrefix(entry[idx:], "node_modules/")
	}
	return ""
}

func parsePNPMPackageKey(key string) (string, string) {
	key = strings.TrimSpace(strings.TrimPrefix(key, "/"))
	if key == "" {
		return "", ""
	}
	if idx := strings.Index(key, "("); idx >= 0 {
		key = key[:idx]
	}
	at := strings.LastIndex(key, "@")
	if at <= 0 {
		return "", ""
	}
	return key[:at], key[at+1:]
}

func parseYarnSelectors(line string) []string {
	parts := strings.Split(line, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.Trim(part, `"`))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseYarnSelector(selector string) (string, string) {
	selector = strings.TrimSpace(strings.Trim(selector, `"`))
	if selector == "" {
		return "", ""
	}
	at := strings.LastIndex(selector, "@")
	if at <= 0 {
		return "", ""
	}
	return selector[:at], selector[at+1:]
}
