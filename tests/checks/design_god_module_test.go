package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func writeGoHubFixture(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/hubrepo\n\ngo 1.23.0\n")
	writeFile(t, filepath.Join(dir, "hub", "hub.go"), "package hub\n\nfunc Value() int { return 1 }\n")
	writeFile(t, filepath.Join(dir, "alpha", "alpha.go"), "package alpha\n\nimport \"example.com/hubrepo/hub\"\n\nfunc Alpha() int { return hub.Value() }\n")
	writeFile(t, filepath.Join(dir, "beta", "beta.go"), "package beta\n\nimport \"example.com/hubrepo/hub\"\n\nfunc Beta() int { return hub.Value() }\n")
	writeFile(t, filepath.Join(dir, "gamma", "gamma.go"), "package gamma\n\nimport \"example.com/hubrepo/hub\"\n\nfunc Gamma() int { return hub.Value() }\n")
}

func TestDesignCheckWarnsForGoGodModule(t *testing.T) {
	dir := t.TempDir()
	writeGoHubFixture(t, dir)

	cfg := graphTestConfig("design-go-god-module", dir, "go")
	cfg.Checks.DesignRules.GodModuleThreshold = 2

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Design Patterns", "design.god-module")
}

func TestDesignCheckSkipsGodModuleBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	writeGoHubFixture(t, dir)

	report, err := codeguard.Run(context.Background(), graphTestConfig("design-go-god-module-neg", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Design Patterns", "design.god-module")
}

func TestDesignCheckWarnsForTypeScriptGodModule(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "core.ts"), "export const core = 1;\n")
	writeFile(t, filepath.Join(dir, "src", "one.ts"), "import { core } from \"./core\";\n\nexport const one = core;\n")
	writeFile(t, filepath.Join(dir, "src", "two.ts"), "import { core } from \"./core\";\n\nexport const two = core;\n")
	writeFile(t, filepath.Join(dir, "src", "three.ts"), "import { core } from \"./core\";\n\nexport const three = core;\n")

	cfg := graphTestConfig("design-ts-god-module", dir, "typescript")
	cfg.Checks.DesignRules.GodModuleThreshold = 2

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Design Patterns", "design.god-module")
}

func TestDesignCheckGodModuleToggleOff(t *testing.T) {
	dir := t.TempDir()
	writeGoHubFixture(t, dir)

	off := false
	cfg := graphTestConfig("design-go-god-module-off", dir, "go")
	cfg.Checks.DesignRules.GodModuleThreshold = 2
	cfg.Checks.DesignRules.DetectGodModules = &off

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Design Patterns", "design.god-module")
}
