package checks_test

import (
	"path/filepath"
	"testing"
)

func TestGoTestQualityConventionalAssertionHelpers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "conventional_test.go"), `package demo

import "testing"

func TestWithConventionalHelper(t *testing.T) {
	assertFoo(t, compute())
}

func TestWithRequireHelper(t *testing.T) {
	value := requireResult(t)
	verifyShape(t, value)
}

func TestWithoutAnyAssertion(t *testing.T) {
	processData(compute())
}
`)

	report := runScan(t, testQualityConfig(t, dir, "go"))

	assertRuleCount(t, report, "ci.test-without-assertion", 1)
	assertRuleCount(t, report, "ci.always-true-test-assertion", 0)
	assertRuleCount(t, report, "ci.conditional-assertion", 0)
	flagged := findingsForRule(report, "ci.test-without-assertion")[0]
	if flagged.Line != 14 {
		t.Fatalf("test-without-assertion line = %d, want 14", flagged.Line)
	}
}

func TestGoTestQualityExemptsHelperProcessTests(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "helper_process_test.go"), `package demo

import (
	"os"
	"os/exec"
	"testing"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(run(os.Args))
}

func TestCustomGuardHelperProcess(t *testing.T) {
	if os.Getenv("DEMO_WANT_SUBPROCESS") != "1" {
		return
	}
	cmd := exec.Command(os.Args[0])
	_ = cmd.Run()
	os.Exit(0)
}
`)

	report := runScan(t, testQualityConfig(t, dir, "go"))

	assertRuleCount(t, report, "ci.test-without-assertion", 0)
	assertRuleCount(t, report, "ci.always-true-test-assertion", 0)
	assertRuleCount(t, report, "ci.conditional-assertion", 0)
}

func TestGoTestQualityExemptsTestMain(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main_test.go"), `package demo

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
`)

	report := runScan(t, testQualityConfig(t, dir, "go"))

	assertRuleCount(t, report, "ci.test-without-assertion", 0)
	assertRuleCount(t, report, "ci.always-true-test-assertion", 0)
	assertRuleCount(t, report, "ci.conditional-assertion", 0)
}
