package initconfig

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// detectEntrypoints finds entrypoints per language group, all relative to
// root:
//
//   - Go: cmd/<name>/main.go (written as the cmd/<name> package directory, the
//     form design reachability matches) and a module-root main.go, beside
//     every go.mod.
//   - TypeScript/JavaScript: package.json "bin" and "main", when the file they
//     name exists and is not in a tree the scanner skips (a "main" pointing
//     at dist/ is build output, not an entrypoint codeguard can scan).
//   - Python: pyproject.toml [project.scripts] / [tool.poetry.scripts]
//     modules, resolved to a file, plus every __main__.py.
func detectEntrypoints(root string, manifests []string, pythonMains []string) map[string][]string {
	found := map[string]map[string]struct{}{}
	add := func(group string, rel string) {
		rel = cleanRel(rel)
		// An entrypoint the scanner never reads (dist/, node_modules/, ...)
		// is build output or a dependency, not something to anchor on.
		if rel == "" || runnersupport.ShouldExclude(rel, nil) {
			return
		}
		if found[group] == nil {
			found[group] = map[string]struct{}{}
		}
		found[group][rel] = struct{}{}
	}

	for _, manifest := range manifests {
		dir := path.Dir(manifest)
		switch path.Base(manifest) {
		case "go.mod":
			for _, entry := range goEntrypoints(root, dir) {
				add("go", entry)
			}
		case "package.json":
			for _, entry := range packageJSONEntrypoints(root, manifest) {
				add("typescript", entry)
			}
		case "pyproject.toml":
			for _, entry := range pyprojectEntrypoints(root, manifest) {
				add("python", entry)
			}
		}
	}
	for _, main := range pythonMains {
		add("python", main)
	}

	out := make(map[string][]string, len(found))
	for group, set := range found {
		list := make([]string, 0, len(set))
		for rel := range set {
			list = append(list, rel)
		}
		sort.Strings(list)
		if len(list) > maxEntrypointsPerLang {
			list = list[:maxEntrypointsPerLang]
		}
		out[group] = list
	}
	return out
}

func goEntrypoints(root string, moduleDir string) []string {
	var entries []string
	if fileExists(root, path.Join(moduleDir, "main.go")) {
		entries = append(entries, path.Join(moduleDir, "main.go"))
	}
	cmdDir := filepath.Join(root, filepath.FromSlash(path.Join(moduleDir, "cmd")))
	children, err := os.ReadDir(cmdDir)
	if err != nil {
		return entries
	}
	for _, child := range children {
		if !child.IsDir() {
			continue
		}
		pkg := path.Join(moduleDir, "cmd", child.Name())
		if fileExists(root, path.Join(pkg, "main.go")) {
			entries = append(entries, pkg)
		}
	}
	return entries
}

func packageJSONEntrypoints(root string, manifest string) []string {
	data, ok := readRootFile(root, manifest)
	if !ok {
		return nil
	}
	var pkg struct {
		Main string          `json:"main"`
		Bin  json.RawMessage `json:"bin"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return nil
	}
	candidates := []string{pkg.Main}
	var single string
	var named map[string]string
	switch {
	case json.Unmarshal(pkg.Bin, &single) == nil:
		candidates = append(candidates, single)
	case json.Unmarshal(pkg.Bin, &named) == nil:
		for _, value := range named {
			candidates = append(candidates, value)
		}
	}
	dir := path.Dir(manifest)
	var entries []string
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || path.IsAbs(candidate) {
			continue
		}
		rel := path.Join(dir, candidate)
		if !escapesRoot(rel) && fileExists(root, rel) {
			entries = append(entries, rel)
		}
	}
	return entries
}

var (
	tomlSectionLine = regexp.MustCompile(`^\s*\[([^\]]+)\]\s*$`)
	tomlScriptLine  = regexp.MustCompile(`^\s*["']?[A-Za-z0-9._-]+["']?\s*=\s*["']([A-Za-z0-9_.]+)(?::[A-Za-z0-9_.]+)?["']`)
)

func pyprojectEntrypoints(root string, manifest string) []string {
	data, ok := readRootFile(root, manifest)
	if !ok {
		return nil
	}
	dir := path.Dir(manifest)
	var entries []string
	inScripts := false
	for _, line := range strings.Split(string(data), "\n") {
		if match := tomlSectionLine.FindStringSubmatch(line); match != nil {
			section := strings.TrimSpace(match[1])
			inScripts = section == "project.scripts" || section == "tool.poetry.scripts"
			continue
		}
		if !inScripts {
			continue
		}
		match := tomlScriptLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if file, found := resolvePythonModule(root, dir, match[1]); found {
			entries = append(entries, file)
		}
	}
	return entries
}

// resolvePythonModule maps a dotted module name to its source file, trying the
// flat and src/ layouts.
func resolvePythonModule(root string, dir string, module string) (string, bool) {
	modulePath := strings.ReplaceAll(module, ".", "/")
	for _, base := range []string{dir, path.Join(dir, "src")} {
		for _, candidate := range []string{modulePath + ".py", modulePath + "/__init__.py", modulePath + "/__main__.py"} {
			rel := path.Join(base, candidate)
			if fileExists(root, rel) {
				return rel, true
			}
		}
	}
	return "", false
}

func cleanRel(rel string) string {
	rel = path.Clean(filepath.ToSlash(strings.TrimSpace(rel)))
	if rel == "." || rel == "" || escapesRoot(rel) {
		return ""
	}
	return rel
}

func escapesRoot(rel string) bool {
	rel = path.Clean(rel)
	return rel == ".." || strings.HasPrefix(rel, "../")
}
