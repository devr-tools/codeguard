package contextgen

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/agentcontext"
	"github.com/devr-tools/codeguard/internal/codeguard/initconfig"
)

// maxListedMakeTargets bounds the Makefile group; the canonical targets
// context.undocumented-commands requires are always kept.
const maxListedMakeTargets = 30

// maxListedScripts bounds the package.json group the same way.
const maxListedScripts = 30

const (
	sourceMakefile    = "Makefile"
	sourcePackageJSON = "package.json"
	sourceGo          = "go.mod"
	sourceCargo       = "Cargo.toml"
	sourcePyproject   = "pyproject.toml"
)

// detectCommands collects commands from each build file that proves them, in
// a fixed source order: Makefile, package.json, go.mod, Cargo.toml,
// pyproject.toml, then CI workflows.
func detectCommands(root string, layout initconfig.Layout, tree treeStats) []Command {
	surface := agentcontext.ReadCommandSurface(root)
	groups := [][]Command{
		makeCommands(surface),
		npmCommands(root, surface),
		goCommands(root, tree),
		cargoCommands(root, tree),
		pyprojectCommands(root),
		workflowCommands(root, layout.Workflows, surface),
	}
	total := 0
	for _, group := range groups {
		total += len(group)
	}
	commands := make([]Command, 0, total)
	for _, group := range groups {
		commands = append(commands, group...)
	}
	return dedupeCommands(commands)
}

// makeCommands lists the Makefile's plain targets in file order. Targets
// that name files (dist/app, foo.o) and private helpers (_x, .X) are left
// out; canonical names always survive the cap.
func makeCommands(surface agentcontext.CommandSurface) []Command {
	canonical := setOf(agentcontext.CanonicalCommandNames())
	var picked []string
	others := 0
	for _, target := range surface.MakeTargets {
		if !listableCommandName(target) {
			continue
		}
		if _, isCanonical := canonical[target]; !isCanonical {
			if others >= maxListedMakeTargets {
				continue
			}
			others++
		}
		picked = append(picked, target)
	}
	commands := make([]Command, 0, len(picked))
	for _, target := range picked {
		commands = append(commands, Command{Invocation: "make " + target, Source: sourceMakefile})
	}
	return commands
}

func listableCommandName(name string) bool {
	return agentcontext.IsResolvableCommandWord(name) &&
		!strings.ContainsAny(name, "/.") && !strings.HasPrefix(name, "_")
}

// npmCommands lists root package.json scripts under the package manager the
// lockfile names.
func npmCommands(root string, surface agentcontext.CommandSurface) []Command {
	if len(surface.NpmScripts) == 0 {
		return nil
	}
	manager := packageManager(root)
	canonical := setOf(agentcontext.CanonicalCommandNames())
	var commands []Command
	others := 0
	for _, script := range surface.NpmScripts {
		if !agentcontext.IsResolvableCommandWord(script) {
			continue
		}
		if _, isCanonical := canonical[script]; !isCanonical {
			if others >= maxListedScripts {
				continue
			}
			others++
		}
		commands = append(commands, Command{Invocation: manager + " run " + script, Source: sourcePackageJSON})
	}
	return commands
}

// packageManager picks the tool from the lockfile; npm is the default when
// no pnpm or yarn lockfile exists.
func packageManager(root string) string {
	switch {
	case isFile(root, "pnpm-lock.yaml"):
		return "pnpm"
	case isFile(root, "yarn.lock"):
		return "yarn"
	default:
		return "npm"
	}
}

// goCommands lists the Go toolchain's whole-module commands when go.mod sits
// at the root; go test is listed only when _test.go files exist.
func goCommands(root string, tree treeStats) []Command {
	if !isFile(root, "go.mod") {
		return nil
	}
	commands := []Command{
		{Invocation: "go build ./...", Source: sourceGo},
		{Invocation: "go vet ./...", Source: sourceGo},
	}
	if tree.hasTestFiles("Go") {
		commands = append(commands, Command{Invocation: "go test ./...", Source: sourceGo})
	}
	return commands
}

// cargoCommands mirrors goCommands for a root Cargo.toml.
func cargoCommands(root string, tree treeStats) []Command {
	if !isFile(root, "Cargo.toml") {
		return nil
	}
	commands := []Command{{Invocation: "cargo build", Source: sourceCargo}}
	if hasRustSources(tree) {
		commands = append(commands, Command{Invocation: "cargo test", Source: sourceCargo})
	}
	return commands
}

// hasRustSources gates cargo test on the crate having any Rust code: Rust
// unit tests live inline (#[cfg(test)]), so no file naming convention marks
// them the way _test.go does for Go.
func hasRustSources(tree treeStats) bool {
	for _, dir := range tree.dirs {
		if dir.byLanguage["Rust"] > 0 {
			return true
		}
	}
	return false
}

var tomlSection = regexp.MustCompile(`(?m)^\s*\[([^\]]+)\]\s*$`)

// pyprojectSections returns the TOML table names declared in pyproject.toml.
func pyprojectSections(root string) (map[string]struct{}, bool) {
	text, ok := readSmallFile(root, "pyproject.toml")
	if !ok {
		return nil, false
	}
	sections := map[string]struct{}{}
	for _, match := range tomlSection.FindAllStringSubmatch(text, -1) {
		sections[strings.TrimSpace(match[1])] = struct{}{}
	}
	return sections, true
}

func hasSectionPrefix(sections map[string]struct{}, prefix string) bool {
	for name := range sections {
		if name == prefix || strings.HasPrefix(name, prefix+".") {
			return true
		}
	}
	return false
}

// pyprojectCommands lists the tools pyproject.toml (or the tool's own config
// file) configures, run through the environment manager its lockfile names.
func pyprojectCommands(root string) []Command {
	sections, ok := pyprojectSections(root)
	if !ok {
		return nil
	}
	runner := pythonRunner(root)
	var commands []Command
	add := func(configured bool, invocation string) {
		if configured {
			commands = append(commands, Command{Invocation: runner + invocation, Source: sourcePyproject})
		}
	}
	add(hasSectionPrefix(sections, "tool.pytest") || isFile(root, "pytest.ini"), "pytest")
	add(hasSectionPrefix(sections, "tool.ruff") || isFile(root, "ruff.toml") || isFile(root, ".ruff.toml"), "ruff check .")
	add(hasSectionPrefix(sections, "tool.black"), "black --check .")
	add(hasSectionPrefix(sections, "tool.mypy") || isFile(root, "mypy.ini"), "mypy .")
	return commands
}

func pythonRunner(root string) string {
	switch {
	case isFile(root, "uv.lock"):
		return "uv run "
	case isFile(root, "poetry.lock"):
		return "poetry run "
	default:
		return ""
	}
}

func dedupeCommands(commands []Command) []Command {
	seen := map[string]struct{}{}
	out := make([]Command, 0, len(commands))
	for _, command := range commands {
		key := command.Source + "|" + command.Invocation
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, command)
	}
	return out
}

func setOf(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}
