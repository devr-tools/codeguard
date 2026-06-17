package semantic

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var supportedRuleIDs = map[string]struct{}{
	"quality.ai.semantic-doc-mismatch":  {},
	"quality.ai.contract-drift":         {},
	"quality.ai.semantic-error-message": {},
	"quality.ai.semantic-test-coverage": {},
	"quality.ai.semantic-test-adequacy": {},
}

func buildRequest(opts Options) (Request, bool) {
	checks := semanticCheckSpecs(opts.CheckSelection)
	if len(checks) == 0 {
		return Request{}, false
	}
	diffText := strings.TrimSpace(opts.DiffText)
	if diffText == "" {
		diffText = loadGitDiff(opts.Target.Path, opts.BaseRef)
	}
	changedFiles := changedFilesFromDiff(diffText)
	if len(changedFiles) == 0 {
		return Request{}, false
	}
	sourceFiles, testFiles := collectSnapshots(opts.Target.Path, changedFiles)
	if len(sourceFiles) == 0 {
		return Request{}, false
	}
	frameworks := detectFrameworks(sourceFiles)
	prompt := buildPromptTemplate(checks, frameworks)
	return Request{
		Version:      requestVersion,
		Runtime:      "codeguard-semantic-v1",
		TargetName:   opts.Target.Name,
		TargetPath:   filepath.ToSlash(opts.Target.Path),
		Language:     opts.Language,
		BaseRef:      opts.BaseRef,
		Diff:         diffText,
		ChangedFiles: changedFiles,
		Checks:       checks,
		SourceFiles:  sourceFiles,
		TestFiles:    testFiles,
		Frameworks:   frameworks,
		Prompt:       prompt,
	}, true
}

func semanticCheckSpecs(selection CheckSelection) []CheckSpec {
	checks := make([]CheckSpec, 0, 5)
	if selection.FunctionContract {
		checks = append(checks, CheckSpec{
			RuleID:      "quality.ai.semantic-doc-mismatch",
			Title:       "Function and documentation mismatch",
			Description: "Flag changed functions whose names or adjacent docs describe behavior that the implementation does not appear to perform.",
		})
	}
	if selection.ContractDrift {
		checks = append(checks, CheckSpec{
			RuleID:      "quality.ai.contract-drift",
			Title:       "Silent contract drift",
			Description: "Flag changed functions whose observable behavior, nearby documentation, error semantics, or test assumptions appear to drift from the existing contract without an explicit caller-facing update.",
		})
	}
	if selection.MisleadingErrorMessages {
		checks = append(checks, CheckSpec{
			RuleID:      "quality.ai.semantic-error-message",
			Title:       "Misleading error message",
			Description: "Flag changed error strings that would mislead an operator about the failing condition, input, or recovery path.",
		})
	}
	if selection.TestBehaviorCoverage {
		checks = append(checks, CheckSpec{
			RuleID:      "quality.ai.semantic-test-coverage",
			Title:       "Behavior not exercised by tests",
			Description: "Flag changed production behavior when nearby changed or local tests do not appear to exercise the new branch, output, or failure mode.",
		})
	}
	if selection.TestAdequacy {
		checks = append(checks, CheckSpec{
			RuleID:      "quality.ai.semantic-test-adequacy",
			Title:       "Tests appear inadequate for changed behavior",
			Description: "Flag changed production behavior when nearby tests look weak or mismatched, including low-value assertions, over-mocking, happy-path-only coverage, missing negative-path checks, missing boundary checks, or risky changes without a matching test update.",
		})
	}
	return checks
}

func findingsFromResponse(newFinding func(string, string, string, int, string) core.Finding, resp Response) []core.Finding {
	findings := make([]core.Finding, 0, len(resp.Verdicts))
	for _, verdict := range resp.Verdicts {
		if _, ok := supportedRuleIDs[verdict.RuleID]; !ok || strings.TrimSpace(verdict.Message) == "" {
			continue
		}
		level := verdict.Level
		if strings.TrimSpace(level) == "" {
			level = "warn"
		}
		findings = append(findings, newFinding(verdict.RuleID, level, verdict.Path, verdict.Line, verdict.Message))
	}
	return findings
}
