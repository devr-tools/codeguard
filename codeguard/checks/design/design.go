package design

import (
	"path/filepath"

	"github.com/devr-tools/codeguard/codeguard/checks/support"
	"github.com/devr-tools/codeguard/codeguard/core"
)

func Evaluate(cfg core.Config, scope core.ScanScope) core.SectionResult {
	if !cfg.Checks.Design {
		return core.SectionResult{
			Name:   "Design Patterns",
			Status: core.StatusSkip,
			Note:   "Disabled in config.",
		}
	}

	result := core.SectionResult{
		Name:   "Design Patterns",
		Status: core.StatusPass,
		Note:   "Architecture layer rules passed.",
	}
	rules := resolveDesignRules(cfg.Checks.DesignRules)
	for _, target := range cfg.Targets {
		if !support.IsGoTarget(target) {
			continue
		}

		files, err := support.ScopedGoFiles(target.Path, scope)
		if err != nil {
			result.Status = core.StatusFail
			result.Note = "Unable to enumerate Go files for design checks."
			result.Findings = append(result.Findings, core.Finding{
				Path:     filepath.ToSlash(target.Path),
				Message:  err.Error(),
				Severity: core.SeverityError,
			})
			continue
		}

		for _, file := range files {
			findings, severity := inspectFile(target.Path, file, rules)
			if len(findings) == 0 {
				continue
			}
			recordDesignFindings(&result, findings, severity)
		}
	}

	if result.Status == core.StatusPass {
		result.Findings = append(result.Findings, core.Finding{
			Path:     filepath.ToSlash("codeguard"),
			Message:  "Layer boundaries held across scanned Go packages",
			Severity: core.SeverityInfo,
		})
	}
	return result
}

func recordDesignFindings(result *core.SectionResult, findings []core.Finding, severity core.Severity) {
	result.Findings = append(result.Findings, findings...)
	switch severity {
	case core.SeverityError:
		result.Status = core.StatusFail
		result.Note = "Architecture boundary violations detected."
	case core.SeverityWarn:
		if result.Status == core.StatusPass {
			result.Status = core.StatusWarn
			result.Note = "Design principle drift detected."
		}
	}
}
