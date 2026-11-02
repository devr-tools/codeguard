package codeguard_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func enterpriseCIFindings(t *testing.T, root string) map[string][]codeguard.Finding {
	t.Helper()
	cfg, err := codeguard.DefaultScanConfig(root, "enterprise")
	if err != nil {
		t.Fatalf("DefaultScanConfig: %v", err)
	}
	cfg.Checks.CI = true
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := map[string][]codeguard.Finding{}
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			out[finding.RuleID] = append(out[finding.RuleID], finding)
		}
	}
	return out
}

func writeEnterpriseFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestEnterpriseProfileCIPolicyIsGeneric(t *testing.T) {
	cfg, err := codeguard.DefaultScanConfig(t.TempDir(), "enterprise")
	if err != nil {
		t.Fatalf("DefaultScanConfig: %v", err)
	}
	ci := cfg.Checks.CIRules
	if ci.RequireWorkflowDir == nil || !*ci.RequireWorkflowDir {
		t.Fatal("enterprise must require a workflow directory")
	}
	if len(ci.RequiredReleaseFiles) != 0 || len(ci.RequiredAutomationPaths) != 0 {
		t.Fatalf("enterprise must not require specific files: release=%v automation=%v", ci.RequiredReleaseFiles, ci.RequiredAutomationPaths)
	}
	if len(ci.RequiredGates) != 2 || ci.RequiredGates[0] != "test" || ci.RequiredGates[1] != "security" {
		t.Fatalf("required gates = %v, want [test security]", ci.RequiredGates)
	}
}

func TestEnterpriseProfileAcceptsAnyWorkflowWithTestAndSecurityGates(t *testing.T) {
	root := t.TempDir()
	writeEnterpriseFile(t, filepath.Join(root, "app.py"), "def main():\n    return 1\n")
	writeEnterpriseFile(t, filepath.Join(root, ".github", "workflows", "pipeline.yml"),
		"name: pipeline\non: [push]\njobs:\n  verify:\n    runs-on: ubuntu-latest\n    steps:\n      - run: pytest\n      - run: semgrep --config auto\n")
	findings := enterpriseCIFindings(t, root)
	for _, rule := range []string{"ci.required-file", "ci.required-workflow-dir", "ci.missing-required-gate"} {
		if len(findings[rule]) != 0 {
			t.Errorf("%s fired on a repository with a generic compliant workflow: %+v", rule, findings[rule])
		}
	}
}

func TestEnterpriseProfileStillFailsWithoutWorkflowsOrGates(t *testing.T) {
	root := t.TempDir()
	writeEnterpriseFile(t, filepath.Join(root, "app.py"), "def main():\n    return 1\n")
	if findings := enterpriseCIFindings(t, root); len(findings["ci.required-workflow-dir"]) == 0 {
		t.Fatalf("enterprise must fail a repository without .github/workflows, got %v", findings)
	}

	gateless := t.TempDir()
	writeEnterpriseFile(t, filepath.Join(gateless, "app.py"), "def main():\n    return 1\n")
	writeEnterpriseFile(t, filepath.Join(gateless, ".github", "workflows", "lint.yml"),
		"name: lint\non: [push]\njobs:\n  lint:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo lint\n")
	findings := enterpriseCIFindings(t, gateless)
	gates := map[string]bool{}
	for _, finding := range findings["ci.missing-required-gate"] {
		gates[finding.Metadata["gate"]] = true
	}
	if !gates["test"] || !gates["security"] {
		t.Fatalf("missing gates = %v, want test and security", gates)
	}
}
