package checks

import (
	"context"
	"fmt"

	ciCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/ci"
	contractsCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/contracts"
	designCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/design"
	promptsCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/prompts"
	qualityCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/quality"
	securityCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/security"
	supplyChainCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/supplychain"
	checkSupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	customrunner "github.com/devr-tools/codeguard/internal/codeguard/runner/custom"
	govulncheckrunner "github.com/devr-tools/codeguard/internal/codeguard/runner/govulncheck"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func Build(ctx context.Context, sc runnersupport.Context) []core.SectionResult {
	sections := make([]core.SectionResult, 0, 7)
	checkEnv := buildCheckContext(sc)
	if sc.Cfg.Checks.Quality {
		sections = append(sections, safeRun("quality", "Quality", func() core.SectionResult { return qualityCheck.Run(ctx, checkEnv) }))
	}
	if sc.Cfg.Checks.Design {
		sections = append(sections, safeRun("design", "Design", func() core.SectionResult { return designCheck.Run(ctx, checkEnv) }))
	}
	if sc.Cfg.Checks.Security {
		sections = append(sections, safeRun("security", "Security", func() core.SectionResult { return securityCheck.Run(ctx, checkEnv) }))
	}
	if sc.Cfg.Checks.Prompts {
		sections = append(sections, safeRun("prompts", "Prompts", func() core.SectionResult { return promptsCheck.Run(ctx, checkEnv) }))
	}
	if sc.Cfg.Checks.CI {
		sections = append(sections, safeRun("ci", "CI", func() core.SectionResult { return ciCheck.Run(ctx, checkEnv) }))
	}
	if sc.Cfg.Checks.SupplyChain {
		sections = append(sections, safeRun("supply-chain", "Supply Chain", func() core.SectionResult { return supplyChainCheck.Run(ctx, checkEnv) }))
	}
	if contractsEnabled(sc) {
		sections = append(sections, safeRun("contracts", "Contracts", func() core.SectionResult { return contractsCheck.Run(ctx, checkEnv) }))
	}
	if len(sc.CustomRules) > 0 {
		sections = append(sections, safeRun("custom", "Custom Rules", func() core.SectionResult { return customrunner.RunSection(ctx, sc) }))
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
		RunCommandCheck: func(ctx context.Context, dir string, check core.CommandCheckConfig) (string, error) {
			return runnersupport.RunCommandCheck(ctx, dir, check)
		},
		RunCommandCheckWithEnv: func(ctx context.Context, dir string, check core.CommandCheckConfig, env []string) (string, error) {
			return runnersupport.RunCommandCheckWithEnv(ctx, dir, check, env)
		},
		RunDiffCommandCheck: func(ctx context.Context, dir string, baseRef string, check core.CommandCheckConfig) (string, error) {
			return runnersupport.RunDiffCommandCheckWithContext(ctx, sc, dir, baseRef, check)
		},
		NormalizedSeverity: runnersupport.NormalizedSeverity,
	}
}
