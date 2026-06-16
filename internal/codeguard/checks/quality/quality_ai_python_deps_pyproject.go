package quality

import (
	"os"
	"path/filepath"
	"strings"
)

// pyproject.toml dependency extraction for the Python dependency catalog.

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
