package ci

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func Run(_ context.Context, env support.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	for _, target := range env.Config.Targets {
		findings = append(findings, findingsForTarget(env, target)...)
	}
	return env.FinalizeSection("ci", "CI/CD", findings)
}

func findingsForTarget(env support.Context, target core.TargetConfig) []core.Finding {
	findings := make([]core.Finding, 0)
	findings = append(findings, requiredWorkflowDirFindings(env, target)...)
	findings = append(findings, requiredPathFindings(env, target, env.Config.Checks.CIRules.RequiredWorkflowFiles, "required workflow file is missing")...)
	findings = append(findings, requiredPathFindings(env, target, env.Config.Checks.CIRules.RequiredReleaseFiles, "required release file is missing")...)
	findings = append(findings, requiredPathFindings(env, target, env.Config.Checks.CIRules.RequiredAutomationPaths, "required automation path is missing")...)
	findings = append(findings, workflowContentFindings(env, target)...)
	findings = append(findings, testFileLocationFindings(env, target)...)
	findings = append(findings, testQualityFindings(env, target)...)
	return findings
}

func requiredWorkflowDirFindings(env support.Context, target core.TargetConfig) []core.Finding {
	if !*env.Config.Checks.CIRules.RequireWorkflowDir {
		return nil
	}
	workflowDir := filepath.Join(target.Path, ".github", "workflows")
	if _, err := os.Stat(workflowDir); err == nil {
		return nil
	}
	return []core.Finding{env.NewFinding(support.FindingInput{
		RuleID:  "ci.required-workflow-dir",
		Level:   "fail",
		Path:    ".github/workflows",
		Message: "workflow directory is required",
	})}
}

func requiredPathFindings(env support.Context, target core.TargetConfig, paths []string, message string) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, required := range paths {
		if _, err := os.Stat(filepath.Join(target.Path, filepath.FromSlash(required))); err != nil {
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "ci.required-file",
				Level:   "fail",
				Path:    required,
				Message: message,
			}))
		}
	}
	return findings
}

func workflowContentFindings(env support.Context, target core.TargetConfig) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, rule := range env.Config.Checks.CIRules.WorkflowContentRules {
		data, err := os.ReadFile(filepath.Join(target.Path, filepath.FromSlash(rule.Path)))
		if err != nil {
			continue
		}
		text := string(data)
		for _, marker := range rule.RequiredContains {
			if !strings.Contains(text, marker) {
				findings = append(findings, env.NewFinding(support.FindingInput{
					RuleID:  "ci.workflow-content",
					Level:   "fail",
					Path:    rule.Path,
					Message: fmt.Sprintf("required workflow marker %q is missing", marker),
				}))
			}
		}
	}
	return findings
}

func testFileLocationFindings(env support.Context, target core.TargetConfig) []core.Finding {
	allowed := env.Config.Checks.CIRules.AllowedTestPaths
	if len(allowed) == 0 {
		return nil
	}
	return env.ScanTargetFiles(target, "ci", func(rel string) bool {
		return isTargetTestFile(target.Language, rel)
	}, func(file string, _ []byte) []core.Finding {
		for _, pattern := range allowed {
			matched, err := filepath.Match(filepath.FromSlash(pattern), filepath.FromSlash(file))
			if err == nil && matched {
				return nil
			}
			if strings.Contains(pattern, "**") {
				prefix := strings.TrimSuffix(filepath.ToSlash(pattern), "**")
				if strings.HasPrefix(filepath.ToSlash(file), prefix) {
					return nil
				}
			}
		}
		return []core.Finding{env.NewFinding(support.FindingInput{
			RuleID:  "ci.test-file-location",
			Level:   "fail",
			Path:    file,
			Line:    1,
			Column:  1,
			Message: "test files must live under configured test paths",
		})}
	})
}
