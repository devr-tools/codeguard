package prompts

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/codeguard/core"
)

var (
	secretInterpolationPattern = regexp.MustCompile(`(\$\{[A-Z0-9_]*(KEY|TOKEN|SECRET|PASSWORD)[A-Z0-9_]*\}|\{\{[^}]*?(secret|token|password|api[_-]?key)[^}]*\}\})`)
	unsafeInstructionPatterns  = []*regexp.Regexp{
		regexp.MustCompile(`(?i)ignore (all )?(previous|prior) instructions`),
		regexp.MustCompile(`(?i)reveal (the )?(system prompt|hidden instructions)`),
		regexp.MustCompile(`(?i)bypass (all )?(safety|policy|guardrails)`),
		regexp.MustCompile(`(?i)do not mention (policy|safety|guardrails)`),
	}
)

func scanPromptFile(path string, rules promptRules) ([]core.Finding, core.Severity) {
	source, err := os.ReadFile(path)
	if err != nil {
		return []core.Finding{{
			Path:     filepath.ToSlash(path),
			Message:  err.Error(),
			Severity: core.SeverityError,
		}}, core.SeverityError
	}

	text := string(source)
	var findings []core.Finding
	severity := core.SeverityInfo

	if rules.forbidSecretInterpolation && secretInterpolationPattern.MatchString(text) {
		findings = append(findings, core.Finding{
			Path:     filepath.ToSlash(path),
			Message:  "prompt file appears to interpolate credentials or secret material",
			Severity: core.SeverityError,
		})
		severity = core.SeverityError
	}

	if rules.forbidUnsafeInstructions {
		for _, pattern := range unsafeInstructionPatterns {
			if pattern.MatchString(text) {
				findings = append(findings, core.Finding{
					Path:     filepath.ToSlash(path),
					Message:  "prompt file contains unsafe instruction pattern: " + strings.TrimSpace(pattern.String()),
					Severity: core.SeverityWarn,
				})
			}
		}
		if severity != core.SeverityError && len(findings) > 0 {
			severity = core.SeverityWarn
		}
	}

	if len(findings) == 0 {
		return nil, core.SeverityInfo
	}
	return findings, severity
}
