package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestDesignCheckFailsForTypeScriptImportCycle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "alpha.ts"), "import { beta } from \"./beta\";\n\nexport const alpha = () => beta();\n")
	writeFile(t, filepath.Join(dir, "src", "beta.ts"), "import { alpha } from \"./alpha\";\n\nexport const beta = () => alpha();\n")

	report, err := codeguard.Run(context.Background(), graphTestConfig("design-ts-cycle", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "fail")
	assertFindingRulePresent(t, report, "Design Patterns", "design.typescript.import-cycle")
}

func TestDesignCheckPassesForAcyclicTypeScriptImports(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "alpha.ts"), "import { beta } from \"./beta\";\n\nexport const alpha = () => beta();\n")
	writeFile(t, filepath.Join(dir, "src", "beta.ts"), "export const beta = () => 1;\n")

	report, err := codeguard.Run(context.Background(), graphTestConfig("design-ts-no-cycle", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Design Patterns", "design.typescript.import-cycle")
}

func TestDesignCheckFailsForTypeScriptImportCycleThroughTSConfigPaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tsconfig.json"), "{\n  // comment to exercise JSONC parsing\n  \"compilerOptions\": {\n    \"baseUrl\": \".\",\n    \"paths\": {\n      \"@app/*\": [\"src/*\",],\n    },\n  },\n}\n")
	writeFile(t, filepath.Join(dir, "src", "alpha.ts"), "import { beta } from \"@app/beta\";\n\nexport const alpha = () => beta();\n")
	writeFile(t, filepath.Join(dir, "src", "beta.ts"), "import { alpha } from \"@app/alpha\";\n\nexport const beta = () => alpha();\n")

	report, err := codeguard.Run(context.Background(), graphTestConfig("design-ts-paths-cycle", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "fail")
	assertFindingRulePresent(t, report, "Design Patterns", "design.typescript.import-cycle")
}

func TestDesignCheckFailsForTypeScriptImportCycleThroughExtendedTSConfigPaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tsconfig.base.json"), "{\n  \"compilerOptions\": {\n    \"baseUrl\": \".\",\n    \"paths\": {\n      \"@app/*\": [\"app/src/*\"]\n    }\n  }\n}\n")
	writeFile(t, filepath.Join(dir, "app", "tsconfig.json"), "{\n  \"extends\": \"../tsconfig.base.json\"\n}\n")
	writeFile(t, filepath.Join(dir, "app", "src", "alpha.ts"), "import { beta } from \"@app/beta\";\n\nexport const alpha = () => beta();\n")
	writeFile(t, filepath.Join(dir, "app", "src", "beta.ts"), "import { alpha } from \"@app/alpha\";\n\nexport const beta = () => alpha();\n")

	report, err := codeguard.Run(context.Background(), graphTestConfig("design-ts-extended-paths-cycle", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "fail")
	assertFindingRulePresent(t, report, "Design Patterns", "design.typescript.import-cycle")
}

func TestDesignCheckFailsForTypeScriptImportCycleThroughPackageExtendedTSConfigPaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages", "tsconfig", "package.json"), "{\n  \"name\": \"@repo/tsconfig\"\n}\n")
	writeFile(t, filepath.Join(dir, "packages", "tsconfig", "base.json"), "{\n  \"compilerOptions\": {\n    \"baseUrl\": \"../..\",\n    \"paths\": {\n      \"@app/*\": [\"app/src/*\"]\n    }\n  }\n}\n")
	writeFile(t, filepath.Join(dir, "app", "tsconfig.json"), "{\n  \"extends\": \"@repo/tsconfig/base.json\"\n}\n")
	writeFile(t, filepath.Join(dir, "app", "src", "alpha.ts"), "import { beta } from \"@app/beta\";\n\nexport const alpha = () => beta();\n")
	writeFile(t, filepath.Join(dir, "app", "src", "beta.ts"), "import { alpha } from \"@app/alpha\";\n\nexport const beta = () => alpha();\n")

	report, err := codeguard.Run(context.Background(), graphTestConfig("design-ts-package-extended-paths-cycle", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "fail")
	assertFindingRulePresent(t, report, "Design Patterns", "design.typescript.import-cycle")
}

func TestDesignCheckFailsForTypeScriptWorkspacePackageCycle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages", "alpha", "package.json"), "{\n  \"name\": \"@repo/alpha\",\n  \"main\": \"./src/index.ts\"\n}\n")
	writeFile(t, filepath.Join(dir, "packages", "beta", "package.json"), "{\n  \"name\": \"@repo/beta\",\n  \"exports\": \"./src/index.ts\"\n}\n")
	writeFile(t, filepath.Join(dir, "packages", "alpha", "src", "index.ts"), "import { beta } from \"@repo/beta\";\n\nexport const alpha = () => beta();\n")
	writeFile(t, filepath.Join(dir, "packages", "beta", "src", "index.ts"), "import { alpha } from \"@repo/alpha\";\n\nexport const beta = () => alpha();\n")

	report, err := codeguard.Run(context.Background(), graphTestConfig("design-ts-workspace-cycle", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "fail")
	assertFindingRulePresent(t, report, "Design Patterns", "design.typescript.import-cycle")
}

func TestDesignCheckPrefersSourceExportConditionsForWorkspacePackages(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages", "alpha", "package.json"), "{\n  \"name\": \"@repo/alpha\",\n  \"exports\": {\n    \".\": {\n      \"default\": \"./dist/index.js\",\n      \"import\": \"./src/index.ts\"\n    }\n  }\n}\n")
	writeFile(t, filepath.Join(dir, "packages", "beta", "package.json"), "{\n  \"name\": \"@repo/beta\",\n  \"main\": \"./src/index.ts\"\n}\n")
	writeFile(t, filepath.Join(dir, "packages", "alpha", "src", "index.ts"), "import { beta } from \"@repo/beta\";\n\nexport const alpha = () => beta();\n")
	writeFile(t, filepath.Join(dir, "packages", "beta", "src", "index.ts"), "import { alpha } from \"@repo/alpha\";\n\nexport const beta = () => alpha();\n")

	report, err := codeguard.Run(context.Background(), graphTestConfig("design-ts-conditional-exports-cycle", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "fail")
	assertFindingRulePresent(t, report, "Design Patterns", "design.typescript.import-cycle")
}

func TestDesignCheckFailsForTypeScriptImportCycleThroughPackageImports(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), "{\n  \"name\": \"app\",\n  \"imports\": {\n    \"#core/*\": \"./src/*\"\n  }\n}\n")
	writeFile(t, filepath.Join(dir, "src", "alpha.ts"), "import { beta } from \"#core/beta\";\n\nexport const alpha = () => beta();\n")
	writeFile(t, filepath.Join(dir, "src", "beta.ts"), "import { alpha } from \"#core/alpha\";\n\nexport const beta = () => alpha();\n")

	report, err := codeguard.Run(context.Background(), graphTestConfig("design-ts-package-imports-cycle", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "fail")
	assertFindingRulePresent(t, report, "Design Patterns", "design.typescript.import-cycle")
}
