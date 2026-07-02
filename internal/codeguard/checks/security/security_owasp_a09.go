package security

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// a09Language selects the per-language patterns for the A09 (Security Logging
// and Monitoring Failures) heuristics.
type a09Language int

const (
	a09LanguageNone a09Language = iota
	a09LanguageGo
	a09LanguagePython
	a09LanguageScript // TypeScript and JavaScript
)

// a09Line bundles the raw and masked forms of one source line for the A09
// heuristics. Masked lines have comments and string contents blanked with
// byte offsets preserved, so span indexes are valid on both forms.
type a09Line struct {
	file     string
	lineNo   int
	raw      string
	masked   string
	language a09Language
}

// a09FindingsForFile runs the A09 heuristics for one file: secret-bearing
// values passed to logging calls, and raw error values written into HTTP
// responses. Patterns match masked source (comments and string contents
// blanked) so identifiers inside comments or plain literals cannot fire; the
// raw line is consulted only for the literal heuristics documented on
// secretBearingArgs.
func a09FindingsForFile(env support.Context, file string, source string) []core.Finding {
	language := a09LanguageForFile(file)
	if language == a09LanguageNone {
		return nil
	}
	rawLines := strings.Split(source, "\n")
	maskedLines := strings.Split(a09MaskedSource(language, source), "\n")

	findings := make([]core.Finding, 0)
	excepts := pythonExceptTracker{}
	for idx, raw := range rawLines {
		line := a09Line{file: file, lineNo: idx + 1, raw: raw, masked: maskedLines[idx], language: language}
		if finding := logSecretExposureFinding(env, line); finding != nil {
			findings = append(findings, *finding)
		}
		if finding := unsanitizedErrorResponseFinding(env, line, &excepts); finding != nil {
			findings = append(findings, *finding)
		}
	}
	return findings
}

func a09LanguageForFile(file string) a09Language {
	switch {
	case isGoFile(file):
		return a09LanguageGo
	case isPythonFile(file):
		return a09LanguagePython
	case isTypeScriptFile(file):
		return a09LanguageScript
	default:
		return a09LanguageNone
	}
}

func a09MaskedSource(language a09Language, source string) string {
	switch language {
	case a09LanguageGo:
		return support.MaskCLikeSource(source, support.CLikeGo)
	case a09LanguagePython:
		return support.MaskPythonSource(source)
	default:
		return support.MaskCLikeSource(source, support.CLikeTypeScript)
	}
}

func newA09Finding(env support.Context, line a09Line, ruleID string, message string) *core.Finding {
	finding := env.NewFinding(support.FindingInput{RuleID: ruleID, Level: "warn", Path: line.file, Line: line.lineNo, Column: 1, Message: message})
	return &finding
}
