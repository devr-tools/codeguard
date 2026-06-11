package security

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/codeguard/core"
)

func runGovulncheck(targetPath string, rules securityRules, result *core.SectionResult) {
	if rules.govulncheckMode == "off" {
		return
	}

	commandPath, err := exec.LookPath(rules.govulncheckCommand)
	if err != nil {
		finding := core.Finding{
			Path:     filepath.ToSlash(targetPath),
			Message:  fmt.Sprintf("%s is not installed", rules.govulncheckCommand),
			Severity: core.SeverityWarn,
		}
		if rules.govulncheckMode == "required" {
			finding.Severity = core.SeverityError
		}
		recordSecurityFinding(result, finding)
		return
	}

	cmd := exec.CommandContext(context.Background(), commandPath, "./...")
	cmd.Dir = targetPath
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err == nil {
		result.Findings = append(result.Findings, core.Finding{
			Path:     filepath.ToSlash(targetPath),
			Message:  "govulncheck completed without reported vulnerabilities",
			Severity: core.SeverityInfo,
		})
		return
	}

	if looksLikeGovulncheckVulnerability(trimmed) {
		findings := parseGovulncheckFindings(trimmed, targetPath)
		if len(findings) == 0 {
			findings = []core.Finding{{
				Path:     filepath.ToSlash(targetPath),
				Message:  firstMeaningfulLine(trimmed, "govulncheck reported vulnerabilities"),
				Severity: core.SeverityError,
			}}
		}
		for _, finding := range findings {
			recordSecurityFinding(result, finding)
		}
		return
	}

	if rules.govulncheckMode == "required" {
		recordSecurityFinding(result, core.Finding{
			Path:     filepath.ToSlash(targetPath),
			Message:  fmt.Sprintf("govulncheck execution failed: %v", err),
			Severity: core.SeverityError,
		})
		return
	}

	if !errors.Is(err, exec.ErrNotFound) {
		recordSecurityFinding(result, core.Finding{
			Path:     filepath.ToSlash(targetPath),
			Message:  fmt.Sprintf("govulncheck could not complete: %s", firstMeaningfulLine(trimmed, err.Error())),
			Severity: core.SeverityWarn,
		})
	}
}
