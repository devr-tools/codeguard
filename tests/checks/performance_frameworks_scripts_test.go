package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// Framework-aware TypeScript/JavaScript rules
// (performance_rules.detect_framework_patterns): React render-cost and
// Express middleware tests, including the negative cases that prove the
// evidence gates and exemptions hold.

func TestPerformanceCheckWarnsForReactExpensiveRender(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "ItemList.tsx"),
		"import React from \"react\";\n\nexport function ItemList({ items }: Props) {\n  const rows = items.filter((i) => i.active).map((i) => render(i));\n  return <ul>{rows}</ul>;\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-ts-react-render", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.typescript.react-expensive-render")
}

func TestPerformanceCheckWarnsForReactJSONParseInComponentBody(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "Config.tsx"),
		"import React from \"react\";\n\nexport function ConfigPanel({ raw }: Props) {\n  const config = JSON.parse(raw);\n  return <pre>{config.name}</pre>;\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-ts-react-parse", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.typescript.react-expensive-render")
}

func TestPerformanceCheckSkipsReactRenderWorkInsideUseMemo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "ItemList.tsx"),
		"import React, { useMemo } from \"react\";\n\nexport function ItemList({ items }: Props) {\n  const rows = useMemo(\n    () => items.filter((i) => i.active).map((i) => render(i)),\n    [items],\n  );\n  return <ul>{rows}</ul>;\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-ts-react-memoized", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.typescript.react-expensive-render")
}

func TestPerformanceCheckSkipsArrayChainsWithoutReactEvidence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "transform.ts"),
		"export function BuildList(items: Item[]) {\n  return items.filter((i) => i.active).map((i) => render(i));\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-ts-no-react", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.typescript.react-expensive-render")
}

func TestPerformanceCheckWarnsForExpressSyncMiddleware(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "server.ts"),
		"import express from \"express\";\nimport bcrypt from \"bcrypt\";\n\nconst app = express();\n\napp.use((req, res, next) => {\n  req.hash = bcrypt.hashSync(req.body.password, 10);\n  next();\n});\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-ts-express-sync", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.typescript.express-sync-middleware")
	// Precedence: the specific middleware rule replaces the generic sync-io
	// finding on the same line, so the line never reports twice.
	assertFindingRuleAbsent(t, report, "Performance", "performance.typescript.sync-io-in-handler")
}

func TestPerformanceCheckKeepsGenericSyncIOForNonCPUHeavyMiddlewareCalls(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "server.ts"),
		"import express from \"express\";\nimport fs from \"fs\";\n\nconst app = express();\n\napp.use((req, res, next) => {\n  res.locals.config = fs.readFileSync(\"config.json\", \"utf8\");\n  next();\n});\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-ts-express-fs", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.typescript.express-sync-middleware")
	assertFindingRulePresent(t, report, "Performance", "performance.typescript.sync-io-in-handler")
}

func TestPerformanceCheckSkipsSyncMiddlewareWithoutExpressEvidence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "server.ts"),
		"import bcrypt from \"bcrypt\";\n\nconst app = createApp();\n\napp.use((req, res, next) => {\n  req.hash = bcrypt.hashSync(req.body.password, 10);\n  next();\n});\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-ts-no-express", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.typescript.express-sync-middleware")
	// Without express evidence the generic sync-io rule still applies.
	assertFindingRulePresent(t, report, "Performance", "performance.typescript.sync-io-in-handler")
}
