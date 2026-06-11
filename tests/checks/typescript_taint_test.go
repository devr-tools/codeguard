package checks_test

import (
	"context"
	"os"
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
