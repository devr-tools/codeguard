package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// runContext dispatches `codeguard context <init|update>`: generating and
// refreshing agent instruction files from detected repository facts.
func runContext(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		writeContextUsage(stderr)
		return exitError
	}
	switch args[0] {
	case "init":
		return runContextInit(args[1:], stdout, stderr)
	case "update":
		return runContextUpdate(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		writeContextUsage(stdout)
		return exitOK
	default:
		_, _ = fmt.Fprintf(stderr, "unknown context command %q\n\n", args[0])
		writeContextUsage(stderr)
		return exitError
	}
}

func writeContextUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "usage:")
	_, _ = fmt.Fprintln(w, "  codeguard context init   [-path .] [-file AGENTS.md] [-force] [-dry-run] [-config path]")
	_, _ = fmt.Fprintln(w, "  codeguard context update [-path .] [-file AGENTS.md|CLAUDE.md] [-dry-run] [-config path]")
}

// contextFlags are the flags both subcommands share.
type contextFlags struct {
	root       *string
	file       *string
	dryRun     *bool
	configPath *string
}

func registerContextFlags(fs *flag.FlagSet, fileDefault string, fileHelp string) contextFlags {
	return contextFlags{
		root:       fs.String("path", ".", "repository root to detect facts from"),
		file:       fs.String("file", fileDefault, fileHelp),
		dryRun:     fs.Bool("dry-run", false, "print instead of writing"),
		configPath: fs.String("config", "", "optional codeguard config whose context_rules tune the post-write agent-context check"),
	}
}

// docPath resolves -file against the repository root unless it is absolute.
func (f contextFlags) docPath() string {
	if filepath.IsAbs(*f.file) {
		return *f.file
	}
	return filepath.Join(*f.root, *f.file)
}

func runContextInit(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("context init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := registerContextFlags(fs, "AGENTS.md", "file to write, relative to -path (AGENTS.md or CLAUDE.md)")
	force := fs.Bool("force", false, "overwrite an existing file")
	if ok, code := parseFlags(fs, args, stderr); !ok {
		return code
	}
	facts, err := service.DetectContextFacts(*flags.root)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "context init: detect repository facts: %v\n", err)
		return exitError
	}
	content := service.RenderContextDoc(facts)
	if *flags.dryRun {
		_, _ = io.WriteString(stdout, content)
		return exitOK
	}
	path := flags.docPath()
	if _, statErr := os.Stat(path); statErr == nil && !*force {
		_, _ = fmt.Fprintf(stderr, "context init: %s already exists; use `codeguard context update` to refresh its codeguard-managed blocks, or -force to overwrite it\n", path)
		return exitError
	}
	if err = writeContextDoc(path, content); err != nil {
		_, _ = fmt.Fprintf(stderr, "context init: write %s: %v\n", path, err)
		return exitError
	}
	_, _ = fmt.Fprintf(stdout, "wrote %s (%d lines; fill in the TODO lines by hand)\n", path, strings.Count(content, "\n"))
	return reportContextCheck(*flags.root, *flags.configPath, stdout, stderr)
}

func runContextUpdate(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("context update", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := registerContextFlags(fs, "", "file to refresh, relative to -path (default: AGENTS.md, else CLAUDE.md)")
	if ok, code := parseFlags(fs, args, stderr); !ok {
		return code
	}
	if *flags.file == "" {
		*flags.file = defaultContextDoc(*flags.root)
	}
	path := flags.docPath()
	data, err := os.ReadFile(path) //nolint:gosec // operator-named agent doc under the repository root
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "context update: read %s: %v (run `codeguard context init` to create it)\n", path, err)
		return exitError
	}
	existing := string(data)
	facts, err := service.DetectContextFacts(*flags.root)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "context update: detect repository facts: %v\n", err)
		return exitError
	}
	result, err := service.UpdateContextDoc(existing, facts)
	if errors.Is(err, service.ErrContextNoMarkers) {
		return reportMissingMarkers(*flags.file, existing, facts, stdout, stderr)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "context update: %s: %v\n", path, err)
		return exitError
	}
	writeUpdateSummary(stderr, result)
	if *flags.dryRun || len(result.Changed) == 0 {
		_, _ = io.WriteString(stdout, service.ContextDocDiff(*flags.file, existing, result.Content))
		if len(result.Changed) == 0 {
			_, _ = fmt.Fprintf(stdout, "%s is up to date\n", path)
		}
		return exitOK
	}
	if err = writeContextDoc(path, result.Content); err != nil {
		_, _ = fmt.Fprintf(stderr, "context update: write %s: %v\n", path, err)
		return exitError
	}
	_, _ = fmt.Fprintf(stdout, "updated %s (blocks: %s)\n", path, strings.Join(result.Changed, ", "))
	return reportContextCheck(*flags.root, *flags.configPath, stdout, stderr)
}

// defaultContextDoc picks the agent doc update refreshes when -file is not
// given: AGENTS.md when it exists, otherwise CLAUDE.md.
func defaultContextDoc(root string) string {
	if info, err := os.Stat(filepath.Join(root, "AGENTS.md")); err == nil && info.Mode().IsRegular() {
		return "AGENTS.md"
	}
	if info, err := os.Stat(filepath.Join(root, "CLAUDE.md")); err == nil && info.Mode().IsRegular() {
		return "CLAUDE.md"
	}
	return "AGENTS.md"
}

// reportMissingMarkers shows what update would add to a file with no managed
// blocks and asks the operator to add markers; it never writes.
func reportMissingMarkers(file string, existing string, facts service.ContextFacts, stdout io.Writer, stderr io.Writer) int {
	proposal := service.ProposeContextSections(existing, facts)
	_, _ = io.WriteString(stdout, service.ContextDocDiff(file, existing, proposal))
	names := service.ContextBlockNames()
	_, _ = fmt.Fprintf(stderr, "context update: %s has no codeguard-managed blocks, so nothing was written.\n", file)
	_, _ = fmt.Fprintf(stderr, "The diff above shows what codeguard would manage. Wrap the sections you want refreshed in markers, for example:\n")
	_, _ = fmt.Fprintf(stderr, "  <!-- codeguard:begin %s -->\n  ...\n  <!-- codeguard:end %s -->\n", names[1], names[1])
	_, _ = fmt.Fprintf(stderr, "Managed block names: %s\n", strings.Join(names, ", "))
	return exitError
}

func writeUpdateSummary(w io.Writer, result service.ContextDocUpdate) {
	if len(result.Unknown) > 0 {
		_, _ = fmt.Fprintf(w, "note: left unknown codeguard blocks untouched: %s\n", strings.Join(result.Unknown, ", "))
	}
	if len(result.Missing) > 0 {
		_, _ = fmt.Fprintf(w, "note: no markers for blocks %s; add codeguard:begin/end markers to manage them\n", strings.Join(result.Missing, ", "))
	}
}

// writeContextDoc writes an agent doc, keeping an existing file's permissions.
func writeContextDoc(path string, content string) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(path, []byte(content), mode) //nolint:gosec // agent docs are committed, world-readable documentation
}

// reportContextCheck runs the agent-context section on the repository and
// prints the legibility score and any remaining findings. Findings are
// informational: the write already succeeded.
func reportContextCheck(root string, configPath string, stdout io.Writer, stderr io.Writer) int {
	var rules service.ContextRulesConfig
	if configPath != "" {
		cfg, ok := loadConfigOrFail(configPath, "", stderr)
		if !ok {
			return exitError
		}
		rules = cfg.Checks.ContextRules
	}
	result, err := service.CheckContextDocs(context.Background(), root, rules)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "context check: %v\n", err)
		return exitError
	}
	_, _ = fmt.Fprintf(stdout, "agent-context check: legibility score %d/100\n", result.Score)
	for _, component := range result.Components {
		_, _ = fmt.Fprintf(stdout, "  %-28s %d/%d\n", component.Label, component.Score, component.Max)
	}
	if len(result.Findings) == 0 {
		_, _ = fmt.Fprintln(stdout, "no agent-context findings")
		return exitOK
	}
	_, _ = fmt.Fprintf(stdout, "%d agent-context finding(s):\n", len(result.Findings))
	for _, finding := range result.Findings {
		location := finding.Path
		if location != "" && finding.Line > 0 {
			location = fmt.Sprintf("%s:%d", finding.Path, finding.Line)
		}
		_, _ = fmt.Fprintf(stdout, "  [%s] %s %s: %s\n", finding.Level, finding.RuleID, location, finding.Message)
	}
	return exitOK
}
