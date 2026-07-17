package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestDesignCheckFailsWhenPythonPublicModuleImportsPrivateModule(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "service.py"), "from . import _internal\n\nrun = _internal.run\n")
	writeFile(t, filepath.Join(dir, "app", "_internal.py"), "def run():\n    return 'ok'\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-python-private-import"
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
	assertFindingRulePresent(t, report, "Design Patterns", "design.python.public-imports-private")
}

func TestDesignCheckFailsWhenPythonPublicModuleImportsCLI(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "service.py"), "from app import cli\n\nrun = cli.run\n")
	writeFile(t, filepath.Join(dir, "app", "cli.py"), "def run():\n    return 'ok'\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-python-cli-import"
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

func TestDesignCheckWarnsForGenericPythonModuleName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "utils.py"), "VALUE = 1\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-python-generic-module"
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

	assertSectionStatus(t, report, "Design Patterns", "warn")
	assertFindingRulePresent(t, report, "Design Patterns", "design.python.generic-module-name")
}

func TestDesignCheckPassesForLayeredPythonLayout(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "service.py"), "def run():\n    return 'ok'\n")
	writeFile(t, filepath.Join(dir, "app", "cli.py"), "from app.service import run\n\nif __name__ == '__main__':\n    run()\n")
	writeFile(t, filepath.Join(dir, "tests", "test_service.py"), "from app import cli\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-python-pass"
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

	assertSectionStatus(t, report, "Design Patterns", "pass")
}

func TestDesignCheckWarnsForPythonClassWithTooManyMethods(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "service.py"), "class Service:\n    def __init__(self):\n        self.ready = True\n\n    def a(self):\n        return 1\n\n    @property\n    def b(self):\n        return 2\n\n    async def c(self):\n        return 3\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-python-max-methods"
	cfg.Targets = []codeguard.TargetConfig{{Name: "api", Path: dir, Language: "python"}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.DesignRules.MaxMethodsPerType = 2

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "warn")
	assertFindingRulePresent(t, report, "Design Patterns", "design.python.max-methods-per-type")
}

func TestDesignCheckWarnsForLargePythonProtocol(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "ports.py"), "from typing import Protocol\n\nclass Store(\n    Protocol,\n):\n    name: str\n    enabled: bool\n\n    def get(self) -> str:\n        ...\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-python-max-protocol"
	cfg.Targets = []codeguard.TargetConfig{{Name: "api", Path: dir, Language: "python"}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.DesignRules.MaxInterfaceMethods = 2

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "warn")
	assertFindingRulePresent(t, report, "Design Patterns", "design.python.max-protocol-members")
}

func TestDesignCheckIgnoresNestedPythonHelpersWhenCountingClassMethods(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "service.py"), "class Service:\n    def a(self):\n        def helper():\n            return 1\n        return helper()\n\n    class Nested:\n        def hidden(self):\n            return 1\n\n    def b(self):\n        return 2\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-python-nested-methods"
	cfg.Targets = []codeguard.TargetConfig{{Name: "api", Path: dir, Language: "python"}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.DesignRules.MaxMethodsPerType = 2

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "pass")
}
