package security

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/codeguard/core"
)

func looksLikeGovulncheckVulnerability(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "vulnerability") || strings.Contains(lower, "vulnerable")
}

func firstMeaningfulLine(text string, fallback string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return fallback
}

func parseGovulncheckFindings(output string, targetPath string) []core.Finding {
	lines := strings.Split(output, "\n")
	var findings []core.Finding
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "Vulnerability #") {
			continue
		}

		messageParts := []string{line}
		for j := i + 1; j < len(lines); j++ {
			next := strings.TrimSpace(lines[j])
			if next == "" {
				break
			}
			if strings.HasPrefix(next, "Vulnerability #") {
				break
			}
			if strings.HasPrefix(next, "Found ") {
				break
			}
			messageParts = append(messageParts, next)
			i = j
		}

		findings = append(findings, core.Finding{
			Path:     filepath.ToSlash(targetPath),
			Message:  strings.Join(messageParts, " | "),
			Severity: core.SeverityError,
		})
	}
	return findings
}
