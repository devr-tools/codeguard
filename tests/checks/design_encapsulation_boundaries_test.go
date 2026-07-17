package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestDesignPublicSurfaceRejectsTypeScriptDeepImport(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "app.ts"), "// application entrypoint\nimport { parseToken } from './auth/internal/token';\n")
	writeFile(t, filepath.Join(dir, "src", "auth", "index.ts"), "export { parseToken } from './internal/token';\n")
	writeFile(t, filepath.Join(dir, "src", "auth", "internal", "token.ts"), "export const parseToken = () => 'token';\n")

	cfg := graphTestConfig("design-ts-public-surface", dir, "typescript")
	cfg.Checks.DesignRules.PublicSurfaces = []codeguard.DesignPublicSurfaceConfig{{
		Name:        "auth",
		Paths:       []string{"src/auth/**"},
		Entrypoints: []string{"src/auth/index.ts"},
	}}
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	finding := findFinding(t, report, "Design Patterns", "design.private-module-import")
	if finding.Path != "src/app.ts" || finding.Line != 2 {
		t.Fatalf("finding location = %s:%d, want src/app.ts:2", finding.Path, finding.Line)
	}
}

func TestDesignPublicSurfaceAllowsEntrypointAndInternalImports(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "app.ts"), "import { parseToken } from './auth';\n")
	writeFile(t, filepath.Join(dir, "src", "auth", "index.ts"), "export { parseToken } from './internal/token';\n")
	writeFile(t, filepath.Join(dir, "src", "auth", "internal", "token.ts"), "export const parseToken = () => 'token';\n")

	cfg := graphTestConfig("design-ts-public-entrypoint", dir, "typescript")
	cfg.Checks.DesignRules.PublicSurfaces = []codeguard.DesignPublicSurfaceConfig{{
		Name:        "auth",
		Paths:       []string{"src/auth/**"},
		Entrypoints: []string{"src/auth/index.ts"},
	}}
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Design Patterns", "design.private-module-import")
}

func TestDesignProductionCodeRejectsTypeScriptTestImport(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "app.ts"), "// runtime module\nimport { fakeClock } from '../test/helpers/clock';\n")
	writeFile(t, filepath.Join(dir, "test", "helpers", "clock.ts"), "export const fakeClock = () => 0;\n")

	cfg := graphTestConfig("design-ts-production-test", dir, "typescript")
	cfg.Checks.DesignRules.ProductionTest = &codeguard.DesignProductionTestConfig{
		ProductionPaths: []string{"src/**"},
		TestPaths:       []string{"test/**"},
	}
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	finding := findFinding(t, report, "Design Patterns", "design.production-imports-test")
	if finding.Path != "src/app.ts" || finding.Line != 2 {
		t.Fatalf("finding location = %s:%d, want src/app.ts:2", finding.Path, finding.Line)
	}
}

func TestDesignProductionTestPolicyCanBeDisabled(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "app.ts"), "import { fakeClock } from '../test/helpers/clock';\n")
	writeFile(t, filepath.Join(dir, "test", "helpers", "clock.ts"), "export const fakeClock = () => 0;\n")
	off := false

	cfg := graphTestConfig("design-ts-production-test-disabled", dir, "typescript")
	cfg.Checks.DesignRules.ProductionTest = &codeguard.DesignProductionTestConfig{
		Enabled:         &off,
		ProductionPaths: []string{"src/**"},
		TestPaths:       []string{"test/**"},
	}
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Design Patterns", "design.production-imports-test")
}

func TestDesignEncapsulationPoliciesUseGoImportSourceLocation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "internal", "service", "service.go"), "package service\n\nimport \"example.com/app/internal/testsupport\"\n")
	writeFile(t, filepath.Join(dir, "internal", "testsupport", "fixture.go"), "package testsupport\n")

	cfg := graphTestConfig("design-go-production-test", dir, "go")
	cfg.Checks.DesignRules.ProductionTest = &codeguard.DesignProductionTestConfig{
		ProductionPaths: []string{"internal/service/**"},
		TestPaths:       []string{"internal/testsupport/**"},
	}
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	finding := findFinding(t, report, "Design Patterns", "design.production-imports-test")
	if finding.Path != "internal/service/service.go" || finding.Line != 3 {
		t.Fatalf("finding location = %s:%d, want internal/service/service.go:3", finding.Path, finding.Line)
	}
}
