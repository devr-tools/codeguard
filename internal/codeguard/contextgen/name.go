package contextgen

import (
	"encoding/json"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	tomlNameLine = regexp.MustCompile(`^\s*name\s*=\s*"([^"\n]+)"\s*$`)
	safeName     = regexp.MustCompile(`^[A-Za-z0-9@][A-Za-z0-9@._/ -]*$`)
)

// projectName is the name a root manifest declares (package.json name,
// pyproject.toml [project] name, Cargo.toml [package] name, or the last
// segment of the go.mod module path), falling back to the directory name.
func projectName(root string) string {
	for _, candidate := range []func(string) string{packageJSONName, pyprojectName, cargoName, goModuleName} {
		if name := strings.TrimSpace(candidate(root)); name != "" && safeName.MatchString(name) {
			return name
		}
	}
	return filepath.Base(root)
}

func packageJSONName(root string) string {
	text, ok := readSmallFile(root, "package.json")
	if !ok {
		return ""
	}
	var manifest struct {
		Name string `json:"name"`
	}
	if json.Unmarshal([]byte(text), &manifest) != nil {
		return ""
	}
	return manifest.Name
}

func pyprojectName(root string) string {
	return tomlTableName(root, "pyproject.toml", "project")
}

func cargoName(root string) string {
	return tomlTableName(root, "Cargo.toml", "package")
}

// tomlTableName reads `name = "..."` from the named top-level table.
func tomlTableName(root string, file string, table string) string {
	text, ok := readSmallFile(root, file)
	if !ok {
		return ""
	}
	inTable := false
	for _, line := range strings.Split(text, "\n") {
		if match := tomlSection.FindStringSubmatch(line); match != nil {
			inTable = strings.TrimSpace(match[1]) == table
			continue
		}
		if !inTable {
			continue
		}
		if match := tomlNameLine.FindStringSubmatch(line); match != nil {
			return match[1]
		}
	}
	return ""
}

func goModuleName(root string) string {
	text, ok := readSmallFile(root, "go.mod")
	if !ok {
		return ""
	}
	if match := goModuleLine.FindStringSubmatch(text); match != nil {
		return path.Base(match[1])
	}
	return ""
}
