package externalreports

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestImportGitleaksDoesNotRetainSecretPayload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gitleaks.json")
	redactedMarker := "gitleaks-" + "credential-payload-must-not-escape"
	data, err := os.ReadFile(filepath.Join("testdata", "gitleaks.json"))
	if err != nil {
		t.Fatal(err)
	}
	// #nosec G703 -- path is created beneath this test's temporary directory.
	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	sections, err := Import([]core.ExternalReportConfig{{Path: path, Format: "gitleaks"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 || sections[0].Status != core.StatusFail || len(sections[0].Findings) != 1 {
		t.Fatalf("unexpected sections: %#v", sections)
	}
	finding := sections[0].Findings[0]
	if finding.RuleID != "external.gitleaks.aws-access-key" || finding.Path != "internal/config.go" || finding.Line != 8 || finding.Column != 4 {
		t.Fatalf("unexpected finding: %#v", finding)
	}
	if finding.Metadata["external_format"] != "gitleaks" {
		t.Fatalf("missing provenance: %#v", finding.Metadata)
	}
	if strings.Contains(fmt.Sprintf("%#v", sections), redactedMarker) {
		t.Fatalf("secret payload was retained in finding: %#v", sections)
	}
}

func TestImportGitleaksDropsUnsafePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gitleaks.json")
	// #nosec G703 -- path is created beneath this test's temporary directory.
	if err := os.WriteFile(path, []byte(`[{"RuleID":"key","File":"../credential","StartLine":1,"Secret":"never-retain"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	sections, err := Import([]core.ExternalReportConfig{{Path: path, Format: "gitleaks"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := sections[0].Findings[0].Path; got != "" {
		t.Fatalf("unsafe path was retained: %q", got)
	}
}

func TestImportTrivyNormalizesFindingsAndRedactsSecretPayload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trivy.json")
	redactedMarker := "trivy-" + "credential-payload-must-not-escape"
	data, err := os.ReadFile(filepath.Join("testdata", "trivy.json"))
	if err != nil {
		t.Fatal(err)
	}
	// #nosec G703 -- path is created beneath this test's temporary directory.
	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	sections, err := Import([]core.ExternalReportConfig{{Path: path, Format: "trivy", Source: "Trivy FS"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 || sections[0].Status != core.StatusFail || len(sections[0].Findings) != 3 {
		t.Fatalf("unexpected sections: %#v", sections)
	}
	byKind := trivyFindingsByKind(sections[0].Findings)
	assertTrivyFinding(t, byKind["vulnerability"], trivyFindingShape{ruleID: "external.trivy-fs.vulnerability.cve-2026-1234", level: "fail", path: "go.mod"})
	assertTrivyFinding(t, byKind["misconfiguration"], trivyFindingShape{ruleID: "external.trivy-fs.misconfiguration.avd-ksv-0001", level: "warn", path: "deploy/app.yaml", line: 12})
	assertTrivyFinding(t, byKind["secret"], trivyFindingShape{ruleID: "external.trivy-fs.secret.generic-api-key", level: "fail", path: "go.mod", line: 19})
	if strings.Contains(fmt.Sprintf("%#v", sections), redactedMarker) {
		t.Fatalf("secret payload was retained in finding: %#v", sections)
	}
}

// trivyFindingShape is the expected identity of an imported Trivy finding;
// a zero line is not checked.
type trivyFindingShape struct {
	ruleID string
	level  string
	path   string
	line   int
}

func trivyFindingsByKind(findings []core.Finding) map[string]core.Finding {
	byKind := make(map[string]core.Finding, len(findings))
	for _, finding := range findings {
		byKind[finding.Metadata["trivy_kind"]] = finding
	}
	return byKind
}

func assertTrivyFinding(t *testing.T, finding core.Finding, want trivyFindingShape) {
	t.Helper()
	got := trivyFindingShape{ruleID: finding.RuleID, level: finding.Level, path: finding.Path, line: finding.Line}
	if want.line == 0 {
		got.line = 0
	}
	if got != want {
		t.Fatalf("unexpected finding: %#v", finding)
	}
}
