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
