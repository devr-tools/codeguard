package ci

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/codeguard/core"
)

func inspectRepository(root string, rules ciRules) ([]core.Finding, core.Severity) {
	var findings []core.Finding

	if rules.requireWorkflowDir {
		workflowDir := filepath.Join(root, ".github", "workflows")
		info, err := os.Stat(workflowDir)
		if err != nil || !info.IsDir() {
			findings = append(findings, core.Finding{
				Path:     filepath.ToSlash(workflowDir),
				Message:  "required workflow directory is missing",
				Severity: core.SeverityError,
			})
		}
	}

	for _, rel := range rules.requiredWorkflowFiles {
		findings = append(findings, requiredPathFinding(root, rel, "required workflow file is missing")...)
	}
	for _, rule := range rules.workflowContentRules {
		findings = append(findings, workflowContentFindings(root, rule)...)
	}
	for _, rel := range rules.requiredReleaseFiles {
		findings = append(findings, requiredPathFinding(root, rel, "required release file is missing")...)
	}
	for _, rel := range rules.requiredAutomation {
		findings = append(findings, requiredPathFinding(root, rel, "required automation path is missing")...)
	}

	if len(findings) == 0 {
		return nil, core.SeverityInfo
	}
	return findings, core.SeverityError
}

func requiredPathFinding(root string, relativePath string, message string) []core.Finding {
	path := filepath.Join(root, relativePath)
	if _, err := os.Stat(path); err != nil {
		return []core.Finding{{
			Path:     filepath.ToSlash(relativePath),
			Message:  message,
			Severity: core.SeverityError,
		}}
	}
	return nil
}

func workflowContentFindings(root string, rule core.WorkflowRuleConfig) []core.Finding {
	path := filepath.Join(root, rule.Path)
	content, err := os.ReadFile(path)
	if err != nil {
		return []core.Finding{{
			Path:     filepath.ToSlash(rule.Path),
			Message:  "workflow content rule could not read file",
			Severity: core.SeverityError,
		}}
	}

	text := string(content)
	var findings []core.Finding
	for _, needle := range rule.RequiredContains {
		if !strings.Contains(text, needle) {
			findings = append(findings, core.Finding{
				Path:     filepath.ToSlash(rule.Path),
				Message:  "workflow file is missing required content: " + needle,
				Severity: core.SeverityError,
			})
		}
	}
	return findings
}
