package checks_test

import (
	"context"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func qualityCommandReport(t *testing.T, dir string, commands []codeguard.CommandCheckConfig) codeguard.Report {
	t.Helper()
	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-command-check"
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: "typescript"}}
	cfg.Checks.Quality = true
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.QualityRules.LanguageCommands = map[string][]codeguard.CommandCheckConfig{"typescript": commands}
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}

func qualityCommandMessages(t *testing.T, report codeguard.Report) []string {
	t.Helper()
	messages := make([]string, 0)
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID != "quality.command-check" {
				continue
			}
			if finding.Level != "fail" {
				t.Errorf("quality.command-check level = %q, want fail", finding.Level)
			}
			messages = append(messages, finding.Message)
		}
	}
	sort.Strings(messages)
	return messages
}

// Each failing configured command yields exactly one quality.command-check
// finding naming the target and command and carrying the tool output; a
// passing command in the same list yields none.
func TestQualityCommandCheckEmitsOneFindingPerFailingCommand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "index.ts"), "export const answer = 42;\n")
	failing := filepath.Join(dir, "fake-eslint.sh")
	writeExecutableFile(t, failing, "#!/bin/sh\necho 'src/index.ts:1:1 no-unused-vars'\nexit 1\n")
	silent := filepath.Join(dir, "fake-tsc.sh")
	writeExecutableFile(t, silent, "#!/bin/sh\nexit 2\n")
	passing := filepath.Join(dir, "fake-prettier.sh")
	writeExecutableFile(t, passing, "#!/bin/sh\necho 'all files formatted'\nexit 0\n")

	report := qualityCommandReport(t, dir, []codeguard.CommandCheckConfig{
		{Name: "eslint", Command: failing},
		{Name: "tsc", Command: silent},
		{Name: "prettier", Command: passing},
	})

	want := []string{
		`target "web" quality command "eslint" failed: src/index.ts:1:1 no-unused-vars`,
		`target "web" quality command "tsc" failed: tsc failed: exit status 2`,
	}
	if got := qualityCommandMessages(t, report); !reflect.DeepEqual(got, want) {
		t.Fatalf("quality.command-check messages =\n  %q\nwant\n  %q", got, want)
	}
	assertSectionStatus(t, report, "Code Quality", "fail")
}

func TestQualityCommandCheckSilentWhenCommandsPass(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "index.ts"), "export const answer = 42;\n")
	passing := filepath.Join(dir, "fake-tsc.sh")
	writeExecutableFile(t, passing, "#!/bin/sh\necho 'no errors'\nexit 0\n")

	report := qualityCommandReport(t, dir, []codeguard.CommandCheckConfig{{Name: "tsc", Command: passing}})
	if got := qualityCommandMessages(t, report); len(got) != 0 {
		t.Fatalf("passing command produced quality.command-check findings: %q", got)
	}

	// No command configured for the target's language: nothing runs.
	none := qualityCommandReport(t, dir, nil)
	if got := qualityCommandMessages(t, none); len(got) != 0 {
		t.Fatalf("unconfigured language produced quality.command-check findings: %q", got)
	}
}
