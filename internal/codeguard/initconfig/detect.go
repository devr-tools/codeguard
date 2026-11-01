// Package initconfig derives a repository-aware starter configuration from
// what is actually in a repository: which languages it is written in, where
// its entrypoints are, whether it has GitHub Actions workflows and which
// package manifests it declares.
//
// It backs both `codeguard init` (which writes the result as a small config
// file) and the config-less `codeguard scan` fallback (which resolves the same
// result in memory), so a first scan without a config and a first scan with a
// freshly generated config apply the same policy.
package initconfig

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	checksupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// Layout is what detection learned about a repository. Every path in it is
// slash-separated and relative to the detected root.
type Layout struct {
	// Languages holds one entry per language that gets its own target,
	// ordered by file count (largest first).
	Languages []LanguageStat
	// Minor holds languages that were seen but fell below the target
	// threshold, so a stray helper script does not add a whole target.
	Minor []LanguageStat
	// Workflows lists the GitHub Actions workflow files found under
	// .github/workflows.
	Workflows []string
	// Manifests lists the package manifests the supply-chain section reads.
	Manifests []string
	// SourceFiles is the number of source files counted.
	SourceFiles int
}

// LanguageStat describes one detected language.
type LanguageStat struct {
	// Language is the value written to targets[].language.
	Language string
	// Files is the number of source files counted for the language.
	Files int
	// Entrypoints are root-relative entrypoint paths for the language.
	Entrypoints []string
}

const (
	// A language gets its own target when it holds at least this share of the
	// counted source files, or at least minorLanguageFileCount files, so one
	// helper script in a Go repository does not add a Python target.
	minorLanguageShare     = 0.10
	minorLanguageFileCount = 25
	maxEntrypointsPerLang  = 20
)

// languageByExtension maps a source extension to its language group. The
// TypeScript and JavaScript extensions share one group because codeguard
// analyzes them with the same target language.
var languageByExtension = map[string]string{
	".go":   "go",
	".py":   "python",
	".ts":   "typescript",
	".tsx":  "typescript",
	".mts":  "typescript",
	".cts":  "typescript",
	".js":   "javascript",
	".jsx":  "javascript",
	".mjs":  "javascript",
	".cjs":  "javascript",
	".rs":   "rust",
	".java": "java",
	".cc":   "cpp",
	".cpp":  "cpp",
	".cxx":  "cpp",
	".hpp":  "cpp",
	".hh":   "cpp",
	".hxx":  "cpp",
	".cs":   "csharp",
	".rb":   "ruby",
}

// detectionExcludes exists because counting the wrong files picks the wrong
// targets: hidden directories hold tool state and whole editor or agent
// worktrees (a second copy of the repository), and testdata/fixtures hold
// deliberately foreign sample files. The scanner's dependency and build
// exclusions (node_modules, vendor, dist, .venv, ...) apply in addition.
var detectionExcludes = []string{
	".*", "**/.*",
	"testdata", "**/testdata",
	"fixtures", "**/fixtures",
	"__fixtures__", "**/__fixtures__",
}

// Detect inspects the repository rooted at root.
func Detect(root string) (Layout, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	info, err := os.Stat(root)
	if err != nil {
		return Layout{}, err
	}
	if !info.IsDir() {
		return Layout{}, fmt.Errorf("%s is not a directory", root)
	}

	counts := map[string]int{}
	var manifests, pythonMains []string
	_, err = runnersupport.WalkFiles(root, detectionExcludes, func(rel string) bool {
		if checksupport.IsSupplyChainManifest(rel) {
			manifests = append(manifests, rel)
		}
		lang, ok := languageByExtension[strings.ToLower(path.Ext(rel))]
		if !ok {
			return false
		}
		counts[lang]++
		if path.Base(rel) == "__main__.py" {
			pythonMains = append(pythonMains, rel)
		}
		return true
	})
	if err != nil {
		return Layout{}, err
	}

	sort.Strings(manifests)
	layout := Layout{Manifests: manifests, Workflows: detectWorkflows(root)}
	entrypoints := detectEntrypoints(root, manifests, pythonMains)
	layout.Languages, layout.Minor, layout.SourceFiles = classifyLanguages(counts, entrypoints)
	return layout, nil
}

// classifyLanguages folds JavaScript into TypeScript, applies the target
// threshold, and orders the result by file count.
func classifyLanguages(counts map[string]int, entrypoints map[string][]string) (major []LanguageStat, minor []LanguageStat, total int) {
	if js := counts["javascript"]; js > 0 && counts["typescript"] > 0 {
		counts["typescript"] += js
		delete(counts, "javascript")
	}
	stats := make([]LanguageStat, 0, len(counts))
	for lang, files := range counts {
		total += files
		stats = append(stats, LanguageStat{Language: lang, Files: files, Entrypoints: entrypoints[entrypointGroup(lang)]})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Files != stats[j].Files {
			return stats[i].Files > stats[j].Files
		}
		return stats[i].Language < stats[j].Language
	})
	for i, stat := range stats {
		share := float64(stat.Files) / float64(total)
		if i == 0 || share >= minorLanguageShare || stat.Files >= minorLanguageFileCount {
			major = append(major, stat)
			continue
		}
		stat.Entrypoints = nil
		minor = append(minor, stat)
	}
	return major, minor, total
}

func entrypointGroup(lang string) string {
	if lang == "javascript" {
		return "typescript"
	}
	return lang
}

func detectWorkflows(root string) []string {
	entries, err := os.ReadDir(filepath.Join(root, ".github", "workflows"))
	if err != nil {
		return nil
	}
	workflows := make([]string, 0, len(entries))
	for _, entry := range entries {
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if entry.IsDir() || (ext != ".yml" && ext != ".yaml") {
			continue
		}
		workflows = append(workflows, ".github/workflows/"+entry.Name())
	}
	return workflows
}

// fileExists reports whether rel names a regular file under root.
func fileExists(root string, rel string) bool {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil && info.Mode().IsRegular()
}

// readRootFile reads a small repository file named by a root-relative path.
func readRootFile(root string, rel string) ([]byte, bool) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) //nolint:gosec // rel comes from walking root
	if err != nil {
		return nil, false
	}
	return data, true
}
