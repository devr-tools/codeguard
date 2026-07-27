package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestDeliveryMissingRollbackStrategyAndPostDeployVerification(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".github", "workflows", "deploy.yml"), "name: deploy\njobs:\n  prod:\n    steps:\n      - run: kubectl apply -f deploy/app.yaml\n")

	cfg := deliveryTestConfig(dir, "delivery-missing-rollback")
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Delivery", "warn")
	assertFindingRulePresent(t, report, "Delivery", "delivery.missing-rollback-strategy")
	assertFindingRulePresent(t, report, "Delivery", "delivery.missing-post-deploy-verification")
}

func TestDeliveryUnsafeMigrationOrder(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "migrations", "001_drop_legacy_email.sql"), "ALTER TABLE users DROP COLUMN legacy_email;\n")

	report, err := codeguard.Run(context.Background(), deliveryTestConfig(dir, "delivery-unsafe-migration"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Delivery", "delivery.unsafe-migration-order")
}

func TestDeliveryHighRiskChangeWithoutKillSwitch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "payments", "charge.go"), "package payments\n\nfunc Charge(order Order) error {\n\treturn charge(order)\n}\n\ntype Order struct{}\n")

	report, err := codeguard.Run(context.Background(), deliveryTestConfig(dir, "delivery-no-kill-switch"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Delivery", "delivery.high-risk-change-without-kill-switch")
}

func TestDeliveryAllowsRollbackVerificationAndKillSwitchEvidence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".github", "workflows", "deploy.yml"), "name: deploy\njobs:\n  prod:\n    steps:\n      - run: kubectl apply -f deploy/app.yaml\n      - run: curl -fsS https://example.invalid/health\n      - run: echo rollback via kubectl rollout undo\n")
	writeFile(t, filepath.Join(dir, "src", "payments", "charge.go"), "package payments\n\nfunc Charge(order Order, flags Flags) error {\n\tif flags.Enabled(\"new_charge\") {\n\t\treturn charge(order)\n\t}\n\treturn nil\n}\n\ntype Order struct{}\ntype Flags interface { Enabled(string) bool }\n")

	report, err := codeguard.Run(context.Background(), deliveryTestConfig(dir, "delivery-evidence"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Delivery", "pass")
}

func deliveryTestConfig(dir string, name string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	off := false
	on := true
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.SupplyChain = false
	cfg.Checks.Context = &off
	cfg.Checks.Delivery = &on
	cfg.Cache.Enabled = &off
	return cfg
}
