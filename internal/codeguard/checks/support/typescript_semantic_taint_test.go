package support

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestAnalyzeTypeScriptTarget_UntrustedInputFlowFindings(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is required for TypeScript semantic tests")
	}

	libPath := discoverTypeScriptLibPath(".")
	if libPath == "" {
		t.Skip("typescript library not available")
	}
	t.Setenv(codeguardTypeScriptLibEnv, libPath)

	dir := t.TempDir()
	writeTestFile(t, dir, "tsconfig.json", `{"compilerOptions":{"allowJs":true,"checkJs":true,"noEmit":true}}`)
	writeTestFile(t, dir, "flow.ts", `import { exec } from "child_process";

export function run(req, element) {
  const cmd = req.query.cmd;
  exec(cmd);

  const { html } = req.body;
  element.innerHTML = html;
}
`)
	writeTestFile(t, dir, "safe.ts", `import { exec } from "child_process";

export function runSafe() {
  const cmd = "echo ok";
  exec(cmd);
}
`)

	results, ok, err := AnalyzeTypeScriptTarget(context.Background(), core.TargetConfig{
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

	if !hasFinding(results.Security, "security.typescript.untrusted-input-flow", filepath.ToSlash(filepath.Join("flow.ts")), 5) {
		t.Fatalf("expected shell execution taint finding in flow.ts line 5, got %#v", results.Security)
	}
	if !hasFinding(results.Security, "security.typescript.untrusted-input-flow", filepath.ToSlash(filepath.Join("flow.ts")), 8) {
		t.Fatalf("expected unsafe HTML taint finding in flow.ts line 8, got %#v", results.Security)
	}
	if hasFinding(results.Security, "security.typescript.untrusted-input-flow", filepath.ToSlash(filepath.Join("safe.ts")), 5) {
		t.Fatalf("did not expect taint finding in safe.ts, got %#v", results.Security)
	}
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

func hasFinding(findings []FindingInput, ruleID string, path string, line int) bool {
	for _, finding := range findings {
		if finding.RuleID == ruleID && finding.Path == path && finding.Line == line {
			return true
		}
	}
	return false
}

func writeTestFile(t *testing.T, dir string, name string, contents string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
