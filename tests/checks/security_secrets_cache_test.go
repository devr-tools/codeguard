package checks_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func cachedSecretsReport(t *testing.T, dir string, cachePath string) codeguard.Report {
	t.Helper()
	cfg := codeguard.ExampleConfig()
	cfg.Name = "security-secrets-cache"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Security = true
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.Quality = false
	cfg.Checks.SupplyChain = false
	cfg.Checks.SecurityRules.GovulncheckMode = "off"
	cfg.Checks.SecurityRules.Secrets = nil
	enabled := true
	cfg.Cache.Enabled = &enabled
	cfg.Cache.Path = cachePath
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}

// fixtureDiagnosticKeys flattens the security-secrets diagnostics into
// comparable keys (id, kind, path, evidence).
func fixtureDiagnosticKeys(report codeguard.Report) []string {
	keys := make([]string, 0)
	for _, section := range report.Sections {
		if section.Name != "Security" {
			continue
		}
		for _, diagnostic := range section.Diagnostics {
			if diagnostic.ID != "security.credential-fixture" {
				continue
			}
			parts := append([]string{diagnostic.ID, diagnostic.Kind, diagnostic.Level, diagnostic.Path, diagnostic.Message}, diagnostic.Evidence...)
			keys = append(keys, strings.Join(parts, "|"))
		}
	}
	sort.Strings(keys)
	return keys
}

// The secrets pass reports suppressed synthetic fixtures as diagnostics. A
// warm run served from the per-file findings cache must report exactly the
// diagnostics a cold run does; they used to be collected only as a side
// effect of evaluating a file, so a cache hit silently dropped them.
func TestSecretsDiagnosticsSurviveFindingsCache(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pkg", "client_test.go"),
		"package pkg\n\nvar key = \""+cred("AKIA", "0000000000000000")+"\"\n")
	writeFile(t, filepath.Join(dir, "pkg", "store_test.go"),
		"package pkg\n\nvar dsn = \"postgres://app:p4ssw0rdish@127.0.0.1:5432/app\"\n")
	writeFile(t, filepath.Join(dir, "docs", "setup.md"),
		"Use key = \""+cred("AKIAIOSFODNN7", "EXAMPLE")+"\" in examples.\n")
	cachePath := filepath.Join(t.TempDir(), "cache.json")

	cold := fixtureDiagnosticKeys(cachedSecretsReport(t, dir, cachePath))
	if len(cold) != 3 {
		t.Fatalf("cold diagnostics = %v, want 3 credential-fixture entries", cold)
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("cold run did not write the findings cache: %v", err)
	}
	warm := fixtureDiagnosticKeys(cachedSecretsReport(t, dir, cachePath))
	if !reflect.DeepEqual(cold, warm) {
		t.Fatalf("warm diagnostics differ from cold\ncold: %v\nwarm: %v", cold, warm)
	}

	// Editing one file re-evaluates only that file; the others replay from
	// the cache with their diagnostics intact.
	writeFile(t, filepath.Join(dir, "pkg", "store_test.go"), "package pkg\n\nvar dsn = \"\"\n")
	partial := fixtureDiagnosticKeys(cachedSecretsReport(t, dir, cachePath))
	if len(partial) != 2 {
		t.Fatalf("partially warm diagnostics = %v, want the 2 unchanged entries", partial)
	}
}
