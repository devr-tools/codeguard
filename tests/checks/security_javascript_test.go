package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestSecurityCheckFindsNativeJavaScriptPatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "index.js"), strings.Join([]string{
		"const { exec, spawn } = require(\"node:child_process\")",
		"process.env.NODE_TLS_REJECT_UNAUTHORIZED = \"0\"",
		"const target = document.createElement(\"div\")",
		"exec(\"ls\")",
		"spawn(\"ls\", [], { shell: true })",
		"setTimeout(\"danger()\", 50)",
		"window.postMessage({ ok: true }, \"*\")",
		"target.innerHTML = \"<p>unsafe</p>\"",
		"",
	}, "\n"))

	cfg := codeguard.ExampleConfig()
	cfg.Name = "security-javascript-native"
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: "javascript"}}
	cfg.Checks.Security = true
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.Quality = false
	cfg.Checks.SecurityRules.GovulncheckMode = "off"

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "fail")
	assertFindingRulePresent(t, report, "Security", "security.javascript.insecure-tls")
	assertFindingRulePresent(t, report, "Security", "security.javascript.shell-execution")
	assertFindingRulePresent(t, report, "Security", "security.javascript.string-timer-code")
	assertFindingRulePresent(t, report, "Security", "security.javascript.postmessage-wildcard")
	assertFindingRulePresent(t, report, "Security", "security.javascript.unsafe-html-sink")
}

func TestSecurityCheckFindsNewNativeTypeScriptPatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "index.ts"), strings.Join([]string{
		"export function scheduleWork(target: Window) {",
		"  setInterval(\"danger()\", 100);",
		"  target.postMessage({ ok: true }, \"*\");",
		"}",
		"",
	}, "\n"))

	cfg := codeguard.ExampleConfig()
	cfg.Name = "security-typescript-extra"
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: "typescript"}}
	cfg.Checks.Security = true
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.Quality = false
	cfg.Checks.SecurityRules.GovulncheckMode = "off"

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "warn")
	assertFindingRulePresent(t, report, "Security", "security.typescript.string-timer-code")
	assertFindingRulePresent(t, report, "Security", "security.typescript.postmessage-wildcard")
}

func TestSecurityCheckIgnoresJavaScriptPatternsInsideStrings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "safe.js"), strings.Join([]string{
		"const examples = [",
		"  \"setTimeout('danger()', 50)\",",
		"  \"window.postMessage({}, '*')\",",
		"  \"document.body.innerHTML = value\",",
		"];",
		"export function sample() {",
		"  return examples.join('\\n');",
		"}",
		"",
	}, "\n"))

	cfg := codeguard.ExampleConfig()
	cfg.Name = "security-javascript-safe"
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: "javascript"}}
	cfg.Checks.Security = true
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.Quality = false
	cfg.Checks.SecurityRules.GovulncheckMode = "off"

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "pass")
}
