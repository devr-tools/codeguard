package checks_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	supportpkg "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestAnalyzeTypeScriptTarget_UntrustedInputFlowFindings(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is required for TypeScript semantic tests")
	}

	libPath := discoverTypeScriptLibPathForTest(".")
	if libPath == "" {
		t.Skip("typescript library not available")
	}
	t.Setenv("CODEGUARD_TYPESCRIPT_LIB_PATH", libPath)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tsconfig.json"), `{"compilerOptions":{"allowJs":true,"checkJs":true,"noEmit":true}}`)
	writeFile(t, filepath.Join(dir, "flow.ts"), `import { exec } from "child_process";

export function run(req, element) {
  const cmd = req.query.cmd;
  exec(cmd);

  const { html } = req.body;
  element.innerHTML = html;
}
`)
	writeFile(t, filepath.Join(dir, "safe.ts"), `import { exec } from "child_process";

export function runSafe() {
  const cmd = "echo ok";
  exec(cmd);
}
`)

	results, ok, err := supportpkg.AnalyzeTypeScriptTarget(context.Background(), core.TargetConfig{
		Name:     "fixture",
		Path:     dir,
		Language: "typescript",
	}, testTypeScriptSemanticConfig())
	if err != nil {
		t.Fatalf("AnalyzeTypeScriptTarget returned error: %v", err)
	}
	if !ok {
		t.Fatal("AnalyzeTypeScriptTarget did not run semantic analysis")
	}

	if !hasSemanticFinding(results.Security, "security.typescript.untrusted-input-flow", "flow.ts", 5) {
		t.Fatalf("expected shell execution taint finding in flow.ts line 5, got %#v", results.Security)
	}
	if !hasSemanticFinding(results.Security, "security.typescript.untrusted-input-flow", "flow.ts", 8) {
		t.Fatalf("expected unsafe HTML taint finding in flow.ts line 8, got %#v", results.Security)
	}
	if hasSemanticFinding(results.Security, "security.typescript.untrusted-input-flow", "safe.ts", 5) {
		t.Fatalf("did not expect taint finding in safe.ts, got %#v", results.Security)
	}
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

func TestSecurityTypeScriptTaintExpressAndNextModelsAreImportGated(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "express.ts"),
		"import type { Request as ExpressRequest } from \"express\";\n"+
			"import { exec } from \"child_process\";\n"+
			"export function handler(req: ExpressRequest): void {\n"+
			"  exec(req.query.cmd);\n"+
			"}\n")
	writeFile(t, filepath.Join(dir, "src", "next.ts"),
		"import { NextRequest } from \"next/server\";\n"+
			"import { exec } from \"child_process\";\n"+
			"export async function POST(request: NextRequest): Promise<void> {\n"+
			"  exec((await request.json()).cmd);\n"+
			"}\n")
	writeFile(t, filepath.Join(dir, "src", "lookalike.ts"),
		"import { exec } from \"child_process\";\n"+
			"type Request = { query: { cmd: string } };\n"+
			"export function handler(request: Request): void {\n"+
			"  exec(request.query.cmd);\n"+
			"}\n")
	writeFile(t, filepath.Join(dir, "src", "express_safe.ts"),
		"import type { Request } from \"express\";\n"+
			"import { exec } from \"child_process\";\n"+
			"declare function shellQuote(value: string): string;\n"+
			"export function handler(req: Request): void {\n"+
			"  exec(shellQuote(req.query.cmd));\n"+
			"}\n")

	report := runTypeScriptTaintScan(t, typeScriptTaintConfig(dir))
	models := map[string]string{}
	for _, section := range report.Sections {
		if section.Name != "Security" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID == "security.typescript.taint-flow" {
				models[finding.Path] = finding.Metadata["framework_model"]
			}
		}
	}
	if models["src/express.ts"] != "express" {
		t.Fatalf("expected import-gated Express metadata, got %#v", models)
	}
	if models["src/next.ts"] != "nextjs" {
		t.Fatalf("expected import-gated Next.js metadata, got %#v", models)
	}
	if models["src/lookalike.ts"] != "" {
		t.Fatalf("lookalike request without framework import must use generic model, got %#v", models)
	}
	if _, found := models["src/express_safe.ts"]; found {
		t.Fatalf("sanitized Express input must not produce a taint finding, got %#v", models)
	}
}
