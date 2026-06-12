package quality

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type pythonDependencyCatalog struct {
	hasManifest bool
	// deps holds PEP 503 normalized distribution names declared by the repo.
	deps map[string]struct{}
}

var (
	pythonRequirementLinePattern = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._-]*)`)
	pythonTomlSectionPattern     = regexp.MustCompile(`^\s*\[([^\]]+)\]\s*$`)
	pythonTomlKeyPattern         = regexp.MustCompile(`^\s*(?:"([^"]+)"|([A-Za-z0-9._-]+))\s*=`)
	pythonStringLiteralPattern   = regexp.MustCompile(`["']([A-Za-z0-9][A-Za-z0-9._\[\],<>=!~; -]*)["']`)
	pythonSetupRequiresPattern   = regexp.MustCompile(`(?s)install_requires\s*=\s*\[(.*?)\]`)
)

// normalizePythonPackageName applies PEP 503 normalization so declared
// distribution names and import names compare consistently.
func normalizePythonPackageName(name string) string {
	lowered := strings.ToLower(strings.TrimSpace(name))
	replacer := strings.NewReplacer("_", "-", ".", "-")
	return replacer.Replace(lowered)
}

func readPythonDependencyCatalog(root string) pythonDependencyCatalog {
	catalog := pythonDependencyCatalog{deps: map[string]struct{}{}}
	readPythonRequirementsFiles(root, &catalog)
	readPythonPyprojectDeps(root, &catalog)
	readPythonSetupPyDeps(root, &catalog)
	readPythonSetupCfgDeps(root, &catalog)
	return catalog
}

func (catalog *pythonDependencyCatalog) add(requirement string) {
	name := pythonRequirementName(requirement)
	if name == "" {
		return
	}
	catalog.deps[normalizePythonPackageName(name)] = struct{}{}
}

func (catalog pythonDependencyCatalog) declares(distribution string) bool {
	_, ok := catalog.deps[normalizePythonPackageName(distribution)]
	return ok
}

// pythonRequirementName extracts the distribution name from a requirement
// specifier such as "requests[security]>=2.0; python_version > '3.8'".
func pythonRequirementName(requirement string) string {
	trimmed := strings.TrimSpace(requirement)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "-") {
		return ""
	}
	match := pythonRequirementLinePattern.FindString(trimmed)
	return match
}

func readPythonRequirementsFiles(root string, catalog *pythonDependencyCatalog) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		if entry.IsDir() || !strings.HasPrefix(name, "requirements") || !strings.HasSuffix(name, ".txt") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			continue
		}
		catalog.hasManifest = true
		for _, line := range strings.Split(string(data), "\n") {
			catalog.add(line)
		}
	}
}

func readPythonPyprojectDeps(root string, catalog *pythonDependencyCatalog) {
	data, err := os.ReadFile(filepath.Join(root, "pyproject.toml"))
	if err != nil {
		return
	}
	catalog.hasManifest = true
	section := ""
	inDependencyArray := false
	for _, line := range strings.Split(string(data), "\n") {
		if match := pythonTomlSectionPattern.FindStringSubmatch(line); match != nil {
			section = strings.TrimSpace(match[1])
			inDependencyArray = false
			continue
		}
		switch {
		case isPythonProjectDependencySection(section):
			collectPythonTomlArrayDeps(line, catalog)
		case isPythonPoetryDependencySection(section):
			collectPythonPoetryDep(line, catalog)
		case section == "project":
			collectPythonProjectTableDeps(line, &inDependencyArray, catalog)
		}
	}
}

func isPythonProjectDependencySection(section string) bool {
	return section == "project.optional-dependencies" || section == "dependency-groups"
}

func isPythonPoetryDependencySection(section string) bool {
	if section == "tool.poetry.dependencies" || section == "tool.poetry.dev-dependencies" {
		return true
	}
	return strings.HasPrefix(section, "tool.poetry.group.") && strings.HasSuffix(section, ".dependencies")
}

// collectPythonProjectTableDeps handles "[project]" content, including the
// project name and the dependencies array.
func collectPythonProjectTableDeps(line string, inArray *bool, catalog *pythonDependencyCatalog) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "name") && strings.Contains(trimmed, "=") {
		for _, match := range pythonStringLiteralPattern.FindAllStringSubmatch(trimmed, 1) {
			catalog.add(match[1])
		}
		return
	}
	if strings.HasPrefix(trimmed, "dependencies") && strings.Contains(trimmed, "=") {
		*inArray = true
	}
	if *inArray {
		for _, match := range pythonStringLiteralPattern.FindAllStringSubmatch(line, -1) {
			catalog.add(match[1])
		}
		if strings.Contains(trimmed, "]") {
			*inArray = false
		}
	}
}

// collectPythonTomlArrayDeps handles sections whose values are arrays of
// requirement strings, e.g. [project.optional-dependencies].
func collectPythonTomlArrayDeps(line string, catalog *pythonDependencyCatalog) {
	for _, match := range pythonStringLiteralPattern.FindAllStringSubmatch(line, -1) {
		catalog.add(match[1])
	}
}

// collectPythonPoetryDep handles "name = version" style entries in poetry
// dependency tables.
func collectPythonPoetryDep(line string, catalog *pythonDependencyCatalog) {
	match := pythonTomlKeyPattern.FindStringSubmatch(line)
	if match == nil {
		return
	}
	key := match[1]
	if key == "" {
		key = match[2]
	}
	if strings.EqualFold(key, "python") {
		return
	}
	catalog.add(key)
}

func readPythonSetupPyDeps(root string, catalog *pythonDependencyCatalog) {
	data, err := os.ReadFile(filepath.Join(root, "setup.py"))
	if err != nil {
		return
	}
	catalog.hasManifest = true
	for _, block := range pythonSetupRequiresPattern.FindAllStringSubmatch(string(data), -1) {
		for _, match := range pythonStringLiteralPattern.FindAllStringSubmatch(block[1], -1) {
			catalog.add(match[1])
		}
	}
}

func readPythonSetupCfgDeps(root string, catalog *pythonDependencyCatalog) {
	data, err := os.ReadFile(filepath.Join(root, "setup.cfg"))
	if err != nil {
		return
	}
	catalog.hasManifest = true
	inRequires := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "install_requires"):
			inRequires = true
			if idx := strings.Index(trimmed, "="); idx >= 0 {
				catalog.add(strings.TrimSpace(trimmed[idx+1:]))
			}
		case strings.HasPrefix(trimmed, "[") || (trimmed != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.Contains(trimmed, "=")):
			if !strings.HasPrefix(trimmed, "install_requires") {
				inRequires = false
			}
		case inRequires && trimmed != "":
			catalog.add(trimmed)
		}
	}
}
