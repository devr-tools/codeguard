package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestDesignBoundaryPathPatternsSupportRecursiveGlobCharacterClassAndLiteralDescendants(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "domain", "handler2.ts"), "import { store } from '../adapters/store';\nexport const run = store;\n")
	writeFile(t, filepath.Join(dir, "src", "adapters", "store.ts"), "export const store = 1;\n")

	cfg := graphTestConfig("design-path-patterns", dir, "typescript")
	cfg.Checks.DesignRules.Layers = []codeguard.DesignLayerConfig{
		{Name: "domain", Paths: []string{"src/**/handler[0-9].ts"}, DenyDependOn: []string{"adapters"}},
		{Name: "adapters", Paths: []string{"src/adapters"}},
	}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	finding := findFinding(t, report, "Design Patterns", "design.layer-boundary")
	if finding.Path != "src/domain/handler2.ts" || finding.Line != 1 {
		t.Fatalf("finding location = %s:%d, want src/domain/handler2.ts:1", finding.Path, finding.Line)
	}
}
