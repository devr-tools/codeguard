package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestSecurityCheckFailsForHardcodedCredential(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.go"), "package main\nconst awsKey = \""+cred("AKIA", "1234567890ABCDEF")+"\"\n")

	report, err := codeguard.Run(context.Background(), codeguard.Config{
		Name: "security-test",
		Targets: []codeguard.TargetConfig{{
			Name:     "repo",
			Path:     dir,
			Language: "go",
		}},
		Checks: codeguard.CheckConfig{
			Security: true,
		},
		Output: codeguard.OutputConfig{Format: "text"},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "fail")
	assertFindingRulePresent(t, report, "Security", "security.hardcoded-credential")
}

func TestSecurityCheckWarnsForNameBasedSecret(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.go"), "package main\nconst apiKey = \"super-secret-token\"\n")

	report, err := codeguard.Run(context.Background(), codeguard.Config{
		Name: "security-name-based-secret",
		Targets: []codeguard.TargetConfig{{
			Name:     "repo",
			Path:     dir,
			Language: "go",
		}},
		Checks: codeguard.CheckConfig{
			Security: true,
		},
		Output: codeguard.OutputConfig{Format: "text"},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "warn")
	assertFindingRulePresent(t, report, "Security", "security.hardcoded-secret")
	assertFindingConfidence(t, report, "Security", "security.hardcoded-secret", "low")
}

func TestSecurityCheckWarnsForShellExecution(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "exec.go"), "package main\nimport \"os/exec\"\nfunc main(){exec.Command(\"sh\")}\n")

	report, err := codeguard.Run(context.Background(), codeguard.Config{
		Name: "security-warn-test",
		Targets: []codeguard.TargetConfig{{
			Name:     "repo",
			Path:     dir,
			Language: "go",
		}},
		Checks: codeguard.CheckConfig{
			Security: true,
		},
		Output: codeguard.OutputConfig{Format: "text"},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "warn")
	assertFindingRulePresent(t, report, "Security", "security.shell-execution")
}

func TestSecurityCheckFailsWhenGovulncheckIsRequiredButMissing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\nfunc main() {}\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "govulncheck-required"
	cfg.Targets = []codeguard.TargetConfig{{
		Name:     "repo",
		Path:     dir,
		Language: "go",
	}}
	cfg.Checks.Security = true
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.Quality = false
	cfg.Checks.SecurityRules.GovulncheckMode = "required"
	cfg.Checks.SecurityRules.GovulncheckCommand = "missing-govulncheck-binary"

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "fail")
}

func TestSecurityCheckWarnsWhenGovulncheckIsAutoButMissing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\nfunc main() {}\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "govulncheck-auto"
	cfg.Targets = []codeguard.TargetConfig{{
		Name:     "repo",
		Path:     dir,
		Language: "go",
	}}
	cfg.Checks.Security = true
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.Quality = false
	cfg.Checks.SecurityRules.GovulncheckMode = "auto"
	cfg.Checks.SecurityRules.GovulncheckCommand = "missing-govulncheck-binary"

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "warn")
}

func TestSecurityCheckSurfacesStructuredGovulncheckFindings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\nfunc main() {}\n")
	script := filepath.Join(dir, "fake-govulncheck.sh")
	writeExecutableFile(t, script, "#!/bin/sh\necho 'Vulnerability #1: GO-2024-0001'\necho '  Found in: example.com/module@v1.0.0'\necho '  Fixed in: example.com/module@v1.0.1'\necho ''\necho 'Vulnerability #2: GO-2024-0002'\necho '  Found in: example.com/other@v0.9.0'\nexit 1\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "govulncheck-structured"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Security = true
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.Quality = false
	cfg.Checks.SecurityRules.GovulncheckMode = "required"
	cfg.Checks.SecurityRules.GovulncheckCommand = script

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "fail")
	assertSectionFindingCountAtLeast(t, report, "Security", 2)
}

func TestSecurityCheckFindsNativeTypeScriptPatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "index.ts"), strings.Join([]string{
		"import { exec as runExec, spawn as runSpawn } from \"node:child_process\"",
		"import { runInNewContext } from \"node:vm\"",
		"const target = document.createElement(\"div\")",
		"process.env.NODE_TLS_REJECT_UNAUTHORIZED = \"0\"",
		"runExec(\"ls\")",
		"runSpawn(",
		"  \"ls\",",
		"  [],",
		"  { shell: true },",
		")",
		"eval(\"danger\")",
		"runInNewContext(\"danger\")",
		"target.innerHTML = \"<p>unsafe</p>\"",
		"",
	}, "\n"))

	cfg := codeguard.ExampleConfig()
	cfg.Name = "security-typescript-native"
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

	assertSectionStatus(t, report, "Security", "fail")
	assertFindingRulePresent(t, report, "Security", "security.typescript.insecure-tls")
	assertFindingRulePresent(t, report, "Security", "security.typescript.shell-execution")
	assertFindingRulePresent(t, report, "Security", "security.typescript.dynamic-code")
	assertFindingRulePresent(t, report, "Security", "security.typescript.vm-dynamic-code")
	assertFindingRulePresent(t, report, "Security", "security.typescript.unsafe-html-sink")
}

func TestSecurityCheckIgnoresTypeScriptPatternsInsideStrings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "safe.ts"), strings.Join([]string{
		"const examples = [",
		"  \"eval('danger')\",",
		"  \"node.innerHTML = '<p>x</p>'\",",
		"  \"require('child_process').exec('ls')\",",
		"];",
		"export function sample() {",
		"  return examples.join(\"\\n\");",
		"}",
		"",
	}, "\n"))

	cfg := codeguard.ExampleConfig()
	cfg.Name = "security-typescript-safe"
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

	assertSectionStatus(t, report, "Security", "pass")
}

func TestSecurityCheckSkipsGovulncheckForNonGoTargets(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app.py"), "print('hello')\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "security-non-go-target"
	cfg.Targets = []codeguard.TargetConfig{{Name: "api", Path: dir, Language: "python"}}
	cfg.Checks.Security = true
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.Quality = false
	cfg.Checks.SecurityRules.GovulncheckMode = "required"
	cfg.Checks.SecurityRules.GovulncheckCommand = "missing-govulncheck-binary"

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "pass")
}

func TestSecurityCheckWarnsForNativeTypeScriptSecurityPatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app.ts"), "export function run(input: string) {\n  return eval(input);\n}\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "security-typescript-native"
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
}
