package runner

import (
	"github.com/devr-tools/codeguard/internal/codeguard/config"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// attachRunDiagnostics reports operational diagnostics raised after every
// section has finalized (history writes for rule stats and the waiver audit).
// They are attached to the first section, mirroring how corpus diagnostics go
// to the first section that finalizes, and raise its status the same way
// FinalizeSectionWithDiagnostics does.
func attachRunDiagnostics(sections []core.SectionResult, diagnostics []core.Diagnostic) {
	if len(sections) == 0 {
		return
	}
	attachSectionDiagnostics(&sections[0], diagnostics)
}

// attachScoreHistoryDiagnostics reports score-history write failures on the
// section that published the score artifact, falling back to the first
// section when that section is absent from the report.
func attachScoreHistoryDiagnostics(sections []core.SectionResult, diagnostics []runnersupport.SectionDiagnostic) {
	for _, item := range diagnostics {
		index := sectionIndex(sections, item.SectionID)
		if index < 0 {
			attachRunDiagnostics(sections, []core.Diagnostic{item.Diagnostic})
			continue
		}
		attachSectionDiagnostics(&sections[index], []core.Diagnostic{item.Diagnostic})
	}
}

func sectionIndex(sections []core.SectionResult, id string) int {
	for index := range sections {
		if sections[index].ID == id {
			return index
		}
	}
	return -1
}

func attachSectionDiagnostics(section *core.SectionResult, diagnostics []core.Diagnostic) {
	if len(diagnostics) == 0 {
		return
	}
	section.Diagnostics = append(section.Diagnostics, diagnostics...)
	for _, diagnostic := range diagnostics {
		if !diagnostic.Operational {
			continue
		}
		switch diagnostic.Level {
		case "fail":
			section.Status = core.StatusFail
		case "warn":
			if section.Status != core.StatusFail {
				section.Status = core.StatusWarn
			}
		}
	}
}

func WriteBaselineFile(path string, entries []core.BaselineEntry) error {
	return runnersupport.WriteBaselineFile(path, entries)
}

func SlopHistoryPath(cfg core.Config) string {
	config.ApplyDefaults(&cfg)
	return runnersupport.SlopHistoryPathForBase(cfg.Cache.Path)
}

func LoadSlopHistory(path string) map[string][]core.SlopHistoryEntry {
	return runnersupport.LoadSlopHistory(path)
}

func PerfScoreHistoryPath(cfg core.Config) string {
	config.ApplyDefaults(&cfg)
	return runnersupport.PerfScoreHistoryPathForBase(cfg.Cache.Path)
}

func LoadPerfScoreHistory(path string) map[string][]core.PerformanceHistoryEntry {
	return runnersupport.LoadPerfScoreHistory(path)
}

func RuleStatsHistoryPath(cfg core.Config) string {
	config.ApplyDefaults(&cfg)
	return runnersupport.RuleStatsHistoryPathForBase(cfg.Cache.Path)
}

func LoadRuleStatsHistory(path string) []core.RuleStatsHistoryEntry {
	return runnersupport.LoadRuleStatsHistory(path)
}

func WaiverAuditHistoryPath(cfg core.Config) string {
	config.ApplyDefaults(&cfg)
	return runnersupport.WaiverAuditHistoryPathForBase(cfg.Cache.Path)
}

func LoadWaiverAuditHistory(path string) []core.WaiverAuditHistoryEntry {
	return runnersupport.LoadWaiverAuditHistory(path)
}

func BaselineEntriesFromReport(report core.Report) []core.BaselineEntry {
	return runnersupport.BaselineEntriesFromReport(report)
}
