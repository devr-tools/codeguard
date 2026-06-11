package prompts

import (
	"path/filepath"

	"github.com/devr-tools/codeguard/codeguard/core"
)

func Evaluate(cfg core.Config, scope core.ScanScope) core.SectionResult {
	if !cfg.Checks.Prompts {
		return core.SectionResult{
			Name:   "AI Prompts",
			Status: core.StatusSkip,
			Note:   "Disabled in config.",
		}
	}

	result := core.SectionResult{
		Name:   "AI Prompts",
		Status: core.StatusPass,
		Note:   "Prompt discovery and safety checks passed.",
	}
	rules := resolvePromptRules(cfg.Checks.PromptRules)
	for _, target := range cfg.Targets {
		files, err := discoverPromptFiles(target.Path, rules, scope)
		if err != nil {
			result.Status = core.StatusFail
			result.Note = "Unable to enumerate prompt files."
			result.Findings = append(result.Findings, core.Finding{
				Path:     filepath.ToSlash(target.Path),
				Message:  err.Error(),
				Severity: core.SeverityError,
			})
			continue
		}

		if len(files) == 0 {
			continue
		}
		for _, file := range files {
			findings, severity := scanPromptFile(file, rules)
			if len(findings) == 0 {
				continue
			}
			recordPromptFindings(&result, findings, severity)
		}
	}

	if result.Status == core.StatusPass {
		result.Findings = append(result.Findings, core.Finding{
			Path:     filepath.ToSlash("codeguard"),
			Message:  "No prompt safety violations detected in discovered prompt assets",
			Severity: core.SeverityInfo,
		})
	}
	return result
}

func recordPromptFindings(result *core.SectionResult, findings []core.Finding, severity core.Severity) {
	result.Findings = append(result.Findings, findings...)
	switch severity {
	case core.SeverityError:
		result.Status = core.StatusFail
		result.Note = "Blocking prompt safety violations detected."
	case core.SeverityWarn:
		if result.Status == core.StatusPass {
			result.Status = core.StatusWarn
			result.Note = "Reviewable prompt safety patterns detected."
		}
	}
}
