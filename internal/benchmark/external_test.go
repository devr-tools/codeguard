package benchmark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportExternalReportsNormalizesJSONFormats(t *testing.T) {
	dir := t.TempDir()
	semgrepPath := writeFixture(t, dir, "semgrep.json", `{
  "results": [
    {
      "check_id": "python.lang.security.audit.dangerous-subprocess-use",
      "path": "src/app.py",
      "start": {"line": 12, "col": 3},
      "extra": {
        "message": "subprocess call with shell=True",
        "severity": "WARNING",
        "metadata": {"category": "security"}
      }
    }
  ]
}`)
	gitleaksPath := writeFixture(t, dir, "gitleaks.json", `[
  {
    "RuleID": "generic-api-key",
    "Description": "Generic API Key",
    "File": "src/config.go",
    "StartLine": 5,
    "StartColumn": 10,
    "Tags": ["key", "secret"]
  }
]`)
	trivyPath := writeFixture(t, dir, "trivy.json", `{
  "Results": [
    {
      "Target": "Dockerfile",
      "Class": "config",
      "Type": "dockerfile",
      "Secrets": [
        {"RuleID": "aws-access-key-id", "Category": "AWS", "Severity": "HIGH", "Title": "AWS Access Key ID", "StartLine": 9}
      ],
      "Misconfigurations": [
        {"ID": "DS002", "Type": "Dockerfile Security Check", "Severity": "MEDIUM", "Title": "Container running as root"}
      ],
      "Vulnerabilities": [
        {"VulnerabilityID": "CVE-2024-1234", "PkgName": "openssl", "Severity": "CRITICAL", "Title": "openssl vuln"}
      ]
    }
  ]
}`)
	truffleHogPath := writeFixture(t, dir, "trufflehog.json", `{"DetectorName":"GitHub","Redacted":"github token","SourceMetadata":{"Data":{"Filesystem":{"file":"src/token.ts","line":7}}}}
`)

	report, err := ImportExternalReports([]ExternalReportInput{
		{Tool: "semgrep", Path: semgrepPath},
		{Tool: "gitleaks", Path: gitleaksPath},
		{Tool: "trivy", Path: trivyPath},
		{Tool: "trufflehog", Path: truffleHogPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Version != ExternalSchemaVersion || report.Summary.Total != 6 || len(report.Findings) != 6 {
		t.Fatalf("unexpected report summary: %#v", report.Summary)
	}
	for tool, want := range map[string]int{"gitleaks": 1, "semgrep": 1, "trivy": 3, "trufflehog": 1} {
		if got := bucketCount(report.Summary.ByTool, tool, "", "", 0); got != want {
			t.Fatalf("%s count = %d, want %d; buckets=%#v", tool, got, want, report.Summary.ByTool)
		}
	}
	for _, want := range []ExternalBucket{
		{Tool: "semgrep", Category: "shell-exec", Count: 1},
		{Tool: "gitleaks", Category: "secrets", Count: 1},
		{Tool: "trivy", Category: "secrets", Count: 1},
		{Tool: "trivy", Category: "container-user", Count: 1},
		{Tool: "trivy", Category: "supply-chain", Count: 1},
		{Tool: "trufflehog", Category: "secrets", Count: 1},
	} {
		if got := bucketCount(report.Summary.ByCategory, want.Tool, want.Category, "", 0); got != want.Count {
			t.Fatalf("%s/%s count = %d, want %d; buckets=%#v", want.Tool, want.Category, got, want.Count, report.Summary.ByCategory)
		}
	}
	if got := bucketCount(report.Summary.ByPathLine, "trivy", "secrets", "Dockerfile", 9); got != 1 {
		t.Fatalf("trivy Dockerfile:9 count = %d", got)
	}
	markdown := RenderExternalMarkdown(report)
	if !strings.Contains(markdown, "| trivy | secrets | 1 |") || !strings.Contains(markdown, "`src/token.ts` | 7 |") {
		t.Fatalf("Markdown report missing normalized rows:\n%s", markdown)
	}
}

func TestImportExternalReportsNormalizesSARIF(t *testing.T) {
	dir := t.TempDir()
	sarifPath := writeFixture(t, dir, "semgrep.sarif", `{
  "version": "2.1.0",
  "$schema": "https://json.schemastore.org/sarif-2.1.0.json",
  "runs": [
    {
      "tool": {
        "driver": {
          "name": "Semgrep",
          "rules": [
            {
              "id": "js.lang.security.audit.xss.innerhtml",
              "shortDescription": {"text": "DOM XSS"},
              "properties": {"tags": ["security", "xss"]}
            }
          ]
        }
      },
      "results": [
        {
          "ruleId": "js.lang.security.audit.xss.innerhtml",
          "level": "warning",
          "message": {"text": "innerHTML sink"},
          "locations": [
            {
              "physicalLocation": {
                "artifactLocation": {"uri": "web/app.ts"},
                "region": {"startLine": 21, "startColumn": 6}
              }
            }
          ]
        }
      ]
    }
  ]
}`)

	report, err := ImportExternalReports([]ExternalReportInput{{Tool: "semgrep", Path: sarifPath}})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(report.Findings))
	}
	finding := report.Findings[0]
	if finding.Source != "sarif" || finding.Category != "unsafe-html" || finding.Path != "web/app.ts" || finding.Line != 21 {
		t.Fatalf("unexpected SARIF finding: %#v", finding)
	}
	if report.Reports[0].Format != "sarif" {
		t.Fatalf("format = %q, want sarif", report.Reports[0].Format)
	}
}

func TestImportExternalReportsUsesSARIFRuleIndexWhenRuleIDIsMissing(t *testing.T) {
	dir := t.TempDir()
	sarifPath := writeFixture(t, dir, "codeql.sarif", `{
  "version": "2.1.0",
  "$schema": "https://json.schemastore.org/sarif-2.1.0.json",
  "runs": [
    {
      "tool": {
        "driver": {
          "name": "Semgrep",
          "rules": [
            {
              "id": "python.lang.security.audit.md5-used-as-security",
              "shortDescription": {"text": "MD5 used for security"},
              "properties": {"problem.severity": "warning"}
            }
          ]
        }
      },
      "results": [
        {
          "ruleIndex": 0,
          "level": "warning",
          "message": {"text": "Use of MD5"},
          "locations": [
            {
              "physicalLocation": {
                "artifactLocation": {"uri": "file://src/hash.py"},
                "region": {"startLine": 8, "startColumn": 12}
              }
            }
          ]
        }
      ]
    }
  ]
}`)

	report, err := ImportExternalReports([]ExternalReportInput{{Tool: "semgrep", Path: sarifPath}})
	if err != nil {
		t.Fatal(err)
	}
	finding := report.Findings[0]
	if finding.RuleID != "python.lang.security.audit.md5-used-as-security" ||
		finding.Category != "crypto-weakness" ||
		finding.Path != "src/hash.py" ||
		finding.Line != 8 ||
		finding.Column != 12 {
		t.Fatalf("unexpected SARIF ruleIndex finding: %#v", finding)
	}
}

func TestImportExternalReportsDoesNotInferRuleIndexWhenMissing(t *testing.T) {
	dir := t.TempDir()
	sarifPath := writeFixture(t, dir, "missing-rule.sarif", `{
  "version": "2.1.0",
  "$schema": "https://json.schemastore.org/sarif-2.1.0.json",
  "runs": [
    {
      "tool": {
        "driver": {
          "rules": [
            {"id": "first.rule", "shortDescription": {"text": "first rule"}}
          ]
        }
      },
      "results": [
        {
          "level": "warning",
          "message": {"text": "standalone finding"},
          "locations": [
            {
              "physicalLocation": {
                "artifactLocation": {"uri": "src/app.go"},
                "region": {"startLine": 11}
              }
            }
          ]
        }
      ]
    }
  ]
}`)

	report, err := ImportExternalReports([]ExternalReportInput{{Tool: "semgrep", Path: sarifPath}})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(report.Findings))
	}
	finding := report.Findings[0]
	if finding.RuleID != "" {
		t.Fatalf("rule id = %q, want empty when SARIF omits ruleId and ruleIndex", finding.RuleID)
	}
	if finding.Message != "standalone finding" || finding.Path != "src/app.go" || finding.Line != 11 {
		t.Fatalf("unexpected SARIF finding: %#v", finding)
	}
}

func TestImportExternalReportsReadsTruffleHogJSONLines(t *testing.T) {
	dir := t.TempDir()
	path := writeFixture(t, dir, "trufflehog.jsonl", `{"DetectorName":"Stripe","Redacted":"stripe token","SourceMetadata":{"Data":{"Filesystem":{"file":"one.go","line":3}}}}
{"DetectorName":"GitHub","Redacted":"github token","SourceMetadata":{"Data":{"Git":{"file":"two.go","line":9}}}}
`)

	report, err := ImportExternalReports([]ExternalReportInput{{Tool: "trufflehog", Path: path}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Total != 2 {
		t.Fatalf("total = %d, want 2", report.Summary.Total)
	}
	if got := bucketCount(report.Summary.ByPathLine, "trufflehog", "secrets", "one.go", 3); got != 1 {
		t.Fatalf("filesystem bucket = %d, want 1", got)
	}
	if got := bucketCount(report.Summary.ByPathLine, "trufflehog", "secrets", "two.go", 9); got != 1 {
		t.Fatalf("git bucket = %d, want 1", got)
	}
}

func TestRenderExternalMarkdownEscapesCells(t *testing.T) {
	report := ExternalReport{
		Version: ExternalSchemaVersion,
		Reports: []ExternalSource{{
			Tool:     "tool|name",
			Format:   "json|sarif",
			Path:     "reports/a|b`c.json",
			Findings: 1,
		}},
		Summary: ExternalSummary{
			ByTool:     []ExternalBucket{{Tool: "tool|name", Count: 1}},
			ByCategory: []ExternalBucket{{Tool: "tool|name", Category: "secret|config", Count: 1}},
			ByPathLine: []ExternalBucket{{Tool: "tool|name", Category: "secret|config", Path: "src/a|b`c.go", Line: 4, Count: 1}},
		},
	}

	markdown := RenderExternalMarkdown(report)
	for _, want := range []string{
		`tool\|name`,
		`json\|sarif`,
		`reports/a\|b'c.json`,
		`secret\|config`,
		`src/a\|b'c.go`,
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("Markdown missing escaped %q:\n%s", want, markdown)
		}
	}
}

func TestImportExternalReportsRejectsUnknownTool(t *testing.T) {
	_, err := ImportExternalReports([]ExternalReportInput{{Tool: "unknown", Path: "report.json"}})
	if err == nil || !strings.Contains(err.Error(), "unsupported external tool") {
		t.Fatalf("expected unsupported tool error, got %v", err)
	}
}

func TestImportExternalReportsRejectsEmptyReport(t *testing.T) {
	dir := t.TempDir()
	path := writeFixture(t, dir, "empty.json", " \n")
	_, err := ImportExternalReports([]ExternalReportInput{{Tool: "gitleaks", Path: path}})
	if err == nil || !strings.Contains(err.Error(), "empty report") {
		t.Fatalf("expected empty report error, got %v", err)
	}
}

func TestImportExternalReportsRejectsMissingPath(t *testing.T) {
	_, err := ImportExternalReports([]ExternalReportInput{{Tool: "gitleaks"}})
	if err == nil || !strings.Contains(err.Error(), "path is required") {
		t.Fatalf("expected missing path error, got %v", err)
	}
}

func bucketCount(buckets []ExternalBucket, tool, category, path string, line int) int {
	for _, bucket := range buckets {
		if bucket.Tool == tool && bucket.Category == category && bucket.Path == path && bucket.Line == line {
			return bucket.Count
		}
	}
	return 0
}

func writeFixture(t *testing.T, dir, name, data string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
