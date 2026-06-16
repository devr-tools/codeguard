package security

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// taintFindingsForFile dispatches source-to-sink taint analysis based on the
// file's language and the configured toggles.
func taintFindingsForFile(env support.Context, file string, source string) []core.Finding {
	rules := env.Config.Checks.SecurityRules
	switch {
	case isGoFile(file) && taintToggleEnabled(rules.TaintGo):
		return goTaintFindings(env, file, source)
	case isPythonFile(file) && taintToggleEnabled(rules.TaintPython):
		return pythonTaintFindings(env, file, source)
	default:
		return nil
	}
}

func taintToggleEnabled(toggle *bool) bool {
	return toggle == nil || *toggle
}

func isGoFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".go")
}

// taintChainMessage renders the source-to-sink chain for a finding message.
func taintChainMessage(source string, sourceLine int, sink string, sinkLine int, chain []string) string {
	steps := append(append([]string{}, chain...), sink)
	return fmt.Sprintf("tainted data from %s (line %d) reaches %s (line %d) via %s",
		source, sourceLine, sink, sinkLine, strings.Join(steps, " -> "))
}

// taintSinkInput describes one source-to-sink flow for finding emission.
type taintSinkInput struct {
	ruleID     string
	source     string
	sourceLine int
	chain      []string
	sink       string
	sinkLine   int
}

// appendTaintFinding appends a deduplicated source-to-sink finding, keyed by
// sink line, sink name, and taint source.
func appendTaintFinding(env support.Context, file string, seen map[string]struct{}, findings []core.Finding, input taintSinkInput) []core.Finding {
	key := fmt.Sprintf("%d:%s:%s", input.sinkLine, input.sink, input.source)
	if _, dup := seen[key]; dup {
		return findings
	}
	seen[key] = struct{}{}
	return append(findings, env.NewFinding(support.FindingInput{
		RuleID:  input.ruleID,
		Level:   "fail",
		Path:    file,
		Line:    input.sinkLine,
		Column:  1,
		Message: taintChainMessage(input.source, input.sourceLine, input.sink, input.sinkLine, input.chain),
	}))
}
