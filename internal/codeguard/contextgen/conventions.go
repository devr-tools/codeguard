package contextgen

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// configFact is one tool whose presence a config file proves. The first
// existing candidate is cited in the generated line.
type configFact struct {
	tool       string
	candidates []string
}

// configFacts are checked in order; each yields at most one convention line.
var configFacts = []configFact{
	{"EditorConfig settings", []string{".editorconfig"}},
	{"golangci-lint", []string{".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json"}},
	{"Prettier", []string{".prettierrc", ".prettierrc.json", ".prettierrc.yaml", ".prettierrc.yml", ".prettierrc.js", ".prettierrc.cjs", ".prettierrc.mjs", "prettier.config.js", "prettier.config.cjs", "prettier.config.mjs"}},
	{"ESLint", []string{"eslint.config.js", "eslint.config.mjs", "eslint.config.cjs", "eslint.config.ts", ".eslintrc", ".eslintrc.json", ".eslintrc.js", ".eslintrc.cjs", ".eslintrc.yaml", ".eslintrc.yml"}},
	{"Biome", []string{"biome.json", "biome.jsonc"}},
	{"TypeScript compiler options", []string{"tsconfig.json"}},
	{"Ruff", []string{"ruff.toml", ".ruff.toml"}},
	{"mypy", []string{"mypy.ini"}},
	{"pytest", []string{"pytest.ini"}},
	{"rustfmt", []string{"rustfmt.toml", ".rustfmt.toml"}},
	{"Clippy", []string{"clippy.toml", ".clippy.toml"}},
	{"pre-commit hooks", []string{".pre-commit-config.yaml"}},
	{"codeguard", []string{".codeguard/codeguard.yaml", ".codeguard/codeguard.yml", ".codeguard/codeguard.json", "codeguard.yaml", "codeguard.yml", "codeguard.json"}},
}

// pyprojectToolFacts are tools configured by a [tool.<name>] table in
// pyproject.toml, used when the tool has no standalone config file.
var pyprojectToolFacts = []struct{ tool, section string }{
	{"Ruff", "tool.ruff"},
	{"Black", "tool.black"},
	{"isort", "tool.isort"},
	{"mypy", "tool.mypy"},
	{"pytest", "tool.pytest"},
}

// packageJSONToolFacts are tools configured by a top-level key in the root
// package.json.
var packageJSONToolFacts = []struct{ tool, key string }{
	{"Prettier", "prettier"},
	{"ESLint", "eslintConfig"},
}

// detectConventions reports only what configuration files state: which
// formatters, linters and toolchain versions the repository pins.
func detectConventions(root string) []string {
	var lines []string
	configured := map[string]bool{}
	for _, fact := range configFacts {
		for _, candidate := range fact.candidates {
			if isFile(root, candidate) {
				lines = append(lines, fmt.Sprintf("%s: configured in `%s`.", fact.tool, candidate))
				configured[fact.tool] = true
				break
			}
		}
	}
	if sections, ok := pyprojectSections(root); ok {
		for _, fact := range pyprojectToolFacts {
			if !configured[fact.tool] && hasSectionPrefix(sections, fact.section) {
				lines = append(lines, fmt.Sprintf("%s: configured in `pyproject.toml` [%s].", fact.tool, fact.section))
				configured[fact.tool] = true
			}
		}
	}
	lines = append(lines, packageJSONConventions(root, configured)...)
	lines = append(lines, goModConventions(root)...)
	return lines
}

func packageJSONConventions(root string, configured map[string]bool) []string {
	text, ok := readSmallFile(root, "package.json")
	if !ok {
		return nil
	}
	var manifest map[string]json.RawMessage
	if json.Unmarshal([]byte(text), &manifest) != nil {
		return nil
	}
	var lines []string
	for _, fact := range packageJSONToolFacts {
		if _, ok := manifest[fact.key]; ok && !configured[fact.tool] {
			lines = append(lines, fmt.Sprintf("%s: configured in `package.json` (%q key).", fact.tool, fact.key))
		}
	}
	var engines map[string]string
	if raw, ok := manifest["engines"]; ok && json.Unmarshal(raw, &engines) == nil {
		if node := strings.TrimSpace(engines["node"]); node != "" && !strings.ContainsAny(node, "`\n") {
			lines = append(lines, fmt.Sprintf("Node.js version: `%s` (`package.json` engines.node).", node))
		}
	}
	return lines
}

var (
	goModuleLine    = regexp.MustCompile(`(?m)^module\s+(\S+)\s*$`)
	goDirectiveLine = regexp.MustCompile(`(?m)^go\s+([0-9][0-9A-Za-z.]*)\s*$`)
)

func goModConventions(root string) []string {
	text, ok := readSmallFile(root, "go.mod")
	if !ok {
		return nil
	}
	var lines []string
	if match := goModuleLine.FindStringSubmatch(text); match != nil {
		lines = append(lines, fmt.Sprintf("Go module path: `%s` (`go.mod`).", match[1]))
	}
	if match := goDirectiveLine.FindStringSubmatch(text); match != nil {
		lines = append(lines, fmt.Sprintf("Go language version: %s (`go.mod` go directive).", match[1]))
	}
	return lines
}
