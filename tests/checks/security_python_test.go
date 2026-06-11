package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestSecurityCheckFindsNativePythonPatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app.py"), strings.Join([]string{
		"import os",
		"import subprocess",
		"import requests",
		"requests.get('https://example.com', verify=False)",
		"subprocess.run('ls', shell=True)",
		"eval('danger')",
		"os.system('ls')",
		"",
	}, "\n"))

	cfg := codeguard.ExampleConfig()
	cfg.Name = "security-python-native"
	cfg.Targets = []codeguard.TargetConfig{{Name: "api", Path: dir, Language: "python"}}
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
	assertFindingRulePresent(t, report, "Security", "security.python.insecure-tls")
	assertFindingRulePresent(t, report, "Security", "security.python.shell-execution")
	assertFindingRulePresent(t, report, "Security", "security.python.dynamic-code")
}

func TestSecurityCheckFailsForConfiguredPythonCommand(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app.py"), "print('hello')\n")
	script := filepath.Join(dir, "fake-bandit.sh")
	writeExecutableFile(t, script, "#!/bin/sh\necho 'app.py:1 insecure construct'\nexit 1\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "security-python-command"
	cfg.Targets = []codeguard.TargetConfig{{Name: "api", Path: dir, Language: "python"}}
	cfg.Checks.Security = true
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.Quality = false
	cfg.Checks.SecurityRules.GovulncheckMode = "required"
	cfg.Checks.SecurityRules.LanguageCommands = map[string][]codeguard.CommandCheckConfig{
		"python": {{
			Name:    "bandit",
			Command: script,
		}},
	}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "fail")
	if len(report.Sections[0].Findings) == 0 {
		t.Fatal("expected command finding")
	}
	if !strings.Contains(report.Sections[0].Findings[0].Message, "bandit") {
		t.Fatalf("expected command name in message, got %q", report.Sections[0].Findings[0].Message)
	}
}

func TestSecurityCheckWarnsForNativePythonSecurityPatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app.py"), "import subprocess\nsubprocess.run('echo hi', shell=True)\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "security-python-native"
	cfg.Targets = []codeguard.TargetConfig{{Name: "api", Path: dir, Language: "python"}}
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
