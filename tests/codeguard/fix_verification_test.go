package codeguard_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

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
