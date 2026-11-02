package checks_test

import (
	"context"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/config"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnerchecks "github.com/devr-tools/codeguard/internal/codeguard/runner/checks"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// Silently dropping an unusable secret pattern would shrink coverage with no
// signal, so each one must surface as a security.secrets-config failure.
// codeguard.Run rejects such patterns up front; the section builders
// (runnersupport.NewContext + runnerchecks.Build) do not validate, which is
// the only way they still reach the scanner.
func TestSecretsConfigReportsEveryUnusablePattern(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")

	report := unvalidatedSecretsScan(t, dir, &codeguard.SecretsRulesConfig{
		Enabled:       boolPtr(true),
		AllowPatterns: []string{`^ok$`, `(unclosed`},
		CustomPatterns: []codeguard.CustomSecretPattern{
			{ID: "", Regex: `ACME-[0-9]+`},
			{ID: "acme-broken", Regex: `[unterminated`},
			{ID: "acme-good", Regex: `ACME-[0-9]{10}`},
		},
	})

	want := []string{
		"secrets allow_patterns[1] skipped: " + regexpError(t, `(unclosed`),
		"secrets custom_patterns entry with an empty id skipped",
		`secrets custom_patterns["acme-broken"] skipped: ` + regexpError(t, `[unterminated`),
	}
	got := secretsConfigFindingMessages(t, report)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("security.secrets-config messages =\n  %q\nwant\n  %q", got, want)
	}
}

func TestSecretsConfigSilentForUsablePatterns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")

	for name, secrets := range map[string]*codeguard.SecretsRulesConfig{
		"default": nil,
		"valid patterns": {
			Enabled:        boolPtr(true),
			AllowPatterns:  []string{`^ok$`},
			CustomPatterns: []codeguard.CustomSecretPattern{{ID: "acme-good", Regex: `ACME-[0-9]{10}`}},
		},
	} {
		report := unvalidatedSecretsScan(t, dir, secrets)
		if got := secretsConfigFindingMessages(t, report); len(got) != 0 {
			t.Errorf("%s: unexpected security.secrets-config findings: %q", name, got)
		}
	}
}

// unvalidatedSecretsScan builds the security section the way the runner does,
// minus config.Validate, so invalid patterns reach the scanner.
func unvalidatedSecretsScan(t *testing.T, dir string, secrets *codeguard.SecretsRulesConfig) codeguard.Report {
	t.Helper()
	cfg := codeguard.ExampleConfig()
	cfg.Name = "security-secrets-config"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Security = true
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.Quality = false
	cfg.Checks.SupplyChain = false
	cfg.Checks.SecurityRules.GovulncheckMode = "off"
	cfg.Checks.SecurityRules.Secrets = secrets
	cfg.Cache.Enabled = boolPtr(false)
	config.ApplyDefaults(&cfg)
	ctx := context.Background()
	sc, err := runnersupport.NewContext(ctx, cfg, core.ScanOptions{Mode: core.ScanModeFull})
	if err != nil {
		t.Fatalf("new context: %v", err)
	}
	defer sc.Close()
	return codeguard.Report{Sections: runnerchecks.Build(ctx, sc)}
}

func secretsConfigFindingMessages(t *testing.T, report codeguard.Report) []string {
	t.Helper()
	messages := make([]string, 0)
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID != "security.secrets-config" {
				continue
			}
			if finding.Level != "fail" {
				t.Errorf("security.secrets-config level = %q, want fail", finding.Level)
			}
			messages = append(messages, finding.Message)
		}
	}
	sort.Strings(messages)
	return messages
}

func regexpError(t *testing.T, pattern string) string {
	t.Helper()
	_, err := regexp.Compile(pattern)
	if err == nil {
		t.Fatalf("pattern %q unexpectedly compiles", pattern)
	}
	return err.Error()
}
