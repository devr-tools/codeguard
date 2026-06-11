package languages

import (
	"fmt"

	"github.com/devr-tools/codeguard/codeguard/checks/support"
	"github.com/devr-tools/codeguard/codeguard/core"
)

func Evaluate(cfg core.Config, _ core.ScanScope) core.SectionResult {
	result := core.SectionResult{
		Name:   "Language Support",
		Status: core.StatusPass,
		Note:   "All configured targets use supported runtimes.",
	}

	for _, target := range cfg.Targets {
		if support.IsGoTarget(target) {
			result.Findings = append(result.Findings, core.Finding{
				Path:     target.Path,
				Message:  "Go target enabled for first-class support",
				Severity: core.SeverityInfo,
			})
			continue
		}

		result.Status = core.StatusWarn
		result.Note = "Some targets are declared for future language support."
		result.Findings = append(result.Findings, core.Finding{
			Path:     target.Path,
			Message:  fmt.Sprintf("language %q is not implemented yet", target.Language),
			Severity: core.SeverityWarn,
		})
	}

	return result
}
