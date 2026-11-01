package contextgen

import (
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/agentcontext"
)

// maxWorkflowCommands bounds the commands taken from all CI workflows.
const maxWorkflowCommands = 15

// workflowTools are the first words of CI run lines worth documenting: build,
// test, lint and dependency tools a developer can run locally. Deploy,
// release and shell plumbing lines are left out.
var workflowTools = map[string]bool{
	"make": true, "npm": true, "pnpm": true, "yarn": true, "npx": true,
	"go": true, "gofmt": true, "golangci-lint": true, "cargo": true,
	"pytest": true, "python": true, "python3": true, "uv": true, "poetry": true,
	"ruff": true, "mypy": true, "black": true, "tox": true, "nox": true,
}

type workflowDoc struct {
	Defaults workflowDefaults       `yaml:"defaults"`
	Jobs     map[string]workflowJob `yaml:"jobs"`
}

type workflowDefaults struct {
	Run struct {
		WorkingDirectory string `yaml:"working-directory"`
	} `yaml:"run"`
}

type workflowJob struct {
	Defaults workflowDefaults `yaml:"defaults"`
	Steps    []workflowStep   `yaml:"steps"`
}

type workflowStep struct {
	Run              string `yaml:"run"`
	WorkingDirectory string `yaml:"working-directory"`
}

// workflowCommands takes single-line run steps from each workflow that run
// at the repository root and only invoke make targets and package scripts
// the root build files define, so documenting them can never introduce a
// drift finding.
func workflowCommands(root string, workflows []string, surface agentcontext.CommandSurface) []Command {
	var commands []Command
	seen := map[string]struct{}{}
	for _, workflow := range workflows {
		text, ok := readSmallFile(root, workflow)
		if !ok {
			continue
		}
		var doc workflowDoc
		if yaml.Unmarshal([]byte(text), &doc) != nil || doc.Defaults.Run.WorkingDirectory != "" {
			continue
		}
		for _, line := range workflowRunLines(doc) {
			if len(commands) >= maxWorkflowCommands {
				return commands
			}
			if _, dup := seen[line]; dup || !documentableRunLine(root, line, surface) {
				continue
			}
			seen[line] = struct{}{}
			commands = append(commands, Command{Invocation: line, Source: workflow})
		}
	}
	return commands
}

// workflowRunLines returns run steps in job-name order so output is
// deterministic (YAML maps are unordered once decoded).
func workflowRunLines(doc workflowDoc) []string {
	names := make([]string, 0, len(doc.Jobs))
	for name := range doc.Jobs {
		names = append(names, name)
	}
	sort.Strings(names)
	var lines []string
	for _, name := range names {
		job := doc.Jobs[name]
		if job.Defaults.Run.WorkingDirectory != "" {
			continue
		}
		for _, step := range job.Steps {
			line := strings.TrimSpace(step.Run)
			if line == "" || step.WorkingDirectory != "" || strings.Contains(line, "\n") {
				continue
			}
			lines = append(lines, line)
		}
	}
	return lines
}

// documentableRunLine accepts a run line only when it is plain (no
// expressions, variables, directory changes or redirections), starts with a
// known tool, and every make target, package script and ./ path it names
// resolves at the repository root.
func documentableRunLine(root string, line string, surface agentcontext.CommandSurface) bool {
	if strings.ContainsAny(line, "$`<>\\'\"") {
		return false
	}
	for _, command := range splitRunLine(line) {
		fields := strings.Fields(command)
		if len(fields) == 0 || !workflowTools[fields[0]] {
			return false
		}
		if !runFieldsResolve(root, fields, surface) {
			return false
		}
	}
	return true
}

func splitRunLine(line string) []string {
	for _, sep := range []string{"&&", "||", ";", "|"} {
		line = strings.ReplaceAll(line, sep, "\n")
	}
	return strings.Split(line, "\n")
}

func runFieldsResolve(root string, fields []string, surface agentcontext.CommandSurface) bool {
	switch fields[0] {
	case "make":
		if !makeArgsResolve(fields[1:], surface) {
			return false
		}
	case "npm", "pnpm", "yarn":
		if !npmArgsResolve(root, fields[1:], surface) {
			return false
		}
	}
	for _, field := range fields[1:] {
		if !strings.HasPrefix(field, "./") || strings.Contains(field, "...") {
			continue
		}
		if !isFile(root, field) && !isDir(root, field) {
			return false
		}
	}
	return true
}

func makeArgsResolve(args []string, surface agentcontext.CommandSurface) bool {
	targets := setOf(surface.MakeTargets)
	for _, arg := range args {
		switch {
		case arg == "-C" || arg == "-f" || strings.HasPrefix(arg, "--"):
			return false
		case strings.HasPrefix(arg, "-") || strings.Contains(arg, "="):
			continue
		default:
			if _, ok := targets[arg]; !ok {
				return false
			}
		}
	}
	return true
}

func npmArgsResolve(root string, args []string, surface agentcontext.CommandSurface) bool {
	if !isFile(root, "package.json") {
		return false
	}
	if len(args) >= 2 && (args[0] == "run" || args[0] == "run-script") {
		_, ok := setOf(surface.NpmScripts)[args[1]]
		return ok
	}
	return true
}
