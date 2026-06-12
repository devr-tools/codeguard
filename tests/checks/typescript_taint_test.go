package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

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

func TestSecurityTypeScriptTaintFlowsAcrossModules(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeTaintDBFile(t, dir)
	writeFile(t, filepath.Join(dir, "src", "routes.ts"),
		"import { runQuery } from \"./db\";\n"+
			"export function handler(req: any): void {\n"+
			"  runQuery(req.query.id);\n"+
			"}\n")

	report := runTypeScriptTaintScan(t, typeScriptTaintConfig(dir))

	assertSectionStatus(t, report, "Security", "warn")
	assertFindingRulePresent(t, report, "Security", "security.typescript.taint-flow")
	assertTaintFindingMessageContains(t, report,
		"request query (src/routes.ts:3)",
		"runQuery arg (src/routes.ts:3)",
		"pool.query sink (src/db.ts:4)",
	)
}

func TestSecurityTypeScriptTaintFlowsThroughReExportChain(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeTaintDBFile(t, dir)
	writeFile(t, filepath.Join(dir, "src", "index.ts"), "export { runQuery } from \"./db\";\n")
	writeFile(t, filepath.Join(dir, "src", "routes.ts"),
		"import { runQuery } from \"./index\";\n"+
			"export function handler(req: any): void {\n"+
			"  runQuery(req.body.id);\n"+
			"}\n")

	report := runTypeScriptTaintScan(t, typeScriptTaintConfig(dir))

	assertFindingRulePresent(t, report, "Security", "security.typescript.taint-flow")
	assertTaintFindingMessageContains(t, report, "request body", "pool.query sink (src/db.ts:4)")
}

func TestSecurityTypeScriptTaintSanitizedFlowsAreNotReported(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeTaintDBFile(t, dir)
	writeFile(t, filepath.Join(dir, "src", "esc.ts"),
		"export function escapeSql(value: string): string {\n"+
			"  return value.replace(/'/g, \"''\");\n"+
			"}\n")
	writeFile(t, filepath.Join(dir, "src", "safe.ts"),
		"import { Pool } from \"pg\";\n"+
			"import { runQuery } from \"./db\";\n"+
			"import { escapeSql } from \"./esc\";\n"+
			"const pool = new Pool();\n"+
			"export function escapedHandler(req: any): void {\n"+
			"  runQuery(escapeSql(req.query.id));\n"+
			"}\n"+
			"export function parameterizedHandler(req: any): void {\n"+
			"  pool.query(\"SELECT * FROM users WHERE id = $1\", [req.query.id]);\n"+
			"}\n"+
			"export function encodedHandler(req: any): void {\n"+
			"  fetch(\"https://api.example.com/u/\" + encodeURIComponent(req.query.id));\n"+
			"}\n")

	report := runTypeScriptTaintScan(t, typeScriptTaintConfig(dir))

	assertNoTaintFindings(t, report)
}

func TestSecurityTypeScriptTaintFlowsAcrossModuleCycle(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "cyclea.ts"),
		"import { bounce } from \"./cycleb\";\n"+
			"export function relay(value: string, depth: number): string {\n"+
			"  return bounce(value, depth - 1);\n"+
			"}\n"+
			"export function handler(req: any): void {\n"+
			"  relay(req.query.id, 2);\n"+
			"}\n")
	writeFile(t, filepath.Join(dir, "src", "cycleb.ts"),
		"import { relay } from \"./cyclea\";\n"+
			"import { Pool } from \"pg\";\n"+
			"const pool = new Pool();\n"+
			"export function bounce(value: string, depth: number): string {\n"+
			"  if (depth > 0) {\n"+
			"    return relay(value, depth);\n"+
			"  }\n"+
			"  return String(pool.query(\"SELECT \" + value));\n"+
			"}\n")

	report := runTypeScriptTaintScan(t, typeScriptTaintConfig(dir))

	assertFindingRulePresent(t, report, "Security", "security.typescript.taint-flow")
	assertTaintFindingMessageContains(t, report,
		"request query (src/cyclea.ts:6)",
		"pool.query sink (src/cycleb.ts:8)",
	)
}

func TestSecurityTypeScriptTaintDepthCapTruncatesLongChains(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "a.ts"),
		"import { hopOne } from \"./b\";\n"+
			"export function handler(req: any): void {\n"+
			"  hopOne(req.query.id);\n"+
			"}\n")
	writeFile(t, filepath.Join(dir, "src", "b.ts"),
		"import { hopTwo } from \"./c\";\n"+
			"export function hopOne(value: string): void {\n"+
			"  hopTwo(value);\n"+
			"}\n")
	writeFile(t, filepath.Join(dir, "src", "c.ts"),
		"import { Pool } from \"pg\";\n"+
			"const pool = new Pool();\n"+
			"export function hopTwo(value: string): void {\n"+
			"  pool.query(\"SELECT \" + value);\n"+
			"}\n")

	cfg := typeScriptTaintConfig(dir)

	report := runTypeScriptTaintScan(t, cfg)
	assertFindingRulePresent(t, report, "Security", "security.typescript.taint-flow")

	cfg.Checks.SecurityRules.TypeScriptTaintMaxDepth = 1
	assertNoTaintFindings(t, runTypeScriptTaintScan(t, cfg))
}

func TestSecurityTypeScriptTaintExistingSingleFileFindingsRemainStable(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "index.ts"),
		"const exec = require(\"node:child_process\").exec;\n"+
			"exec(\"echo hi\");\n")

	report := runTypeScriptTaintScan(t, typeScriptTaintConfig(dir))

	assertFindingRulePresent(t, report, "Security", "security.typescript.shell-execution")
	assertNoTaintFindings(t, report)
}
