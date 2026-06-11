package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestDesignCheckFailsWhenPythonPublicModuleTransitivelyDependsOnCLI(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "service.py"), "from app.web import handler\n\nrun = handler.handle\n")
	writeFile(t, filepath.Join(dir, "app", "web", "handler.py"), "from app import cli\n\nhandle = cli.run\n")
	writeFile(t, filepath.Join(dir, "app", "cli.py"), "def run():\n    return 'ok'\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-python-transitive-cli"
	cfg.Targets = []codeguard.TargetConfig{{
		Name:        "api",
		Path:        dir,
		Language:    "python",
		Entrypoints: []string{"app/cli.py"},
	}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "fail")
	assertFindingRulePresent(t, report, "Design Patterns", "design.python.public-depends-on-cli")
}

func TestDesignCheckFailsForPythonImportCycle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "service.py"), "from app.repo import store\n\nrun = store\n")
	writeFile(t, filepath.Join(dir, "app", "repo.py"), "from app.service import run\n\nstore = run\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-python-import-cycle"
	cfg.Targets = []codeguard.TargetConfig{{Name: "api", Path: dir, Language: "python"}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "fail")
	assertFindingRulePresent(t, report, "Design Patterns", "design.python.import-cycle")
}

func TestDesignCheckHandlesMultilinePythonImports(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "service.py"), "from app import (\n    cli,\n)\n\nrun = cli.run\n")
	writeFile(t, filepath.Join(dir, "app", "cli.py"), "def run():\n    return 'ok'\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-python-multiline-import"
	cfg.Targets = []codeguard.TargetConfig{{
		Name:        "api",
		Path:        dir,
		Language:    "python",
		Entrypoints: []string{"app/cli.py"},
	}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "fail")
	assertFindingRulePresent(t, report, "Design Patterns", "design.python.public-imports-cli")
}
