package checks_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	supportpkg "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func discoverTypeScriptLibPathForTest(targetPath string) string {
	candidates := []string{
		filepath.Join(targetPath, "node_modules", "typescript", "lib", "typescript.js"),
		"/Applications/Visual Studio Code.app/Contents/Resources/app/extensions/node_modules/typescript/lib/typescript.js",
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func testTypeScriptSemanticConfig() core.Config {
	return core.Config{
		Checks: core.CheckConfig{
			DesignRules: core.DesignRulesConfig{
				MaxMethodsPerType:   100,
				MaxInterfaceMethods: 100,
			},
			QualityRules: core.QualityRulesConfig{
				MaxFunctionLines:        1000,
				MaxParameters:           100,
				MaxCyclomaticComplexity: 100,
			},
		},
	}
}

func hasSemanticFinding(findings []supportpkg.FindingInput, ruleID string, path string, line int) bool {
	for _, finding := range findings {
		if finding.RuleID == ruleID && finding.Path == filepath.ToSlash(path) && finding.Line == line {
			return true
		}
	}
	return false
}

func typeScriptTaintConfig(dir string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = "security-typescript-taint"
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: "typescript"}}
	cfg.Checks.Security = true
	cfg.Checks.Design = false
	cfg.Checks.Quality = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	return cfg
}

func runTypeScriptTaintScan(t *testing.T, cfg codeguard.Config) codeguard.Report {
	t.Helper()
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}

func assertTaintFindingMessageContains(t *testing.T, report codeguard.Report, needles ...string) {
	t.Helper()
	messages := taintFindingMessages(report)
	for _, message := range messages {
		if containsAll(message, needles) {
			return
		}
	}
	t.Fatalf("no taint finding message contains %q, got: %q", needles, messages)
}

func assertNoTaintFindings(t *testing.T, report codeguard.Report) {
	t.Helper()
	if messages := taintFindingMessages(report); len(messages) > 0 {
		t.Fatalf("expected no taint findings, got: %q", messages)
	}
}

func taintFindingMessages(report codeguard.Report) []string {
	var messages []string
	for _, section := range report.Sections {
		if section.Name != "Security" {
			continue
		}
		for _, finding := range section.Findings {
			if strings.HasSuffix(finding.RuleID, ".taint-flow") {
				messages = append(messages, finding.Message)
			}
		}
	}
	return messages
}

func containsAll(text string, needles []string) bool {
	for _, needle := range needles {
		if !strings.Contains(text, needle) {
			return false
		}
	}
	return true
}

func writeTaintDBFile(t *testing.T, dir string) {
	writeFile(t, filepath.Join(dir, "src", "db.ts"),
		"import { Pool } from \"pg\";\n"+
			"const pool = new Pool();\n"+
			"export function runQuery(id: string): unknown {\n"+
			"  return pool.query(\"SELECT * FROM users WHERE id = \" + id);\n"+
			"}\n")
}
