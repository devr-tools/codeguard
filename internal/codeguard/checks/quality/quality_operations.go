package quality

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func RunOperations(ctx context.Context, env support.Context) core.SectionResult {
	return support.RunTargetSection(ctx, env, "operations", "Operations", operationsTargetFindings)
}

func operationsTargetFindings(_ context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	rules := env.Config.Checks.OperationsRules
	files := listFiles(env, target)
	findings := make([]core.Finding, 0, 2)
	if !hasCriticalPath(files, rules.CriticalPathPatterns) {
		return findings
	}
	if operationsEnabled(rules.DetectMissingOwner) && !hasAnyPattern(files, rules.OwnerFilePatterns) {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     "operations.missing-owner",
			Level:      "warn",
			Message:    "critical production paths have no CODEOWNERS, OWNERS, or service ownership metadata",
			Confidence: "medium",
			Metadata: map[string]string{
				"searched": fmt.Sprintf("%d", len(rules.OwnerFilePatterns)),
				"scope":    "target",
			},
		}))
	}
	if operationsEnabled(rules.DetectMissingRunbook) && !hasAnyPattern(files, rules.RunbookPathPatterns) {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     "operations.missing-runbook",
			Level:      "warn",
			Message:    "critical production paths have no runbook or operations documentation evidence",
			Confidence: "medium",
			Metadata: map[string]string{
				"searched": fmt.Sprintf("%d", len(rules.RunbookPathPatterns)),
				"scope":    "target",
			},
		}))
	}
	return findings
}

func listFiles(env support.Context, target core.TargetConfig) []string {
	if env.ListTargetFiles != nil {
		files, err := env.ListTargetFiles(target)
		if err == nil {
			return files
		}
	}
	files := make([]string, 0)
	if env.VisitTargetFiles != nil {
		env.VisitTargetFiles(target, func(string) bool { return true }, func(rel string, _ []byte) {
			files = append(files, rel)
		})
	}
	return files
}

func hasCriticalPath(files []string, patterns []string) bool {
	for _, file := range files {
		if isGeneratedOrTest(file) {
			continue
		}
		lower := strings.ToLower(filepath.ToSlash(file))
		if isProductionSource(lower) && matchesPattern(lower, patterns) {
			return true
		}
	}
	return false
}

func hasAnyPattern(files []string, patterns []string) bool {
	for _, file := range files {
		lower := strings.ToLower(filepath.ToSlash(file))
		if matchesPattern(lower, patterns) {
			return true
		}
	}
	return false
}

func matchesPattern(path string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(filepath.ToSlash(pattern)))
		if pattern != "" && strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

func isProductionSource(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".py", ".ts", ".tsx", ".js", ".jsx", ".cc", ".cpp", ".cxx", ".hpp", ".hh", ".hxx", ".h":
		return true
	default:
		return false
	}
}

func isGeneratedOrTest(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, "testdata/") || strings.Contains(lower, "__tests__/") || strings.Contains(lower, "fixtures/") || strings.HasSuffix(lower, "_test.go") || strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.")
}

func operationsEnabled(toggle *bool) bool {
	return toggle == nil || *toggle
}
