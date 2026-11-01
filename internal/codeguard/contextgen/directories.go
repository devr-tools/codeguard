package contextgen

import (
	"os"
	"sort"
	"strings"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// maxListedDirectories bounds the layout block for repositories with a very
// wide root.
const maxListedDirectories = 30

// directoryRule states a purpose for a conventional directory name, and the
// content evidence that must back the name before the purpose is claimed.
// A name without matching evidence gets a TODO instead.
type directoryRule struct {
	purpose  string
	evidence func(d *dirStats, tests *testStats) bool
}

var directoryRules = map[string]directoryRule{
	"cmd": {"Go main packages, one binary per subdirectory", func(d *dirStats, _ *testStats) bool {
		return d.subdirsWith("main.go") > 0
	}},
	"internal":  {"Go packages private to this module (Go internal/ visibility rule)", hasLanguage("Go")},
	"pkg":       {"Go packages importable by other modules", hasLanguage("Go")},
	"docs":      {"Documentation", mostlyDocs},
	"doc":       {"Documentation", mostlyDocs},
	"tests":     {"Tests", hasTests},
	"test":      {"Tests", hasTests},
	"__tests__": {"Tests", hasTests},
	"spec":      {"Tests", hasTests},
	"e2e":       {"End-to-end tests", hasTests},
	"testdata":  {"Test fixtures (Go testdata convention)", func(*dirStats, *testStats) bool { return true }},
	"examples":  {"Examples", func(*dirStats, *testStats) bool { return true }},
	"example":   {"Examples", func(*dirStats, *testStats) bool { return true }},
	"migrations": {"Database migrations", func(d *dirStats, _ *testStats) bool {
		return d.extShare(".sql") >= 0.5
	}},
	"terraform": {"Terraform configuration", func(d *dirStats, _ *testStats) bool {
		return d.extShare(".tf", ".tfvars", ".hcl") >= 0.5
	}},
	"charts": {"Helm charts", func(d *dirStats, _ *testStats) bool {
		return d.subdirsWith("Chart.yaml") > 0
	}},
	"packages": {"Workspace packages, one package.json per subdirectory", func(d *dirStats, _ *testStats) bool {
		return d.subdirsWith("package.json") > 0
	}},
	"apps": {"Workspace applications, one package.json per subdirectory", func(d *dirStats, _ *testStats) bool {
		return d.subdirsWith("package.json") > 0
	}},
	"crates": {"Rust workspace crates, one Cargo.toml per subdirectory", func(d *dirStats, _ *testStats) bool {
		return d.subdirsWith("Cargo.toml") > 0
	}},
}

func hasLanguage(lang string) func(*dirStats, *testStats) bool {
	return func(d *dirStats, _ *testStats) bool { return d.byLanguage[lang] > 0 }
}

func hasTests(_ *dirStats, tests *testStats) bool {
	return tests != nil && tests.files > 0
}

func mostlyDocs(d *dirStats, _ *testStats) bool {
	return d.extShare(".md", ".mdx", ".rst", ".txt", ".adoc") >= 0.5
}

// detectDirectories lists non-hidden top-level directories the scanner would
// walk, with a purpose only where a directoryRule's evidence holds.
func detectDirectories(root string, tree treeStats) []Directory {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || strings.HasPrefix(name, ".") || runnersupport.ShouldExclude(name, nil) {
			continue
		}
		if tree.dirs[name] == nil {
			// Empty, or every file in it is ignored: nothing an agent reads.
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	names = capList(names, maxListedDirectories)
	dirs := make([]Directory, 0, len(names))
	for _, name := range names {
		stats := tree.dirs[name]
		dir := Directory{Path: name + "/", Contents: stats.languagesSummary()}
		if rule, ok := directoryRules[name]; ok && rule.evidence(stats, tree.tests[name]) {
			dir.Purpose = rule.purpose
		}
		dirs = append(dirs, dir)
	}
	return dirs
}
