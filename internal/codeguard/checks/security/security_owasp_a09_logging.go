package security

import (
	"regexp"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	goLoggingCallPattern     = regexp.MustCompile(`\b(?:log|logger|slog|zap|logrus)\.(?:Print|Info|Error|Debug|Warn|Fatal|Panic)[A-Za-z]*\s*\(`)
	pythonLoggingCallPattern = regexp.MustCompile(`\b(?:logging|logger|log)\.(?:debug|info|warning|warn|error|critical|exception|log)\s*\(|\bprint\s*\(`)
	scriptLoggingCallPattern = regexp.MustCompile(`\b(?:console|logger|log)\.(?:log|info|warn|error|debug|trace|verbose)\s*\(`)
)

func a09LoggingCallPattern(language a09Language) *regexp.Regexp {
	switch language {
	case a09LanguageGo:
		return goLoggingCallPattern
	case a09LanguagePython:
		return pythonLoggingCallPattern
	default:
		return scriptLoggingCallPattern
	}
}

// logSecretExposureFinding reports at most one finding per line: the first
// logging call on the line whose argument list carries a secret-bearing value.
// The requirement that the signal sit inside the call's argument span is what
// keeps nearby mentions (comments, adjacent statements) from firing.
func logSecretExposureFinding(env support.Context, line a09Line) *core.Finding {
	pattern := a09LoggingCallPattern(line.language)
	for _, span := range callArgumentSpans(line.masked, pattern) {
		if !secretBearingArgs(line.masked[span[0]:span[1]], line.raw[span[0]:span[1]]) {
			continue
		}
		return newA09Finding(env, line, "security.log-secret-exposure",
			"secret-bearing value passed to a logging call; log a redacted or derived value instead")
	}
	return nil
}

// callArgumentSpans returns the [start,end) byte ranges of the argument lists
// of every pattern match on the masked line. Patterns must end at an opening
// parenthesis; each span runs to the balancing close paren, or to the end of
// the line for calls that continue on later lines.
func callArgumentSpans(masked string, pattern *regexp.Regexp) [][2]int {
	matches := pattern.FindAllStringIndex(masked, -1)
	spans := make([][2]int, 0, len(matches))
	for _, match := range matches {
		start := match[1]
		depth := 1
		end := len(masked)
		for i := start; i < len(masked); i++ {
			switch masked[i] {
			case '(':
				depth++
			case ')':
				depth--
			}
			if depth == 0 {
				end = i
				break
			}
		}
		spans = append(spans, [2]int{start, end})
	}
	return spans
}

// secretBearingArgs reports whether a logging call's argument list carries a
// secret. Heuristics, in order:
//
//	H1: an identifier in the masked argument list has a secret-named component
//	    (f-string and template-literal interpolations stay visible after
//	    masking, so f"{token}" and `${token}` are caught here);
//	H2: a short whitespace-free string literal naming a secret is used as a
//	    structured-logging key (immediately followed by a comma);
//	H3: a string literal containing a secret keyword is concatenated with `+`
//	    to a non-literal expression (e.g. "Authorization: Bearer " + tok);
//	H4: a string literal embeds "<keyword>=" or "<keyword>:" directly followed
//	    by a string-valued format directive (%s/%v/%q or '{').
//
// A secret keyword appearing only inside a plain literal (e.g. "token count:
// %d") matches none of these and does not fire.
func secretBearingArgs(masked string, raw string) bool {
	if textHasSecretIdentifier(masked) {
		return true
	}
	for _, literal := range scanArgLiterals(raw) {
		if literalIsSecretExposure(literal) {
			return true
		}
	}
	return false
}
