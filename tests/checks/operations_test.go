package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func operationsConfig(name string, dir string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.SupplyChain = false
	on := true
	off := false
	cfg.Checks.Reliability = &off
	cfg.Checks.Data = &off
	cfg.Checks.Change = &off
	cfg.Checks.Observability = &off
	cfg.Checks.Operations = &on
	cfg.Checks.Context = &off
	cfg.Cache.Enabled = &off
	return cfg
}

func TestOperationsMissingOwnerAndRunbook(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "internal", "payment", "service.go"), `package payment

func Charge() {}
`)

	report, err := codeguard.Run(context.Background(), operationsConfig("operations-missing", dir))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Operations", "operations.missing-owner")
	assertFindingRulePresent(t, report, "Operations", "operations.missing-runbook")
}

func TestOperationsAcceptsOwnerAndRunbookEvidence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "internal", "payment", "service.go"), `package payment

func Charge() {}
`)
	writeFile(t, filepath.Join(dir, "CODEOWNERS"), `/internal/payment/ @example/payments
`)
	writeFile(t, filepath.Join(dir, "docs", "runbooks", "payment.md"), `# Payment runbook
`)

	report, err := codeguard.Run(context.Background(), operationsConfig("operations-owned", dir))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Operations", "operations.missing-owner")
	assertFindingRuleAbsent(t, report, "Operations", "operations.missing-runbook")
}
