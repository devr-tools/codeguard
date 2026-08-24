package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestQualityEnvironmentBranching(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "billing", "gateway.go"), "package billing\n\nimport \"os\"\n\nfunc Gateway() string {\n\tif os.Getenv(\"ENV\") == \"production\" {\n\t\treturn \"stripe\"\n\t}\n\treturn \"sandbox\"\n}\n")

	report, err := codeguard.Run(context.Background(), qualityEnvironmentTestConfig(dir, "environment-branching"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.environment-branching")
}

func TestQualityEnvironmentBranchingAllowsBootstrapConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config", "gateway.go"), "package config\n\nimport \"os\"\n\nfunc Gateway() string {\n\tif os.Getenv(\"ENV\") == \"production\" {\n\t\treturn \"stripe\"\n\t}\n\treturn \"sandbox\"\n}\n")

	report, err := codeguard.Run(context.Background(), qualityEnvironmentTestConfig(dir, "environment-branching-config"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if messages := qualityRuleMessages(report, "quality.environment-branching"); len(messages) != 0 {
		t.Fatalf("unexpected environment branching findings: %v", messages)
	}
}

func TestQualityEnvironmentBranchingSkipsStringParsingAndRegexValidation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "lib", "conversation-ref.ts"), strings.Join([]string{
		"const CONVERSATION_ID_RE = /^production-[a-z0-9]+$/;",
		"export function parseConversationRef(value?: string | null) {",
		"  const trimmed = (value ?? '').trim();",
		"  if (!trimmed) return null;",
		"  if (CONVERSATION_ID_RE.test(trimmed)) return trimmed;",
		"  return findConversationRefInText(trimmed);",
		"}",
		"declare function findConversationRefInText(value: string): string | null;",
	}, "\n"))

	cfg := qualityEnvironmentTestConfig(dir, "environment-branching-string-parsing")
	cfg.Targets[0].Language = "typescript"
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if messages := qualityRuleMessages(report, "quality.environment-branching"); len(messages) != 0 {
		t.Fatalf("unexpected environment branching findings: %v", messages)
	}
}

func qualityEnvironmentTestConfig(dir string, name string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	off := false
	cfg.Checks.Quality = true
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.SupplyChain = false
	cfg.Checks.Context = &off
	cfg.Cache.Enabled = &off
	return cfg
}

func qualityRuleMessages(report codeguard.Report, ruleID string) []string {
	messages := make([]string, 0)
	for _, section := range report.Sections {
		if section.Name != "Code Quality" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID == ruleID {
				messages = append(messages, finding.Message)
			}
		}
	}
	return messages
}
