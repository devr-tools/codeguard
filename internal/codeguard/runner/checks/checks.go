package checks

import (
	"context"

	ciCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/ci"
	contractsCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/contracts"
	designCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/design"
	promptsCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/prompts"
	qualityCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/quality"
	securityCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/security"
	checkSupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	customrunner "github.com/devr-tools/codeguard/internal/codeguard/runner/custom"
	govulncheckrunner "github.com/devr-tools/codeguard/internal/codeguard/runner/govulncheck"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func Build(ctx context.Context, sc runnersupport.Context) []core.SectionResult {
	sections := make([]core.SectionResult, 0, 6)
	checkEnv := buildCheckContext(sc)
	if sc.Cfg.Checks.Quality {
		sections = append(sections, qualityCheck.Run(ctx, checkEnv))
	}
	if sc.Cfg.Checks.Design {
		sections = append(sections, designCheck.Run(ctx, checkEnv))
	}
	if sc.Cfg.Checks.Security {
		sections = append(sections, securityCheck.Run(ctx, checkEnv))
	}
	if sc.Cfg.Checks.Prompts {
		sections = append(sections, promptsCheck.Run(ctx, checkEnv))
	}
	if sc.Cfg.Checks.CI {
		sections = append(sections, ciCheck.Run(ctx, checkEnv))
	}
	if contractsEnabled(sc) {
		sections = append(sections, contractsCheck.Run(ctx, checkEnv))
	}
	if len(sc.CustomRules) > 0 {
		sections = append(sections, customrunner.RunSection(ctx, sc))
	}
	return sections
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
		RunDiffCommandCheck: func(ctx context.Context, dir string, baseRef string, check core.CommandCheckConfig) (string, error) {
			return runnersupport.RunDiffCommandCheckWithContext(ctx, sc, dir, baseRef, check)
		},
		NormalizedSeverity: runnersupport.NormalizedSeverity,
	}
}
