package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/codeguard/internal/benchmark"
)

func TestComparePrintsMarkdownSummary(t *testing.T) {
	dir := t.TempDir()
	codeguardPath := filepath.Join(dir, "codeguard.json")
	semgrepPath := filepath.Join(dir, "semgrep.json")
	writeResult(t, codeguardPath, benchmark.Result{
		Version: benchmark.SchemaVersion,
		Corpus:  "frozen-prs-v1",
		Tool:    "codeguard",
		Runs: []benchmark.RunResult{
			{ID: "go-1", Mode: "cold", Duration: 120 * time.Millisecond},
			{ID: "go-1", Mode: "warm", Attempt: 1, Duration: 40 * time.Millisecond},
			{ID: "go-1", Mode: "warm", Attempt: 2, Duration: 60 * time.Millisecond, ExitCode: 1},
		},
	})
	writeResult(t, semgrepPath, benchmark.Result{
		Version: benchmark.SchemaVersion,
		Corpus:  "frozen-prs-v1",
		Tool:    "semgrep",
		Runs: []benchmark.RunResult{
			{ID: "go-1", Mode: "cold", Duration: 2 * time.Second},
			{ID: "go-1", Mode: "warm", Attempt: 1, Duration: time.Second, Error: "timed out"},
		},
	})

	var stdout, stderr bytes.Buffer
	exitCode := runMain([]string{"compare", codeguardPath, semgrepPath}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("compare exit code %d, stderr %q", exitCode, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"# CodeGuard Benchmark Comparison",
		"| codeguard | frozen-prs-v1 | 1 | 3 | 120.0ms | 50.0ms | 60.0ms | 1 | 0 |",
		"| semgrep | frozen-prs-v1 | 1 | 2 | 2.00s | 1.00s | 1.50s | 0 | 1 |",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("compare output missing %q:\n%s", want, output)
		}
	}
}

func TestCompareEscapesMarkdownCells(t *testing.T) {
	dir := t.TempDir()
	resultPath := filepath.Join(dir, "result.json")
	writeResult(t, resultPath, benchmark.Result{
		Version: benchmark.SchemaVersion,
		Corpus:  "corp|us",
		Tool:    "tool|name",
		Runs: []benchmark.RunResult{
			{ID: "one", Mode: "cold", Duration: time.Millisecond},
		},
	})

	var stdout, stderr bytes.Buffer
	exitCode := runMain([]string{"compare", resultPath}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("compare exit code %d, stderr %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), `| tool\|name | corp\|us |`) {
		t.Fatalf("compare output did not escape pipes:\n%s", stdout.String())
	}
}

func TestHelpListsCompareCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := runMain([]string{"--help"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("help exit code %d, stderr %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "compare") || !strings.Contains(stdout.String(), "benchmark-report.md") {
		t.Fatalf("help output is not discoverable:\n%s", stdout.String())
	}
}

func TestExternalWritesJSONAndMarkdownReports(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "gitleaks.json")
	if err := os.WriteFile(inputPath, []byte(`[
  {"RuleID":"generic-api-key","Description":"Generic API Key","File":"config.go","StartLine":4,"Tags":["secret"]}
]`), 0o600); err != nil {
		t.Fatal(err)
	}
	jsonOut := filepath.Join(dir, "external.json")
	markdownOut := filepath.Join(dir, "external.md")

	var stdout, stderr bytes.Buffer
	exitCode := runMain([]string{"external", "-report", "gitleaks=" + inputPath, "-json-out", jsonOut, "-markdown-out", markdownOut}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("external exit code %d, stderr %q", exitCode, stderr.String())
	}
	// #nosec G304 -- jsonOut is a test-owned path under t.TempDir().
	jsonData, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(jsonData), `"tool": "gitleaks"`) || !strings.Contains(string(jsonData), `"category": "secrets"`) {
		t.Fatalf("unexpected JSON output:\n%s", string(jsonData))
	}
	// #nosec G304 -- markdownOut is a test-owned path under t.TempDir().
	markdownData, err := os.ReadFile(markdownOut)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markdownData), "# External Benchmark Summary") || !strings.Contains(string(markdownData), "| gitleaks | secrets | 1 |") {
		t.Fatalf("unexpected Markdown output:\n%s", string(markdownData))
	}
}

func TestExternalAcceptsColonReportSeparator(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "semgrep.json")
	if err := os.WriteFile(inputPath, []byte(`{
  "results": [
    {"check_id":"xss.innerhtml","path":"web.ts","start":{"line":2,"col":4},"extra":{"message":"innerHTML sink","severity":"WARNING"}}
  ]
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	jsonOut := filepath.Join(dir, "external.json")

	var stdout, stderr bytes.Buffer
	exitCode := runMain([]string{"external", "-report", "semgrep:" + inputPath, "-json-out", jsonOut}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("external exit code %d, stderr %q", exitCode, stderr.String())
	}
	// #nosec G304 -- jsonOut is a test-owned path under t.TempDir().
	data, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"category": "unsafe-html"`) {
		t.Fatalf("unexpected JSON output:\n%s", string(data))
	}
}

func TestExternalRequiresOutputPath(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "gitleaks.json")
	if err := os.WriteFile(inputPath, []byte(`[]`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := runMain([]string{"external", "-report", "gitleaks=" + inputPath}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
	if !strings.Contains(stderr.String(), "at least one of -json-out or -markdown-out is required") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestExternalRejectsMalformedReportFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := runMain([]string{"external", "-report", "gitleaks"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
	if !strings.Contains(stderr.String(), "tool=path") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func writeResult(t *testing.T, path string, result benchmark.Result) {
	t.Helper()
	if err := benchmark.WriteJSON(path, result); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
