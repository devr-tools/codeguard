package fix

import (
	"context"
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/runner"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func GenerateVerified(ctx context.Context, req GenerateRequest) (Result, error) {
	if req.Generator == nil {
		return Result{}, fmt.Errorf("fix generator is required")
	}
	candidate, err := req.Generator.GenerateFix(ctx, GenerateInput{
		Config:       req.Config,
		Finding:      req.Finding,
		Analysis:     req.Analysis,
		Instructions: "Return a unified diff that resolves the finding without unrelated edits. The patch will only be surfaced if codeguard verification and nearest tests pass.",
	})
	if err != nil {
		return Result{}, err
	}
	return Verify(ctx, req.Config, req.Finding, candidate, req.Options)
}

func Verify(ctx context.Context, cfg core.Config, finding core.Finding, candidate Candidate, opts Options) (Result, error) {
	diffText := strings.TrimSpace(candidate.Diff)
	if diffText == "" {
		return Result{}, fmt.Errorf("candidate diff is required")
	}

	report, err := runner.RunWithOptions(ctx, cfg, core.ScanOptions{
		Mode:     core.ScanModeDiff,
		BaseRef:  verificationBaseRef(opts),
		DiffText: diffText,
	})
	if err != nil {
		return Result{}, fmt.Errorf("verify codeguard checks: %w", err)
	}
	if report.Summary.TotalFindings > 0 {
		return Result{}, fmt.Errorf("patch did not verify cleanly: %d changed-line findings remain", report.Summary.TotalFindings)
	}

	patchedCfg, _, cleanup, err := runnersupport.MaterializePatchedTargets(cfg, diffText)
	if err != nil {
		return Result{}, fmt.Errorf("materialize patched targets: %w", err)
	}
	defer cleanup()

	changedByTarget := changedFilesByTarget(cfg.Targets, diffText)
	testPlan, err := buildTestPlan(cfg, patchedCfg, changedByTarget, opts)
	if err != nil {
		return Result{}, err
	}
	if len(testPlan) == 0 {
		return Result{}, fmt.Errorf("no verification tests could be inferred; provide explicit test commands")
	}

	results, err := runVerificationTests(ctx, testPlan)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Summary:      strings.TrimSpace(candidate.Summary),
		Diff:         diffText,
		Report:       report,
		ChangedFiles: flattenChangedFiles(changedByTarget),
		TestResults:  results,
	}, nil
}

func runVerificationTests(ctx context.Context, plan []testStep) ([]CommandResult, error) {
	results := make([]CommandResult, 0, len(plan))
	for _, step := range plan {
		output, err := runnersupport.RunCommandCheck(ctx, step.dir, step.check)
		result := CommandResult{
			TargetName: step.target.Name,
			CheckName:  step.check.Name,
			Command:    joinCommand(step.check),
			Output:     strings.TrimSpace(output),
		}
		results = append(results, result)
		if err == nil {
			continue
		}
		if result.Output != "" {
			return nil, fmt.Errorf("verification test %q failed for target %q: %s", step.check.Name, step.target.Name, result.Output)
		}
		return nil, fmt.Errorf("verification test %q failed for target %q: %w", step.check.Name, step.target.Name, err)
	}
	return results, nil
}
