package security

import (
	"path/filepath"

	"github.com/devr-tools/codeguard/codeguard/checks/support"
	"github.com/devr-tools/codeguard/codeguard/core"
)

func Evaluate(cfg core.Config, scope core.ScanScope) core.SectionResult {
	if !cfg.Checks.Security {
		return core.SectionResult{
			Name:   "Security",
			Status: core.StatusSkip,
			Note:   "Disabled in config.",
		}
	}

	result := core.SectionResult{
		Name:   "Security",
		Status: core.StatusPass,
		Note:   "No hardcoded secrets or insecure TLS settings detected.",
	}
	rules := resolveSecurityRules(cfg.Checks.SecurityRules)
	for _, target := range cfg.Targets {
		if !support.IsGoTarget(target) {
			continue
		}

		files, err := support.ScopedCandidateTextFiles(target.Path, scope)
		if err != nil {
			result.Status = core.StatusFail
			result.Note = "Unable to enumerate files for security checks."
			result.Findings = append(result.Findings, core.Finding{
				Path:     filepath.ToSlash(target.Path),
				Message:  err.Error(),
				Severity: core.SeverityError,
			})
			continue
		}

		for _, file := range files {
			scanFile(file, &result)
		}

		runGovulncheck(target.Path, rules, &result)
	}

	if result.Status == core.StatusPass {
		result.Findings = append(result.Findings, core.Finding{
			Path:     filepath.ToSlash("codeguard"),
			Message:  "Repository scan completed without blocking security findings",
			Severity: core.SeverityInfo,
		})
	}
	return result
}

func recordSecurityFinding(result *core.SectionResult, finding core.Finding) {
	result.Findings = append(result.Findings, finding)
	switch finding.Severity {
	case core.SeverityError:
		result.Status = core.StatusFail
		result.Note = "Blocking security findings detected."
	case core.SeverityWarn:
		if result.Status == core.StatusPass {
			result.Status = core.StatusWarn
			result.Note = "Reviewable security-sensitive patterns detected."
		}
	default:
	}
}
