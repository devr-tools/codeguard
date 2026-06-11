package targets

import (
	"fmt"

	"github.com/devr-tools/codeguard/codeguard/checks/support"
	"github.com/devr-tools/codeguard/codeguard/core"
)

func Evaluate(cfg core.Config, _ core.ScanScope) core.SectionResult {
	result := core.SectionResult{
		Name:   "Repository Targets",
		Status: core.StatusPass,
		Note:   "All configured targets are present.",
	}

	for _, state := range support.CollectTargetStates(cfg) {
		if state.Err != nil {
			result.Status = core.StatusFail
			result.Note = "One or more configured targets are missing."
			result.Findings = append(result.Findings, core.Finding{
				Path:     state.Target.Path,
				Message:  state.Err.Error(),
				Severity: core.SeverityError,
			})
			continue
		}

		entryType := "file"
		if state.Info != nil && state.Info.IsDir() {
			entryType = "directory"
		}
		result.Findings = append(result.Findings, core.Finding{
			Path:     state.Target.Path,
			Message:  fmt.Sprintf("%s target %q resolved", entryType, state.Target.Name),
			Severity: core.SeverityInfo,
		})
	}

	return result
}
