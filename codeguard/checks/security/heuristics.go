package security

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/codeguard/core"
)

var secretPattern = regexp.MustCompile(`(?i)(api[_-]?key|secret|token|password)\s*[:=]\s*["'][^"'$\n]{6,}["']`)

func scanFile(path string, result *core.SectionResult) {
	source, err := os.ReadFile(path)
	if err != nil {
		recordSecurityFinding(result, core.Finding{
			Path:     filepath.ToSlash(path),
			Message:  err.Error(),
			Severity: core.SeverityError,
		})
		return
	}

	if secretPattern.Match(source) {
		recordSecurityFinding(result, core.Finding{
			Path:     filepath.ToSlash(path),
			Message:  "possible hardcoded credential detected",
			Severity: core.SeverityError,
		})
	}

	if bytes.Contains(source, []byte("BEGIN ")) && bytes.Contains(source, []byte("PRIVATE KEY")) {
		recordSecurityFinding(result, core.Finding{
			Path:     filepath.ToSlash(path),
			Message:  "private key material detected",
			Severity: core.SeverityError,
		})
	}

	if strings.Contains(string(source), "InsecureSkipVerify: true") {
		recordSecurityFinding(result, core.Finding{
			Path:     filepath.ToSlash(path),
			Message:  "TLS certificate verification is disabled",
			Severity: core.SeverityError,
		})
	}

	if strings.HasSuffix(path, ".go") && (strings.Contains(string(source), "exec.Command(") || strings.Contains(string(source), "exec.CommandContext(")) {
		recordSecurityFinding(result, core.Finding{
			Path:     filepath.ToSlash(path),
			Message:  "shell execution detected; review command construction and input handling",
			Severity: core.SeverityWarn,
		})
	}
}
