package githubaction

import (
	"strings"
)

// Suggestion is an exact replacement for new-file lines StartLine..EndLine of
// a finding's file. Empty Lines deletes those lines.
type Suggestion struct {
	StartLine int
	EndLine   int
	Lines     []string
}

// SuggestionFunc returns an exact, deterministic replacement for a finding,
// reading the current line text from the PR diff. It returns false whenever
// the replacement would need judgment; a guided fix is never a suggestion.
type SuggestionFunc func(finding Finding, diff DiffLines) (Suggestion, bool)

// DeterministicSuggestion is the default SuggestionFunc. It knows the
// line-local fixes whose replacement follows mechanically from the source
// line, and declines everything else.
func DeterministicSuggestion(finding Finding, diff DiffLines) (Suggestion, bool) {
	switch finding.RuleID {
	case "quality.typescript.debugger-statement", "quality.javascript.debugger-statement":
		return deleteDebuggerStatement(finding, diff)
	default:
		return Suggestion{}, false
	}
}

// deleteDebuggerStatement removes a line that holds nothing but a debugger
// statement. A debugger statement sharing its line with other code (for
// example "if (x) debugger;") is left to the reviewer.
func deleteDebuggerStatement(finding Finding, diff DiffLines) (Suggestion, bool) {
	text, ok := diff.Text(finding.Path, finding.Line)
	if !ok {
		return Suggestion{}, false
	}
	switch strings.TrimSpace(text) {
	case "debugger", "debugger;":
		return Suggestion{StartLine: finding.Line, EndLine: finding.Line}, true
	default:
		return Suggestion{}, false
	}
}

// suggestionAllowed reports whether a finding may carry a suggestion block at
// all. A suggestion echoes source text, so security findings, which can sit on
// a credential, never get one.
func suggestionAllowed(finding Finding) bool {
	if strings.HasPrefix(finding.RuleID, "security.") {
		return false
	}
	_, secret := finding.Metadata["secret_type"]
	return !secret
}
