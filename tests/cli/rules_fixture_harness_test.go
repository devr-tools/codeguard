package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/rulepacktest"
)

const rulePackTestConfig = `name: rules-test
targets:
  - name: repo
    path: .
    language: go
rule_packs:
  - name: policy
    rules:
      - id: custom.no-todo
        title: No TODO markers
        message: TODO marker found
        content_regex: "TODO"
      - id: custom.no-env-file
        title: No env files
        message: env file committed
        paths: [".env", "**/.env"]
      - id: custom.no-body-logs
        title: Never log request bodies
        message: request body logged
        natural_language: never log request bodies in handlers
`

func writeRulePackFixtures(t *testing.T, files map[string]string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.yaml")
	if err := os.WriteFile(configPath, []byte(rulePackTestConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	fixtures := filepath.Join(dir, "fixtures")
	for rel, body := range files {
		path := filepath.Join(fixtures, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return configPath, fixtures
}

func TestParseAnnotationsAcrossCommentSyntaxes(t *testing.T) {
	content := strings.Join([]string{
		"// ruleid: custom.a",               // 1 -> 2
		"code()",                            // 2
		"# ok: custom.a, custom.b",          // 3 -> 5 (stacked)
		"-- ruleid: custom.c",               // 4 -> 5
		"code()",                            // 5
		"<!-- ruleid-file: custom.d -->",    // 6 -> file level
		"/* ok: custom.e */",                // 7 -> 8
		"code() // ruleid: custom.trailing", // 8: not comment-only, ignored
		"; ruleid: custom.f",                // 9 -> end of file
	}, "\n")
	got := rulepacktest.ParseAnnotations(content)
	want := []rulepacktest.Annotation{
		{Kind: "ruleid", RuleID: "custom.a", Line: 2, SourceLine: 1},
		{Kind: "ok", RuleID: "custom.a", Line: 5, SourceLine: 3},
		{Kind: "ok", RuleID: "custom.b", Line: 5, SourceLine: 3},
		{Kind: "ruleid", RuleID: "custom.c", Line: 5, SourceLine: 4},
		{Kind: "ruleid", RuleID: "custom.d", Line: 0, SourceLine: 6},
		{Kind: "ok", RuleID: "custom.e", Line: 8, SourceLine: 7},
		{Kind: "ruleid", RuleID: "custom.f", Line: -1, SourceLine: 9},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("annotations =\n%+v\nwant\n%+v", got, want)
	}
	if got := rulepacktest.ParseAnnotations("status ok: custom.a\nruleid: custom.a\n"); len(got) != 0 {
		t.Fatalf("prose and bare text must not be annotations: %+v", got)
	}
}

func TestRulesTestPassesMatchingFixtures(t *testing.T) {
	configPath, fixtures := writeRulePackFixtures(t, map[string]string{
		"app.py":          "# ruleid: custom.no-todo\nx = 1  # TODO remove\n# ok: custom.no-todo\ny = 2\n",
		"handlers/api.go": "package api\n\n// ruleid: custom.no-todo\n// TODO: wire auth\nvar a = 1\n",
		"config/.env":     "# ruleid-file: custom.no-env-file\nA=1\n",
		"README.txt":      "no annotations and no findings\n",
	})
	stdout, stderr, code := runDiscoveryCLI(t, "rules", "test", "-config", configPath, fixtures)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	for _, want := range []string{
		"PASS " + filepath.ToSlash(filepath.Join(fixtures, "app.py")),
		"PASS " + filepath.ToSlash(filepath.Join(fixtures, "handlers/api.go")),
		"PASS " + filepath.ToSlash(filepath.Join(fixtures, "config/.env")),
		"skipped custom.no-body-logs: natural-language rule needs an AI runtime",
		"3 passed, 0 failed",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "README.txt") {
		t.Fatalf("a file with no annotations and no findings is not a test:\n%s", stdout)
	}
}

func TestRulesTestReportsMismatchesWithLineNumbers(t *testing.T) {
	configPath, fixtures := writeRulePackFixtures(t, map[string]string{
		"app.py": strings.Join([]string{
			"# ruleid: custom.no-todo", // 1
			"x = 1  # TODO remove",     // 2: expected and reported
			"# ruleid: custom.no-todo", // 3
			"y = 2",                    // 4: expected, not reported
			"# ok: custom.no-todo",     // 5
			"z = 3  # TODO keep",       // 6: ok, but reported
			"w = 4  # TODO stray",      // 7: not annotated
			"# ruleid: custom.unknown", // 8
			"v = 5",                    // 9
			"",
		}, "\n"),
	})
	stdout, _, code := runDiscoveryCLI(t, "rules", "test", "-config", configPath, filepath.Join(fixtures, "app.py"))
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, stdout)
	}
	for _, want := range []string{
		"FAIL " + filepath.ToSlash(filepath.Join(fixtures, "app.py")),
		"  custom.no-todo: expected lines 2, 4, reported lines 2, 6, 7",
		"    missing: no finding on line 4",
		"    unexpected: finding on line 6 (annotated ok)",
		"    unexpected: finding on line 7 (no ruleid annotation)",
		"  custom.unknown: unknown rule id: no configured rule pack defines it",
		"0 passed, 1 failed",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestRulesTestJSONFormat(t *testing.T) {
	configPath, fixtures := writeRulePackFixtures(t, map[string]string{
		"app.py": "# ruleid: custom.no-todo\ny = 2\n",
	})
	stdout, _, code := runDiscoveryCLI(t, "rules", "test", "-config", configPath, "-format", "json", fixtures)
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, stdout)
	}
	var report rulepacktest.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout)
	}
	if report.Passed != 0 || report.Failed != 1 || len(report.Files) != 1 {
		t.Fatalf("report = %+v", report)
	}
	rule := report.Files[0].Rules[0]
	if rule.RuleID != "custom.no-todo" || !reflect.DeepEqual(rule.Expected, []int{2}) || len(rule.Reported) != 0 || !reflect.DeepEqual(rule.Missing, []int{2}) {
		t.Fatalf("rule = %+v", rule)
	}
	if !reflect.DeepEqual(report.Skipped, []string{"custom.no-body-logs"}) {
		t.Fatalf("skipped = %v", report.Skipped)
	}
}

func TestRulesTestFailsWithoutAnnotationsOrArguments(t *testing.T) {
	configPath, fixtures := writeRulePackFixtures(t, map[string]string{"notes.txt": "nothing here\n"})
	if _, stderr, code := runDiscoveryCLI(t, "rules", "test", "-config", configPath, fixtures); code != 1 || !strings.Contains(stderr, "no fixture file holds") {
		t.Fatalf("exit = %d stderr = %q, want a no-annotations failure", code, stderr)
	}
	if _, stderr, code := runDiscoveryCLI(t, "rules", "test", "-config", configPath); code != 1 || !strings.Contains(stderr, "name at least one fixture") {
		t.Fatalf("exit = %d stderr = %q, want a usage failure", code, stderr)
	}
	if _, stderr, code := runDiscoveryCLI(t, "rules", "test", "-config", configPath, "-format", "sarif", fixtures); code != 1 || !strings.Contains(stderr, "invalid rules test format") {
		t.Fatalf("exit = %d stderr = %q, want a format error", code, stderr)
	}
}
