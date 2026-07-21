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
	const secret = "gitleaks-secret-value-must-not-escape"
	data, err := os.ReadFile(filepath.Join("testdata", "gitleaks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
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
	if strings.Contains(fmt.Sprintf("%#v", sections), secret) {
		t.Fatalf("secret payload was retained in finding: %#v", sections)
	}
}

func TestImportGitleaksDropsUnsafePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gitleaks.json")
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
	const secret = "trivy-secret-value-must-not-escape"
	data, err := os.ReadFile(filepath.Join("testdata", "trivy.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	sections, err := Import([]core.ExternalReportConfig{{Path: path, Format: "trivy", Source: "Trivy FS"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 || sections[0].Status != core.StatusFail || len(sections[0].Findings) != 3 {
		t.Fatalf("unexpected sections: %#v", sections)
	}
	var vulnerability, misconfiguration, findingSecret core.Finding
	for _, finding := range sections[0].Findings {
		switch finding.Metadata["trivy_kind"] {
		case "vulnerability":
			vulnerability = finding
		case "misconfiguration":
			misconfiguration = finding
		case "secret":
			findingSecret = finding
		}
	}
	if vulnerability.RuleID != "external.trivy-fs.vulnerability.cve-2026-1234" || vulnerability.Level != "fail" || vulnerability.Path != "go.mod" {
		t.Fatalf("unexpected vulnerability: %#v", vulnerability)
	}
	if misconfiguration.RuleID != "external.trivy-fs.misconfiguration.avd-ksv-0001" || misconfiguration.Level != "warn" || misconfiguration.Path != "deploy/app.yaml" || misconfiguration.Line != 12 {
		t.Fatalf("unexpected misconfiguration: %#v", misconfiguration)
	}
	if findingSecret.RuleID != "external.trivy-fs.secret.generic-api-key" || findingSecret.Level != "fail" || findingSecret.Path != "go.mod" || findingSecret.Line != 19 {
		t.Fatalf("unexpected secret finding: %#v", findingSecret)
	}
	if strings.Contains(fmt.Sprintf("%#v", sections), secret) {
		t.Fatalf("secret payload was retained in finding: %#v", sections)
	}
}
