package ci

import (
	"path/filepath"

	"github.com/devr-tools/codeguard/codeguard/core"
)

func Evaluate(cfg core.Config, _ core.ScanScope) core.SectionResult {
	if !cfg.Checks.CI {
		return core.SectionResult{
			Name:   "CI/CD",
			Status: core.StatusSkip,
			Note:   "Disabled in config.",
		}
	}

	result := core.SectionResult{
		Name:   "CI/CD",
		Status: core.StatusPass,
		Note:   "Workflow and release policy checks passed.",
	}
	rules := resolveCIRules(cfg.Checks.CIRules)

	for _, target := range cfg.Targets {
		findings, severity := inspectRepository(target.Path, rules)
		if len(findings) == 0 {
			continue
		}
		recordFindings(&result, findings, severity)
	}

	if result.Status == core.StatusPass {
		result.Findings = append(result.Findings, core.Finding{
			Path:     filepath.ToSlash("codeguard"),
			Message:  "Required CI/CD repository assets are present",
			Severity: core.SeverityInfo,
		})
	}
	return result
}

func recordFindings(result *core.SectionResult, findings []core.Finding, severity core.Severity) {
	result.Findings = append(result.Findings, findings...)
	switch severity {
	case core.SeverityError:
		result.Status = core.StatusFail
		result.Note = "Required CI/CD assets are missing."
	case core.SeverityWarn:
		if result.Status == core.StatusPass {
			result.Status = core.StatusWarn
			result.Note = "Reviewable CI/CD policy issues detected."
		}
	}
}
