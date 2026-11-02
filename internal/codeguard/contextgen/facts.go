// Package contextgen writes and refreshes AI-agent instruction files
// (AGENTS.md, CLAUDE.md) from facts codeguard can detect deterministically.
//
// The agent-context check family lints those files; this package generates
// them from the same parsers, so a freshly generated file passes the family's
// own rules by construction. Nothing is inferred beyond what a build file,
// manifest, config file or directory listing states: every generated line is
// either a detected fact or an explicit "TODO:" for a human to fill in.
package contextgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/initconfig"
)

// Facts is everything detection learned about a repository, in render order.
type Facts struct {
	// Name is the repository directory's base name.
	Name string
	// Languages lists detected source languages by file count, largest first.
	Languages []LanguageFact
	// Manifests lists the package manifests found, root-relative.
	Manifests []string
	// Workflows lists the GitHub Actions workflow files found.
	Workflows []string
	// Commands lists build, test and lint commands with where each came from.
	Commands []Command
	// Directories lists the top-level directories; an empty Purpose renders
	// as a TODO.
	Directories []Directory
	// Entrypoints lists detected program entrypoints.
	Entrypoints []Entrypoint
	// Tests lists where test files live, grouped by top-level directory.
	Tests []TestLocation
	// Conventions lists facts read from formatter, linter and toolchain
	// configuration files.
	Conventions []string
}

// LanguageFact is one detected language and its source file count.
type LanguageFact struct {
	Name  string
	Files int
}

// Command is one runnable command and the file that proves it exists.
type Command struct {
	// Invocation is the command line, run from the repository root.
	Invocation string
	// Source names the file or tool the command was read from, such as
	// "Makefile" or ".github/workflows/ci.yml". Commands render grouped by it.
	Source string
}

// Directory is one top-level directory.
type Directory struct {
	// Path is the directory name with a trailing slash.
	Path string
	// Purpose is a one-line description, set only when the name and contents
	// agree unambiguously.
	Purpose string
	// Contents summarizes what the directory holds (for example "12 Go
	// files"); it is a count, never an interpretation.
	Contents string
}

// Entrypoint is one program entrypoint.
type Entrypoint struct {
	Path     string
	Language string
}

// TestLocation is a top-level directory (or "." for the root) holding test
// files, with how many and in which languages.
type TestLocation struct {
	Path      string
	Files     int
	Languages []string
}

// Detect inspects the repository rooted at root.
func Detect(root string) (Facts, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Facts{}, err
	}
	layout, err := initconfig.Detect(absRoot)
	if err != nil {
		return Facts{}, err
	}
	tree, err := walkTree(absRoot)
	if err != nil {
		return Facts{}, err
	}
	facts := Facts{
		Name:        projectName(absRoot),
		Languages:   languageFacts(layout),
		Manifests:   capList(layout.Manifests, maxListedManifests),
		Workflows:   layout.Workflows,
		Directories: detectDirectories(absRoot, tree),
		Entrypoints: entrypointFacts(absRoot, layout),
		Tests:       tree.testLocations(),
	}
	facts.Commands = detectCommands(absRoot, layout, tree)
	facts.Conventions = detectConventions(absRoot)
	return facts, nil
}

const maxListedManifests = 10

func languageFacts(layout initconfig.Layout) []LanguageFact {
	facts := make([]LanguageFact, 0, len(layout.Languages)+len(layout.Minor))
	for _, group := range [][]initconfig.LanguageStat{layout.Languages, layout.Minor} {
		for _, stat := range group {
			facts = append(facts, LanguageFact{Name: languageDisplayName(stat.Language), Files: stat.Files})
		}
	}
	return facts
}

func entrypointFacts(root string, layout initconfig.Layout) []Entrypoint {
	var entries []Entrypoint
	for _, stat := range layout.Languages {
		for _, rel := range stat.Entrypoints {
			entries = append(entries, Entrypoint{Path: entrypointDisplayPath(root, rel), Language: languageDisplayName(stat.Language)})
		}
	}
	return entries
}

// entrypointDisplayPath gives package-directory entrypoints a trailing slash,
// which is what lets the drift rule verify the rendered path still exists.
func entrypointDisplayPath(root string, rel string) string {
	if isDir(root, rel) {
		return rel + "/"
	}
	return rel
}

var languageNames = map[string]string{
	"go":         "Go",
	"python":     "Python",
	"typescript": "TypeScript",
	"javascript": "JavaScript",
	"rust":       "Rust",
	"java":       "Java",
	"cpp":        "C++",
	"csharp":     "C#",
	"ruby":       "Ruby",
}

func languageDisplayName(lang string) string {
	if name, ok := languageNames[lang]; ok {
		return name
	}
	return lang
}

func capList(list []string, limit int) []string {
	if len(list) > limit {
		return append([]string(nil), list[:limit]...)
	}
	return append([]string(nil), list...)
}

func isDir(root string, rel string) bool {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil && info.IsDir()
}

func isFile(root string, rel string) bool {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil && info.Mode().IsRegular()
}

// readSmallFile reads a root-relative file, refusing anything over 1 MiB:
// every file read here is a build, manifest or config file.
func readSmallFile(root string, rel string) (string, bool) {
	path := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return "", false
	}
	data, err := os.ReadFile(path) //nolint:gosec // fixed config file names joined under the repository root
	if err != nil {
		return "", false
	}
	return string(data), true
}

func pluralize(count int, singular string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %ss", count, singular)
}
