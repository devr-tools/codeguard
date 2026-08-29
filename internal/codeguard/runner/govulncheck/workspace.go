package govulncheck

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Module struct {
	Dir        string
	ModulePath string
}

type Workspace struct {
	Root         string
	RootModule   string
	Modules      []Module
	Replacements map[string]string
}

// DiscoverWorkspace resolves active Go modules without invoking project tools.
func DiscoverWorkspace(start string) (Workspace, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return Workspace{}, err
	}
	workPath := findUp(abs, "go.work")
	if workPath != "" {
		return parseWorkspace(workPath)
	}
	modPath := findUp(abs, "go.mod")
	if modPath == "" {
		return Workspace{Root: abs, Replacements: map[string]string{}}, nil
	}
	modulePath, err := readModulePath(modPath)
	if err != nil {
		return Workspace{}, err
	}
	dir := filepath.Dir(modPath)
	return Workspace{Root: dir, RootModule: modulePath, Modules: []Module{{Dir: dir, ModulePath: modulePath}}, Replacements: map[string]string{}}, nil
}

func findUp(start, name string) string {
	dir := start
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func parseWorkspace(path string) (Workspace, error) {
	root := filepath.Dir(path)
	data, err := os.ReadFile(path) // #nosec G304 -- path is the discovered go.work file, not repository content.
	if err != nil {
		return Workspace{}, err
	}
	uses, replacements := parseWorkDirectives(string(data), root)
	modules := make([]Module, 0, len(uses))
	for _, use := range uses {
		modFile := filepath.Join(use, "go.mod")
		modulePath, readErr := readModulePath(modFile)
		if readErr != nil {
			return Workspace{}, fmt.Errorf("workspace module %s: %w", use, readErr)
		}
		modules = append(modules, Module{Dir: use, ModulePath: modulePath})
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].ModulePath < modules[j].ModulePath })
	rootModule := ""
	if modulePath, readErr := readModulePath(filepath.Join(root, "go.mod")); readErr == nil {
		rootModule = modulePath
	}
	return Workspace{Root: root, RootModule: rootModule, Modules: modules, Replacements: replacements}, nil
}

func parseWorkDirectives(content, root string) ([]string, map[string]string) {
	var uses []string
	replacements := map[string]string{}
	inUse := false
	inReplace := false
	for scanner := bufio.NewScanner(strings.NewReader(content)); scanner.Scan(); {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "//", 2)[0])
		switch {
		case line == "use (":
			inUse = true
		case inUse && line == ")":
			inUse = false
		case line == "replace (":
			inReplace = true
		case inReplace && line == ")":
			inReplace = false
		case inUse && line != "":
			uses = append(uses, resolveWorkPath(root, strings.Fields(line)[0]))
		case strings.HasPrefix(line, "use "):
			uses = append(uses, resolveWorkPath(root, strings.Fields(strings.TrimPrefix(line, "use "))[0]))
		case inReplace && strings.Contains(line, "=>"):
			addReplacement(replacements, root, line)
		case strings.HasPrefix(line, "replace ") && strings.Contains(line, "=>"):
			addReplacement(replacements, root, strings.TrimSpace(strings.TrimPrefix(line, "replace ")))
		}
	}
	return uses, replacements
}

func addReplacement(replacements map[string]string, root, directive string) {
	parts := strings.SplitN(directive, "=>", 2)
	oldFields, newFields := strings.Fields(parts[0]), strings.Fields(parts[1])
	if len(oldFields) > 0 && len(newFields) > 0 {
		replacements[oldFields[0]] = resolveWorkPath(root, newFields[0])
	}
}

func resolveWorkPath(root, value string) string {
	value = strings.Trim(value, "\"")
	if filepath.IsAbs(value) || (!strings.HasPrefix(value, ".") && strings.Contains(value, ".")) {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(root, filepath.FromSlash(value)))
}

func readModulePath(path string) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is a go.mod resolved from the workspace root/use directives.
	if err != nil {
		return "", err
	}
	for scanner := bufio.NewScanner(strings.NewReader(string(data))); scanner.Scan(); {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "module" {
			return strings.Trim(fields[1], "\""), nil
		}
	}
	return "", fmt.Errorf("module directive missing in %s", path)
}
