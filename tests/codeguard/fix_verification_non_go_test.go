package codeguard_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestVerifyFixReturnsOnlyVerifiedPythonPatch(t *testing.T) {
	dir := t.TempDir()
	writeAPITestFile(t, filepath.Join(dir, "service.py"), `def run():
    try:
        do_thing()
    except Exception:
        pass


def do_thing():
    raise RuntimeError("boom")
`)
	writeAPITestFile(t, filepath.Join(dir, "tests", "test_service.py"), `import unittest

import service


class ServiceTests(unittest.TestCase):
    def test_run_reraises_the_underlying_error(self):
        with self.assertRaisesRegex(RuntimeError, "boom"):
            service.run()


if __name__ == "__main__":
    unittest.main()
`)

	cfg := qualityOnlyConfigForLanguage(dir, "verify-fix-python", "python")
	finding := firstFinding(t, cfg)

	diff := strings.Join([]string{
		"diff --git a/service.py b/service.py",
		"--- a/service.py",
		"+++ b/service.py",
		"@@ -1,7 +1,7 @@",
		" def run():",
		"     try:",
		"         do_thing()",
		"     except Exception:",
		"-        pass",
		"+        raise",
		" ",
		" ",
		" def do_thing():",
		"",
	}, "\n")

	result, err := codeguard.VerifyFix(context.Background(), cfg, finding, codeguard.FixCandidate{
		Summary: "re-raise the swallowed exception",
		Diff:    diff,
	}, codeguard.FixOptions{})
	if err != nil {
		t.Fatalf("verify fix: %v", err)
	}
	if len(result.TestResults) != 1 {
		t.Fatalf("expected one inferred python test command, got %#v", result.TestResults)
	}
	if !strings.Contains(result.TestResults[0].CheckName, "python3 -m unittest tests/test_service.py") {
		t.Fatalf("unexpected inferred python command: %#v", result.TestResults[0])
	}
}

func TestVerifyFixReturnsOnlyVerifiedJavaScriptPatch(t *testing.T) {
	dir := t.TempDir()
	writeAPITestFile(t, filepath.Join(dir, "service.js"), `function run() {
  try {
    doThing();
  } catch (err) {}
}

function doThing() {
  throw new Error("boom");
}

module.exports = { run };
`)
	writeAPITestFile(t, filepath.Join(dir, "service.test.js"), `const test = require("node:test");
const assert = require("node:assert/strict");
const { run } = require("./service");

test("run rethrows the underlying error", () => {
  assert.throws(() => run(), /boom/);
});
`)

	cfg := qualityOnlyConfigForLanguage(dir, "verify-fix-javascript", "javascript")
	finding := firstFinding(t, cfg)

	diff := strings.Join([]string{
		"diff --git a/service.js b/service.js",
		"--- a/service.js",
		"+++ b/service.js",
		"@@ -1,6 +1,8 @@",
		" function run() {",
		"   try {",
		"     doThing();",
		"-  } catch (err) {}",
		"+  } catch (err) {",
		"+    throw err;",
		"+  }",
		" }",
		" ",
		" function doThing() {",
		"",
	}, "\n")

	result, err := codeguard.VerifyFix(context.Background(), cfg, finding, codeguard.FixCandidate{
		Summary: "rethrow the swallowed error",
		Diff:    diff,
	}, codeguard.FixOptions{})
	if err != nil {
		t.Fatalf("verify fix: %v", err)
	}
	if len(result.TestResults) != 1 {
		t.Fatalf("expected one inferred javascript test command, got %#v", result.TestResults)
	}
	if result.TestResults[0].CheckName != "node --test service.test.js" {
		t.Fatalf("unexpected inferred javascript command: %#v", result.TestResults[0])
	}
}

func TestVerifyFixFallsBackToPackageManagerTestsForJavaScript(t *testing.T) {
	dir := t.TempDir()
	writeAPITestFile(t, filepath.Join(dir, "package.json"), `{
  "name": "fixverify-js",
  "scripts": {
    "test": "node tests/run-check.js"
  }
}
`)
	writeAPITestFile(t, filepath.Join(dir, "service.js"), `function run() {
  try {
    doThing();
  } catch (err) {}
}

function doThing() {
  throw new Error("boom");
}

module.exports = { run };
`)
	writeAPITestFile(t, filepath.Join(dir, "tests", "run-check.js"), `const assert = require("node:assert/strict");
const { run } = require("../service");

assert.throws(() => run(), /boom/);
`)

	cfg := qualityOnlyConfigForLanguage(dir, "verify-fix-javascript-npm", "javascript")
	finding := firstFinding(t, cfg)

	diff := strings.Join([]string{
		"diff --git a/service.js b/service.js",
		"--- a/service.js",
		"+++ b/service.js",
		"@@ -1,6 +1,8 @@",
		" function run() {",
		"   try {",
		"     doThing();",
		"-  } catch (err) {}",
		"+  } catch (err) {",
		"+    throw err;",
		"+  }",
		" }",
		" ",
		" function doThing() {",
		"",
	}, "\n")

	result, err := codeguard.VerifyFix(context.Background(), cfg, finding, codeguard.FixCandidate{
		Summary: "rethrow the swallowed error",
		Diff:    diff,
	}, codeguard.FixOptions{})
	if err != nil {
		t.Fatalf("verify fix: %v", err)
	}
	if len(result.TestResults) != 1 {
		t.Fatalf("expected one inferred package-manager test command, got %#v", result.TestResults)
	}
	if result.TestResults[0].CheckName != "npm test" {
		t.Fatalf("unexpected inferred package-manager command: %#v", result.TestResults[0])
	}
}
