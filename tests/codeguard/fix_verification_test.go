package codeguard_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func readAPITestFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(contents)
}

func TestVerifyFixReturnsOnlyVerifiedGoPatch(t *testing.T) {
	dir := t.TempDir()
	writeAPITestFile(t, filepath.Join(dir, "go.mod"), "module example.com/fixverify\n\ngo 1.23.0\n")
	writeAPITestFile(t, filepath.Join(dir, "service.go"), `package fixverify

import "errors"

func run() error {
	err := doThing()
	_ = err
	return nil
}

func doThing() error {
	return errors.New("boom")
}
`)
	writeAPITestFile(t, filepath.Join(dir, "service_test.go"), `package fixverify

import "testing"

func TestRunReturnsUnderlyingError(t *testing.T) {
	if err := run(); err == nil || err.Error() != "boom" {
		t.Fatalf("run() = %v, want boom", err)
	}
}
`)

	cfg := qualityOnlyConfig(dir, "verify-fix-pass")
	finding := firstFinding(t, cfg)

	diff := strings.Join([]string{
		"diff --git a/service.go b/service.go",
		"--- a/service.go",
		"+++ b/service.go",
		"@@ -3,9 +3,10 @@ import \"errors\"",
		" ",
		" func run() error {",
		"-\terr := doThing()",
		"-\t_ = err",
		"-\treturn nil",
		"+\tif err := doThing(); err != nil {",
		"+\t\treturn err",
		"+\t}",
		"+\treturn nil",
		" }",
		" ",
		" func doThing() error {",
		"",
	}, "\n")

	result, err := codeguard.VerifyFix(context.Background(), cfg, finding, codeguard.FixCandidate{
		Summary: "return the underlying error instead of swallowing it",
		Diff:    diff,
	}, codeguard.FixOptions{})
	if err != nil {
		t.Fatalf("verify fix: %v", err)
	}
	if result.Report.Summary.TotalFindings != 0 {
		t.Fatalf("expected verified patch to clear changed-line findings, got %#v", result.Report.Summary)
	}
	if len(result.TestResults) != 1 {
		t.Fatalf("expected one inferred test command, got %#v", result.TestResults)
	}
	if result.TestResults[0].CheckName != "go test ." {
		t.Fatalf("unexpected inferred test command: %#v", result.TestResults[0])
	}
	if !strings.Contains(result.Diff, "return err") {
		t.Fatalf("expected verified diff in result, got %q", result.Diff)
	}
}

func TestVerifyFixBatchVerifiesDeterministicAggregateAndReportsSkippedItems(t *testing.T) {
	dir := t.TempDir()
	writeAPITestFile(t, filepath.Join(dir, "go.mod"), "module example.com/fixbatch\n\ngo 1.23.0\n")
	writeAPITestFile(t, filepath.Join(dir, "service.go"), "package fixbatch\n\nfunc run(){ }\n")
	writeAPITestFile(t, filepath.Join(dir, "service_test.go"), "package fixbatch\n\nimport \"testing\"\n\nfunc TestRun(t *testing.T) { run() }\n")

	cfg := qualityOnlyConfig(dir, "verify-fix-batch")
	diff := strings.Join([]string{
		"diff --git a/service.go b/service.go",
		"--- a/service.go",
		"+++ b/service.go",
		"@@ -1,3 +1,3 @@",
		" package fixbatch",
		" ",
		"-func run(){ }",
		"+func run() {}",
		"",
	}, "\n")

	result, err := codeguard.VerifyFixBatch(context.Background(), codeguard.FixBatchRequest{
		Config: cfg,
		Items: []codeguard.FixBatchItem{
			{Finding: codeguard.Finding{RuleID: "quality.gofmt", Fingerprint: "format"}, Candidate: codeguard.FixCandidate{Diff: diff}},
			{Finding: codeguard.Finding{RuleID: "quality.gofmt", Fingerprint: "duplicate"}, Candidate: codeguard.FixCandidate{Diff: diff}},
			{Finding: codeguard.Finding{RuleID: "quality.max-file-lines", Fingerprint: "guided"}, Candidate: codeguard.FixCandidate{Diff: diff}},
		},
	})
	if err != nil {
		t.Fatalf("verify fix batch: %v", err)
	}
	if got, want := result.Included, []int{0}; !slices.Equal(got, want) {
		t.Fatalf("included = %#v, want %#v", got, want)
	}
	if result.Verification.Report.Summary.TotalFindings != 0 {
		t.Fatalf("expected clean aggregate report, got %#v", result.Verification.Report.Summary)
	}
	if len(result.Skipped) != 2 || result.Skipped[0].Reason != codeguard.FixBatchReasonConflictingFiles || result.Skipped[1].Reason != codeguard.FixBatchReasonNonDeterministic {
		t.Fatalf("skipped = %#v", result.Skipped)
	}
	if source := readAPITestFile(t, filepath.Join(dir, "service.go")); !strings.Contains(source, "func run(){ }") {
		t.Fatalf("batch verification modified source target: %q", source)
	}
}

func TestVerifyFixRejectsPatchWhenNearestTestFails(t *testing.T) {
	dir := t.TempDir()
	writeAPITestFile(t, filepath.Join(dir, "go.mod"), "module example.com/fixverify\n\ngo 1.23.0\n")
	writeAPITestFile(t, filepath.Join(dir, "service.go"), `package fixverify

import "errors"

func run() error {
	err := doThing()
	_ = err
	return nil
}

func doThing() error {
	return errors.New("boom")
}
`)
	writeAPITestFile(t, filepath.Join(dir, "service_test.go"), `package fixverify

import "testing"

func TestRunReturnsUnderlyingError(t *testing.T) {
	if err := run(); err == nil || err.Error() != "boom" {
		t.Fatalf("run() = %v, want boom", err)
	}
}
`)

	cfg := qualityOnlyConfig(dir, "verify-fix-fail")
	finding := firstFinding(t, cfg)

	diff := strings.Join([]string{
		"diff --git a/service.go b/service.go",
		"--- a/service.go",
		"+++ b/service.go",
		"@@ -3,9 +3,10 @@ import \"errors\"",
		" ",
		" func run() error {",
		"-\terr := doThing()",
		"-\t_ = err",
		"-\treturn nil",
		"+\tif err := doThing(); err != nil {",
		"+\t\treturn nil",
		"+\t}",
		"+\treturn nil",
		" }",
		" ",
		" func doThing() error {",
		"",
	}, "\n")

	_, err := codeguard.VerifyFix(context.Background(), cfg, finding, codeguard.FixCandidate{
		Summary: "remove the warning but still hide the error",
		Diff:    diff,
	}, codeguard.FixOptions{})
	if err == nil {
		t.Fatal("expected verification failure")
	}
	if !strings.Contains(err.Error(), "verification test") {
		t.Fatalf("expected test verification error, got %v", err)
	}
}

func TestVerifyFixFailsClosedWithoutInferableTests(t *testing.T) {
	dir := t.TempDir()
	writeAPITestFile(t, filepath.Join(dir, "prompts", "system.prompt"), "Use ${OPENAI_API_KEY} for downstream calls.\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "verify-fix-no-tests"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = true
	cfg.Checks.CI = false

	finding := firstFinding(t, cfg)
	diff := strings.Join([]string{
		"diff --git a/prompts/system.prompt b/prompts/system.prompt",
		"--- a/prompts/system.prompt",
		"+++ b/prompts/system.prompt",
		"@@ -1 +1 @@",
		"-Use ${OPENAI_API_KEY} for downstream calls.",
		"+Keep prompts generic.",
		"",
	}, "\n")

	_, err := codeguard.VerifyFix(context.Background(), cfg, finding, codeguard.FixCandidate{
		Summary: "remove secret interpolation from the prompt",
		Diff:    diff,
	}, codeguard.FixOptions{})
	if err == nil {
		t.Fatal("expected missing-tests verification failure")
	}
	if !strings.Contains(err.Error(), "no verification tests could be inferred") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateVerifiedFixUsesGeneratorAndVerification(t *testing.T) {
	dir := t.TempDir()
	writeAPITestFile(t, filepath.Join(dir, "go.mod"), "module example.com/fixverify\n\ngo 1.23.0\n")
	writeAPITestFile(t, filepath.Join(dir, "service.go"), `package fixverify

import "errors"

func run() error {
	err := doThing()
	_ = err
	return nil
}

func doThing() error {
	return errors.New("boom")
}
`)
	writeAPITestFile(t, filepath.Join(dir, "service_test.go"), `package fixverify

import "testing"

func TestRunReturnsUnderlyingError(t *testing.T) {
	if err := run(); err == nil || err.Error() != "boom" {
		t.Fatalf("run() = %v, want boom", err)
	}
}
`)

	cfg := qualityOnlyConfig(dir, "generate-verified-fix")
	finding := firstFinding(t, cfg)
	diff := strings.Join([]string{
		"diff --git a/service.go b/service.go",
		"--- a/service.go",
		"+++ b/service.go",
		"@@ -3,9 +3,10 @@ import \"errors\"",
		" ",
		" func run() error {",
		"-\terr := doThing()",
		"-\t_ = err",
		"-\treturn nil",
		"+\tif err := doThing(); err != nil {",
		"+\t\treturn err",
		"+\t}",
		"+\treturn nil",
		" }",
		" ",
		" func doThing() error {",
		"",
	}, "\n")

	generator := &stubFixGenerator{candidate: codeguard.FixCandidate{
		Summary: "return the error to the caller",
		Diff:    diff,
	}}
	result, err := codeguard.GenerateVerifiedFix(context.Background(), codeguard.FixGenerateRequest{
		Config:    cfg,
		Finding:   finding,
		Analysis:  "swallowed error",
		Generator: generator,
		Options:   codeguard.FixOptions{},
	})
	if err != nil {
		t.Fatalf("generate verified fix: %v", err)
	}
	if generator.calls != 1 {
		t.Fatalf("generator calls = %d, want 1", generator.calls)
	}
	if result.Report.Summary.TotalFindings != 0 {
		t.Fatalf("expected verified report, got %#v", result.Report.Summary)
	}
}
