package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/rulepacktest"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

const rulesTestUsage = `usage: codeguard rules test [-config codeguard.yaml] [-profile name] [-format text|json] <fixture path...>

Runs the config's custom rule packs over annotated fixture files and compares
the findings with the annotations. A comment line (any of //, #, --, ;, /*,
<!--) holding "ruleid: <id>" expects <id> to report on the next line, and
"ok: <id>" expects it not to; "ruleid-file:" / "ok-file:" describe the
file-level findings of path-only rules. Several ids may be listed, comma
separated. Every reported finding needs a ruleid annotation.

A directory is scanned as its own target, so rule path filters match paths
relative to it; a file is scanned relative to its parent directory. Built-in
rules do not run, and natural-language rules are skipped (they need an AI
runtime). Exits 1 when any fixture fails or no fixture holds an annotation.

Flags:`

// runRulesTest implements `codeguard rules test`.
func runRulesTest(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("rules test", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(stderr, rulesTestUsage)
		fs.PrintDefaults()
	}
	configPath := fs.String("config", service.DefaultConfigPath(), "config path holding the rule_packs under test")
	profile := fs.String("profile", "", "optional policy profile override")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}
	outputFormat := strings.TrimSpace(*format)
	if outputFormat != "" && outputFormat != "text" && outputFormat != "json" {
		_, _ = fmt.Fprintf(stderr, "invalid rules test format %q (valid: text, json)\n", outputFormat)
		return exitError
	}
	if fs.NArg() == 0 {
		_, _ = fmt.Fprintln(stderr, "rules test: name at least one fixture file or directory")
		fs.Usage()
		return exitError
	}
	cfg, err := loadConfigWithProfile(*configPath, *profile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
		return exitError
	}
	report, err := rulepacktest.Run(context.Background(), cfg, fs.Args())
	if err != nil && !errors.Is(err, rulepacktest.ErrNoAnnotations) {
		_, _ = fmt.Fprintf(stderr, "rules test: %v\n", err)
		return exitError
	}
	if outputFormat == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if encodeErr := encoder.Encode(report); encodeErr != nil {
			_, _ = fmt.Fprintf(stderr, "write rules test output: %v\n", encodeErr)
			return exitError
		}
	} else {
		writeRulesTestText(stdout, report)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "rules test: %v\n", err)
		return exitError
	}
	if !report.OK() {
		return exitError
	}
	return exitOK
}

func writeRulesTestText(w io.Writer, report rulepacktest.Report) {
	for _, file := range report.Files {
		status := "PASS"
		if !file.Passed {
			status = "FAIL"
		}
		_, _ = fmt.Fprintf(w, "%s %s\n", status, file.Path)
		for _, rule := range file.Rules {
			if line := rulesTestRuleLine(rule, file.Passed); line != "" {
				_, _ = fmt.Fprintln(w, line)
			}
		}
	}
	for _, id := range report.Skipped {
		_, _ = fmt.Fprintf(w, "skipped %s: natural-language rule needs an AI runtime\n", id)
	}
	_, _ = fmt.Fprintf(w, "%d passed, %d failed\n", report.Passed, report.Failed)
}

// rulesTestRuleLine describes one rule of a file: always for a failing file,
// and for a passing one only when it carries a note.
func rulesTestRuleLine(rule rulepacktest.RuleResult, passed bool) string {
	if rule.Note != "" {
		return fmt.Sprintf("  %s: %s", rule.RuleID, rule.Note)
	}
	if passed || (len(rule.Missing) == 0 && len(rule.Unexpected) == 0) {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  %s: expected lines %s, reported lines %s", rule.RuleID, formatLines(rule.Expected), formatLines(rule.Reported))
	for _, line := range rule.Missing {
		fmt.Fprintf(&b, "\n    missing: no finding on line %s", formatLine(line))
	}
	for _, line := range rule.Unexpected {
		reason := "no ruleid annotation"
		if containsLine(rule.OK, line) {
			reason = "annotated ok"
		}
		fmt.Fprintf(&b, "\n    unexpected: finding on line %s (%s)", formatLine(line), reason)
	}
	return b.String()
}

func formatLines(lines []int) string {
	if len(lines) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		parts = append(parts, formatLine(line))
	}
	return strings.Join(parts, ", ")
}

func formatLine(line int) string {
	switch {
	case line == 0:
		return "0 (file-level)"
	case line < 0:
		return "<none: annotation at end of file>"
	default:
		return strconv.Itoa(line)
	}
}

func containsLine(lines []int, want int) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}
