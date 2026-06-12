package semantic

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var supportedRuleIDs = map[string]struct{}{
	"quality.ai.semantic-doc-mismatch":  {},
	"quality.ai.semantic-error-message": {},
	"quality.ai.semantic-test-coverage": {},
}

func buildRequest(opts Options) (Request, bool) {
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
	return Request{
		Version:      requestVersion,
		Runtime:      "codeguard-semantic-v1",
		TargetName:   opts.Target.Name,
		TargetPath:   filepath.ToSlash(opts.Target.Path),
		Language:     opts.Language,
		BaseRef:      opts.BaseRef,
		Diff:         diffText,
		ChangedFiles: changedFiles,
		Checks:       semanticCheckSpecs(),
		SourceFiles:  sourceFiles,
		TestFiles:    testFiles,
	}, true
}

func semanticCheckSpecs() []CheckSpec {
	return []CheckSpec{
		{
			RuleID:      "quality.ai.semantic-doc-mismatch",
			Title:       "Function and documentation mismatch",
			Description: "Flag changed functions whose names or adjacent docs describe behavior that the implementation does not appear to perform.",
		},
		{
			RuleID:      "quality.ai.semantic-error-message",
			Title:       "Misleading error message",
			Description: "Flag changed error strings that would mislead an operator about the failing condition, input, or recovery path.",
		},
		{
			RuleID:      "quality.ai.semantic-test-coverage",
			Title:       "Behavior not exercised by tests",
			Description: "Flag changed production behavior when nearby changed or local tests do not appear to exercise the new branch, output, or failure mode.",
		},
	}
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
