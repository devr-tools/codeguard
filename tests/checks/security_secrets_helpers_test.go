package checks_test

import (
	"context"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func boolPtr(v bool) *bool { return &v }

func secretsScanConfig(t *testing.T, dir string, secrets *codeguard.SecretsRulesConfig, language string) codeguard.Report {
	t.Helper()
	cfg := codeguard.ExampleConfig()
	cfg.Name = "security-secrets"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: language}}
	cfg.Checks.Security = true
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.Quality = false
	cfg.Checks.SupplyChain = false
	cfg.Checks.SecurityRules.GovulncheckMode = "off"
	cfg.Checks.SecurityRules.Secrets = secrets

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}
