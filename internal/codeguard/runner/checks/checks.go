package checks

import (
	"context"
	"fmt"

	checkSupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	govulncheckrunner "github.com/devr-tools/codeguard/internal/codeguard/runner/govulncheck"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func Build(ctx context.Context, sc runnersupport.Context) []core.SectionResult {
	sections := make([]core.SectionResult, 0, len(sectionRegistry))
	checkEnv := buildCheckContext(sc) //nolint:contextcheck // git helpers use a contained timeout; deeper ctx threading is a tracked follow-up
	for _, def := range sectionRegistry {
		if !def.enabled(sc) {
			continue
		}
		sections = append(sections, safeRun(def.id, def.name, func() core.SectionResult {
			return def.run(ctx, sc, checkEnv)
		}))
	}
	return sections
}

// safeRun executes one check section, recovering from any panic so that a single
// failing check (e.g. an index-out-of-range while parsing an untrusted file)
// degrades to a diagnostic warning for that section instead of aborting the
// entire scan. On panic it returns a section bearing a single warning finding
// and the remaining sections still run.
func safeRun(id string, name string, fn func() core.SectionResult) (result core.SectionResult) {
	defer func() {
		if r := recover(); r != nil {
			result = core.SectionResult{
				ID:     id,
				Name:   name,
				Status: core.StatusWarn,
				Findings: []core.Finding{{
					RuleID:  "checks.section.panic",
					Level:   "warning",
					Section: id,
					Message: fmt.Sprintf("%s check did not complete: internal error (%v)", name, r),
				}},
			}
		}
	}()
	return fn()
}

// contractsEnabled resolves the contracts toggle: an explicit config value
// wins, otherwise the family is enabled only for diff scans.
func contractsEnabled(sc runnersupport.Context) bool {
	if sc.Cfg.Checks.Contracts != nil {
		return *sc.Cfg.Checks.Contracts
	}
	return sc.Opts.Mode == core.ScanModeDiff
}

func buildCheckContext(sc runnersupport.Context) checkSupport.Context {
	return checkSupport.Context{
		Config:    sc.Cfg,
		AIEnabled: sc.Opts.EnableAI || (sc.Cfg.AI.Enabled != nil && *sc.Cfg.AI.Enabled),
		Mode:      sc.Opts.Mode,
		BaseRef:   sc.Opts.BaseRef,
		DiffText:  sc.Opts.DiffText,
		ScanTime:  sc.Today,
		ListChangedFiles: func(target core.TargetConfig) ([]core.ChangedFile, error) {
			return runnersupport.ListChangedFiles(sc, target)
		},
		ReadBaseFile: func(target core.TargetConfig, rel string) ([]byte, error) {
			return runnersupport.ReadBaseFile(sc, target, rel)
		},
		ChangedFiles: runnersupport.ChangedDiffFiles(sc),
		VisitTargetFiles: func(target core.TargetConfig, include func(string) bool, visit func(rel string, data []byte)) {
			runnersupport.VisitTargetFiles(sc, target, include, visit)
		},
		DiffScope: func() map[string]core.ChangedLineRanges {
			out := make(map[string]core.ChangedLineRanges, len(sc.Diff))
			for path, ranges := range sc.Diff {
				out[path] = ranges.Export()
			}
			return out
		},
		ScanTargetFiles: func(target core.TargetConfig, sectionID string, include func(string) bool, evaluator func(string, []byte) []core.Finding) []core.Finding {
			return runnersupport.ScanTargetFiles(sc, target, sectionID, include, evaluator)
		},
		NewFinding: func(input checkSupport.FindingInput) core.Finding {
			return runnersupport.NewFinding(sc, runnersupport.FindingInput{
				RuleID:  input.RuleID,
				Level:   input.Level,
				Path:    input.Path,
				Line:    input.Line,
				Column:  input.Column,
				Message: input.Message,
			})
		},
		FinalizeSection: func(id string, name string, findings []core.Finding) core.SectionResult {
			return runnersupport.FinalizeSection(sc, id, name, findings)
		},
		PutArtifact: func(artifact core.Artifact) {
			sc.Artifacts.Put(artifact)
		},
		GetArtifact: func(id string) (core.Artifact, bool) {
			return sc.Artifacts.Get(id)
		},
		CountLines:           runnersupport.CountLines,
		CyclomaticComplexity: runnersupport.CyclomaticComplexity,
		TypeName:             runnersupport.TypeName,
		IsInternalOrCmdFile:  runnersupport.IsInternalOrCmdFile,
		IsCmdFile:            runnersupport.IsCmdFile,
		IsPublicPackageFile:  runnersupport.IsPublicPackageFile,
		IsSDKFacadeFile:      runnersupport.IsSDKFacadeFile,
		IsPromptFile: func(rel string) bool {
			return runnersupport.IsPromptFile(sc, rel)
		},
		RunGovulncheck: func(ctx context.Context, dir string, cmdName string) ([]core.Finding, error) {
			return govulncheckrunner.Run(ctx, dir, cmdName, sc)
		},
		RunCommandCheck:        runnersupport.RunCommandCheck,
		RunCommandCheckWithEnv: runnersupport.RunCommandCheckWithEnv,
		RunDiffCommandCheck: func(ctx context.Context, dir string, baseRef string, check core.CommandCheckConfig) (string, error) {
			return runnersupport.RunDiffCommandCheckWithContext(ctx, sc, dir, baseRef, check)
		},
		NormalizedSeverity: runnersupport.NormalizedSeverity,
	}
}
