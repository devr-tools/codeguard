package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

const scriptToolchainDeadCodeRuleID = "quality.dead-code.toolchain"

func TestScriptToolchainDeadCodeIsOptIn(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "index.ts"), `export function run(): number {
  return 1;
}

function orphanHelper(): number {
  return 2;
}
`)

	report := runScriptToolchainDeadCodeScan(t, dir, "typescript", false)

	assertFindingRuleAbsent(t, report, "Code Quality", scriptToolchainDeadCodeRuleID)
}

func TestScriptToolchainDeadCodeReportsUnusedTypeScriptDeclarations(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "index.ts"), `export function run(): number {
  return usedHelper();
}

function usedHelper(): number {
  return 1;
}

function orphanHelper(): number {
  return 2;
}
`)

	report := runScriptToolchainDeadCodeScan(t, dir, "typescript", true)

	assertToolchainDeadCodePath(t, report, "src/index.ts")
}

func TestScriptToolchainDeadCodeReportsUnreachableJavaScriptStatements(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "index.js"), `export function run() {
  return 1;
  console.log("never reached");
}
`)

	report := runScriptToolchainDeadCodeScan(t, dir, "javascript", true)

	assertToolchainDeadCodePath(t, report, "src/index.js")
}

func TestScriptToolchainDeadCodeUsesConfiguredTypeScriptProject(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tsconfig.json"), `{"compilerOptions":{"allowJs":true,"checkJs":true,"noEmit":true},"include":["src/**/*.ts"]}`)
	writeFile(t, filepath.Join(dir, "src", "index.ts"), `export function root(): number {
  return 1;
}
`)
	writeFile(t, filepath.Join(dir, "packages", "app", "tsconfig.json"), `{"compilerOptions":{"allowJs":true,"checkJs":true,"noEmit":true},"include":["src/**/*.ts"]}`)
	writeFile(t, filepath.Join(dir, "packages", "app", "src", "app.ts"), `export function run(): number {
  return 1;
}

function orphanInConfiguredProject(): number {
  return 2;
}
`)

	report := runScriptToolchainDeadCodeScanWithConfig(t, dir, "typescript", true, func(cfg *codeguard.Config) {
		cfg.Checks.QualityRules.DeadCode.TypeScript.Projects = []string{"packages/app/tsconfig.json"}
	})

	assertToolchainDeadCodePath(t, report, "packages/app/src/app.ts")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "src/index.ts")
}

func TestScriptToolchainDeadCodeReportsEsbuildMetafileTreeShakenModule(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tsconfig.json"), `{"compilerOptions":{"allowJs":true,"checkJs":true,"noEmit":true},"include":["src/**/*.ts"]}`)
	writeFile(t, filepath.Join(dir, "src", "index.ts"), `export function run(): number {
  return 1;
}
`)
	writeFile(t, filepath.Join(dir, "src", "shaken.ts"), `export function treeShaken(): number {
  return 2;
}
`)
	writeFile(t, filepath.Join(dir, "meta.json"), `{
  "inputs": {
    "src/index.ts": {"bytes": 42},
    "src/shaken.ts": {"bytes": 50}
  },
  "outputs": {
    "dist/app.js": {
      "entryPoint": "src/index.ts",
      "inputs": {
        "src/index.ts": {"bytesInOutput": 20},
        "src/shaken.ts": {"bytesInOutput": 0}
      }
    }
  }
}`)

	report := runScriptToolchainDeadCodeScanWithConfig(t, dir, "typescript", true, func(cfg *codeguard.Config) {
		cfg.Checks.QualityRules.DeadCode.TypeScript.Reports = []string{"meta.json"}
	})

	assertToolchainDeadCodePathMessageContains(t, report, "src/shaken.ts", "bundler metafile")
}

func TestScriptToolchainDeadCodeReportsWebpackOrphanJavaScriptModule(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "jsconfig.json"), `{"compilerOptions":{"allowJs":true,"checkJs":true,"noEmit":true},"include":["src/**/*.js"]}`)
	writeFile(t, filepath.Join(dir, "src", "index.js"), `export function run() {
  return 1;
}
`)
	writeFile(t, filepath.Join(dir, "src", "orphan.js"), `export function orphaned() {
  return 2;
}
`)
	writeFile(t, filepath.Join(dir, "webpack-stats.json"), `{
  "modules": [
    {"name": "./src/index.js", "orphan": false},
    {"name": "./src/orphan.js", "orphan": true}
  ]
}`)

	report := runScriptToolchainDeadCodeScanWithConfig(t, dir, "javascript", true, func(cfg *codeguard.Config) {
		cfg.Checks.QualityRules.DeadCode.JavaScript.Reports = []string{"webpack-stats.json"}
	})

	assertToolchainDeadCodePathMessageContains(t, report, "src/orphan.js", "webpack stats")
}

func TestScriptToolchainDeadCodeReportsRollupViteZeroRenderedModule(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tsconfig.json"), `{"compilerOptions":{"allowJs":true,"checkJs":true,"noEmit":true},"include":["src/**/*.ts"]}`)
	writeFile(t, filepath.Join(dir, "src", "index.ts"), `export function run(): number {
  return 1;
}
`)
	writeFile(t, filepath.Join(dir, "src", "rolled.ts"), `export function rolledAway(): number {
  return 2;
}
`)
	writeFile(t, filepath.Join(dir, "rollup-output.json"), `{
  "output": [
    {
      "type": "chunk",
      "fileName": "assets/index.js",
      "isEntry": true,
      "modules": {
        "src/index.ts": {"renderedLength": 24},
        "src/rolled.ts": {"renderedLength": 0, "removedExports": ["rolledAway"]}
      }
    }
  ]
}`)

	report := runScriptToolchainDeadCodeScanWithConfig(t, dir, "typescript", true, func(cfg *codeguard.Config) {
		cfg.Checks.QualityRules.DeadCode.TypeScript.Reports = []string{"rollup-output.json"}
	})

	assertToolchainDeadCodePathMessageContains(t, report, "src/rolled.ts", "Rollup/Vite artifact")
}

func TestScriptToolchainDeadCodeDoesNotInferDeadCodeFromViteManifest(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tsconfig.json"), `{"compilerOptions":{"allowJs":true,"checkJs":true,"noEmit":true},"include":["src/**/*.ts"]}`)
	writeFile(t, filepath.Join(dir, "src", "entry.ts"), `export function run(): number {
  return 1;
}
`)
	writeFile(t, filepath.Join(dir, "src", "manifest_only.ts"), `export function maybeLoadedElsewhere(): number {
  return 2;
}
`)
	writeFile(t, filepath.Join(dir, "manifest.json"), `{
  "src/entry.ts": {
    "file": "assets/entry.js",
    "src": "src/entry.ts",
    "isEntry": true
  }
}`)

	report := runScriptToolchainDeadCodeScanWithConfig(t, dir, "typescript", true, func(cfg *codeguard.Config) {
		cfg.Checks.QualityRules.DeadCode.TypeScript.Reports = []string{"manifest.json"}
	})

	assertToolchainDeadCodeFindingPathAbsent(t, report, "src/manifest_only.ts")
}

func TestScriptToolchainDeadCodeAvoidsKnownScriptFalsePositives(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "public.ts"), `export function publicApi(): number {
  return 1;
}

export const publicValue = 2;
`)
	writeFile(t, filepath.Join(dir, "src", "used.ts"), `export function run(): number {
  return helper();
}

function helper(): number {
  return 1;
}
`)
	writeFile(t, filepath.Join(dir, "src", "widget.test.ts"), `function testOnlyHelper(): number {
  return 1;
}
`)
	writeFile(t, filepath.Join(dir, "tests", "helper.ts"), `function topLevelTestHelper(): number {
  return 1;
}
`)
	writeFile(t, filepath.Join(dir, "__tests__", "helper.ts"), `function topLevelJestHelper(): number {
  return 1;
}
`)
	writeFile(t, filepath.Join(dir, "src", "widget.stories.tsx"), `function StoryOnlyHelper(): null {
  return null;
}
`)
	writeFile(t, filepath.Join(dir, "src", "__generated__", "schema.ts"), `// Code generated by test fixture. DO NOT EDIT.
function generatedHelper(): number {
  return 1;
}
`)
	writeFile(t, filepath.Join(dir, "src", "ignored", "legacy.ts"), `function ignoredHelper(): number {
  return 1;
}
`)
	writeFile(t, filepath.Join(dir, "package.json"), `{"main":"./src/public.ts"}`)
	writeFile(t, filepath.Join(dir, "src", "register.ts"), `setup();

function setup(): void {
}
`)
	writeFile(t, filepath.Join(dir, "meta.json"), `{
  "inputs": {
    "src/public.ts": {"bytes": 42},
    "src/register.ts": {"bytes": 42}
  },
  "outputs": {
    "dist/app.js": {
      "inputs": {
        "src/public.ts": {"bytesInOutput": 0},
        "src/register.ts": {"bytesInOutput": 0}
      }
    }
  }
}`)

	report := runScriptToolchainDeadCodeScanWithConfig(t, dir, "typescript", true, func(cfg *codeguard.Config) {
		cfg.Checks.QualityRules.DeadCode.TypeScript.IgnorePaths = []string{"src/ignored/**"}
		cfg.Checks.QualityRules.DeadCode.TypeScript.Reports = []string{"meta.json"}
	})

	assertFindingRuleAbsent(t, report, "Code Quality", scriptToolchainDeadCodeRuleID)
}

func TestScriptToolchainDeadCodeCoversBundlerFalsePositiveCorpusAndReportsRealDeadModule(t *testing.T) {
	requireTypeScriptSemanticRuntime(t)

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tsconfig.json"), `{"compilerOptions":{"allowJs":true,"checkJs":true,"noEmit":true,"jsx":"preserve"},"include":["src/**/*.ts","src/**/*.tsx","app/**/*.tsx"]}`)
	writeFile(t, filepath.Join(dir, "package.json"), `{
  "main": "./src/index.ts",
  "exports": {
    ".": "./src/index.ts",
    "./feature": "./src/feature.ts"
  },
  "sideEffects": ["./src/polyfill.ts"]
}`)
	writeFile(t, filepath.Join(dir, "src", "index.ts"), `export function run(): number {
  return 1;
}
`)
	writeFile(t, filepath.Join(dir, "src", "feature.ts"), `export function publicFeature(): number {
  return 2;
}
`)
	writeFile(t, filepath.Join(dir, "app", "dashboard", "page.tsx"), `export default function Page() {
  return null;
}
`)
	writeFile(t, filepath.Join(dir, "src", "polyfill.ts"), `import "./install";

export function configure(): void {}
`)
	writeFile(t, filepath.Join(dir, "src", "install.ts"), `globalThis.__fixture = true;
`)
	writeFile(t, filepath.Join(dir, "src", "widget.spec.ts"), `function helperOnlyForSpecs(): number {
  return 1;
}
`)
	writeFile(t, filepath.Join(dir, "src", "widget.stories.tsx"), `export function Primary() {
  return null;
}
`)
	writeFile(t, filepath.Join(dir, "src", "__generated__", "schema.ts"), `// Code generated by test fixture. DO NOT EDIT.
export function generatedHelper(): number {
  return 1;
}
`)
	writeFile(t, filepath.Join(dir, "vendor", "legacy.ts"), `export function vendorHelper(): number {
  return 1;
}
`)
	writeFile(t, filepath.Join(dir, "src", "dead.ts"), `export function removedByBundler(): number {
  return 3;
}
`)
	writeFile(t, filepath.Join(dir, "meta.json"), `{
  "inputs": {
    "src/index.ts": {"bytes": 42},
    "src/feature.ts": {"bytes": 42},
    "app/dashboard/page.tsx": {"bytes": 42},
    "src/polyfill.ts": {"bytes": 42},
    "src/widget.spec.ts": {"bytes": 42},
    "src/widget.stories.tsx": {"bytes": 42},
    "src/__generated__/schema.ts": {"bytes": 42},
    "vendor/legacy.ts": {"bytes": 42},
    "src/dead.ts": {"bytes": 42}
  },
  "outputs": {
    "dist/app.js": {
      "inputs": {
        "src/index.ts": {"bytesInOutput": 0},
        "src/feature.ts": {"bytesInOutput": 0},
        "app/dashboard/page.tsx": {"bytesInOutput": 0},
        "src/polyfill.ts": {"bytesInOutput": 0},
        "src/widget.spec.ts": {"bytesInOutput": 0},
        "src/widget.stories.tsx": {"bytesInOutput": 0},
        "src/__generated__/schema.ts": {"bytesInOutput": 0},
        "vendor/legacy.ts": {"bytesInOutput": 0},
        "src/dead.ts": {"bytesInOutput": 0}
      }
    }
  }
}`)

	report := runScriptToolchainDeadCodeScanWithConfig(t, dir, "typescript", true, func(cfg *codeguard.Config) {
		cfg.Checks.QualityRules.DeadCode.TypeScript.Reports = []string{"meta.json"}
		cfg.Checks.QualityRules.DeadCode.TypeScript.Entrypoints = []string{"src/index.ts"}
	})

	assertToolchainDeadCodePathMessageContains(t, report, "src/dead.ts", "bundler metafile")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "src/index.ts")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "src/feature.ts")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "app/dashboard/page.tsx")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "src/polyfill.ts")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "src/widget.spec.ts")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "src/widget.stories.tsx")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "src/__generated__/schema.ts")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "vendor/legacy.ts")
}

func runScriptToolchainDeadCodeScan(t *testing.T, dir string, language string, enabled bool) codeguard.Report {
	t.Helper()
	return runScriptToolchainDeadCodeScanWithConfig(t, dir, language, enabled, nil)
}

func runScriptToolchainDeadCodeScanWithConfig(t *testing.T, dir string, language string, enabled bool, mutate func(*codeguard.Config)) codeguard.Report {
	t.Helper()
	cfg := codeguard.ExampleConfig()
	cfg.Name = "script-toolchain-dead-code-test"
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: language}}
	cfg.Checks.Quality = true
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	disabled := false
	cfg.Checks.Performance = &disabled
	cfg.Checks.SupplyChain = false
	cfg.Checks.Delivery = &disabled
	cfg.Checks.Reliability = &disabled
	cfg.Checks.Data = &disabled
	cfg.Checks.Observability = &disabled
	cfg.Checks.Operations = &disabled
	cfg.Checks.Change = &disabled
	cfg.Checks.Contracts = &disabled
	cfg.Checks.Context = &disabled
	cfg.Checks.QualityRules.AIChecks.DeadCode = &disabled
	if enabled {
		enabled := true
		cfg.Checks.QualityRules.DeadCode = codeguard.QualityDeadCodeConfig{
			Enabled: &enabled,
			Mode:    "toolchain",
			Level:   "warn",
		}
	}
	if mutate != nil {
		mutate(&cfg)
	}
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}

func assertToolchainDeadCodePath(t *testing.T, report codeguard.Report, wantPath string) {
	t.Helper()
	for _, finding := range toolchainDeadCodeFindings(report) {
		if filepath.ToSlash(finding.Path) == wantPath {
			return
		}
	}
	t.Fatalf("missing %s finding at %q; got %s", scriptToolchainDeadCodeRuleID, wantPath, formatFindings(report))
}

func assertToolchainDeadCodePathMessageContains(t *testing.T, report codeguard.Report, wantPath string, wantMessage string) {
	t.Helper()
	for _, finding := range toolchainDeadCodeFindings(report) {
		if filepath.ToSlash(finding.Path) == wantPath && strings.Contains(finding.Message, wantMessage) {
			return
		}
	}
	t.Fatalf("missing %s finding at %q containing %q; got %s", scriptToolchainDeadCodeRuleID, wantPath, wantMessage, formatFindings(report))
}
