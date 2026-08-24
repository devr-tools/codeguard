package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestCPPToolchainDeadCodeIsOptIn(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main.cpp"), "#include \"live.cpp\"\nint main() { return live(); }\n")
	writeFile(t, filepath.Join(dir, "src", "live.cpp"), "int live() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "src", "orphan.cpp"), "int orphan() { return 2; }\n")

	report := runCPPToolchainDeadCodeScan(t, dir, false)

	assertFindingRuleAbsent(t, report, "Code Quality", goToolchainDeadCodeRuleID)
}

func TestCPPToolchainDeadCodeReportsSourcesOutsideEntrypointGraph(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main.cpp"), "#include \"live.cpp\"\nint main() { return live(); }\n")
	writeFile(t, filepath.Join(dir, "src", "live.cpp"), "int live() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "src", "orphan.cpp"), "int orphan() { return 2; }\n")

	report := runCPPToolchainDeadCodeScan(t, dir, true)

	assertToolchainDeadCodeFindingPath(t, report, "src/orphan.cpp")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "src/live.cpp")
}

func TestCPPToolchainDeadCodeReportsUnreachableNamedModules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main.cpp"), "import live;\nint main() { return live_value(); }\n")
	writeFile(t, filepath.Join(dir, "src", "live.cppm"), "export module live;\nexport int live_value() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "src", "orphan.cppm"), "export module orphan;\nexport int orphan_value() { return 2; }\n")

	report := runCPPToolchainDeadCodeScan(t, dir, true)

	assertToolchainDeadCodeFindingPath(t, report, "src/orphan.cppm")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "src/live.cppm")
}

func TestCPPToolchainDeadCodeReportsDiscardedLinkerSymbols(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main.cpp"), "int main() { return 0; }\n")
	writeFile(t, filepath.Join(dir, "src", "orphan.cpp"), "static int orphan_fn() { return 2; }\n")
	writeFile(t, filepath.Join(dir, "build", "app.map"), "ld.lld: removing unused section '.text._Z9orphan_fnv'\n")

	report := runCPPToolchainDeadCodeScanWithConfig(t, dir, true, func(cfg *codeguard.Config) {
		graph := false
		cfg.Checks.QualityRules.DeadCode.CPP.Graph = &graph
		cfg.Checks.QualityRules.DeadCode.CPP.Reports = []string{"build/app.map"}
	})

	assertToolchainDeadCodeFindingPath(t, report, "src/orphan.cpp")
}

func TestCPPToolchainDeadCodeReportsDiscardedLinkerObjects(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main.cpp"), "int main() { return 0; }\n")
	writeFile(t, filepath.Join(dir, "src", "orphan.cpp"), "int orphan_api() { return 2; }\n")
	writeFile(t, filepath.Join(dir, "build", "app.map"), "linker: discarded object src/orphan.o\n")

	report := runCPPToolchainDeadCodeScanWithConfig(t, dir, true, func(cfg *codeguard.Config) {
		graph := false
		cfg.Checks.QualityRules.DeadCode.CPP.Graph = &graph
		cfg.Checks.QualityRules.DeadCode.CPP.Reports = []string{"build/*.map"}
	})

	assertToolchainDeadCodeFindingPath(t, report, "src/orphan.cpp")
}

func TestCPPToolchainDeadCodeLiveSymbolReportSuppressesGraphOnlyFinding(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main.cpp"), "int main() { return 0; }\n")
	writeFile(t, filepath.Join(dir, "src", "library_api.cpp"), "int public_api() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "build", "nm.txt"), "0000000000001000 T _Z10public_apiv\n")

	report := runCPPToolchainDeadCodeScanWithConfig(t, dir, true, func(cfg *codeguard.Config) {
		cfg.Checks.QualityRules.DeadCode.CPP.Reports = []string{"build/nm.txt"}
	})

	assertToolchainDeadCodeFindingPathAbsent(t, report, "src/library_api.cpp")
}

func TestCPPToolchainDeadCodeMachODeadStripMapReportsDiscardedSymbol(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main.cpp"), "int main() { return 0; }\n")
	writeFile(t, filepath.Join(dir, "src", "dead.cpp"), "static int apple_dead() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "build", "app.map"), "<<dead>>	[  1] _Z10apple_deadv\n")

	report := runCPPToolchainDeadCodeScanWithConfig(t, dir, true, func(cfg *codeguard.Config) {
		graph := false
		cfg.Checks.QualityRules.DeadCode.CPP.Graph = &graph
		cfg.Checks.QualityRules.DeadCode.CPP.Reports = []string{"build/app.map"}
	})

	assertToolchainDeadCodeFindingPath(t, report, "src/dead.cpp")
}

func TestCPPToolchainDeadCodeDumpbinLiveSymbolSuppressesGraphOnlyFinding(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main.cpp"), "int main() { return 0; }\n")
	writeFile(t, filepath.Join(dir, "src", "library_api.cpp"), "int public_api() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "build", "dumpbin.txt"), "004 00000000 SECT3  notype ()    External     | ?public_api@@YAHXZ (int __cdecl public_api(void))\n")

	report := runCPPToolchainDeadCodeScanWithConfig(t, dir, true, func(cfg *codeguard.Config) {
		cfg.Checks.QualityRules.DeadCode.CPP.Reports = []string{"build/dumpbin.txt"}
	})

	assertToolchainDeadCodeFindingPathAbsent(t, report, "src/library_api.cpp")
}

func TestCPPToolchainDeadCodeIgnoresWeakDiscardNoise(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main.cpp"), "int main() { return 0; }\n")
	writeFile(t, filepath.Join(dir, "src", "weak.cpp"), "static int weak_hook() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "build", "app.map"), "ld.lld: removing unused weak section '.text._Z9weak_hookv'\n")

	report := runCPPToolchainDeadCodeScanWithConfig(t, dir, true, func(cfg *codeguard.Config) {
		graph := false
		cfg.Checks.QualityRules.DeadCode.CPP.Graph = &graph
		cfg.Checks.QualityRules.DeadCode.CPP.Reports = []string{"build/app.map"}
	})

	assertFindingRuleAbsent(t, report, "Code Quality", goToolchainDeadCodeRuleID)
}

func TestCPPToolchainDeadCodeAvoidsKnownCPPFalsePositives(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main.cpp"), "int main() { return 0; }\n")
	writeFile(t, filepath.Join(dir, "tests", "helper_test.cpp"), "int helper_only_for_tests() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "third_party", "vendor.cpp"), "int vendor_api() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "src", "generated.cpp"), "// Code generated by fixture. DO NOT EDIT.\nint generated_hook() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "src", "abi.cpp"), "extern \"C\" int callback_entrypoint() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "src", "plugin.cpp"), "REGISTER_PLUGIN(\"demo\", make_plugin);\nint make_plugin() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "src", "templated.cpp"), "template <typename T>\nT identity(T value) { return value; }\n")
	writeFile(t, filepath.Join(dir, "src", "virtuals.cpp"), "struct Base { virtual int run(); };\nint Base::run() { return 1; }\n")

	report := runCPPToolchainDeadCodeScan(t, dir, true)

	assertFindingRuleAbsent(t, report, "Code Quality", goToolchainDeadCodeRuleID)
}

func TestCPPToolchainDeadCodeCoversFalsePositiveCorpusAndReportsRealOrphan(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main.cpp"), "#include \"live.cpp\"\nint main() { return live(); }\n")
	writeFile(t, filepath.Join(dir, "src", "live.cpp"), "int live() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "src", "public_api.cpp"), "int exported_library_api() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "src", "callback.cpp"), "extern \"C\" int callback_entrypoint() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "src", "plugin.cpp"), "REGISTER_PLUGIN(\"demo\", make_plugin);\nint make_plugin() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "src", "template.cpp"), "template <typename T>\nT identity(T value) { return value; }\n")
	writeFile(t, filepath.Join(dir, "src", "virtuals.cpp"), "struct Base { virtual int run(); };\nint Base::run() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "generated", "schema.cpp"), "// Code generated by fixture. DO NOT EDIT.\nint generated_hook() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "third_party", "vendor.cpp"), "int vendor_api() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "tests", "helper_test.cpp"), "int helper_only_for_tests() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "src", "orphan.cpp"), "int orphan() { return 2; }\n")
	writeFile(t, filepath.Join(dir, "build", "nm.txt"), "0000000000001000 T _Z20exported_library_apiv\n")

	report := runCPPToolchainDeadCodeScanWithConfig(t, dir, true, func(cfg *codeguard.Config) {
		cfg.Checks.QualityRules.DeadCode.CPP.Reports = []string{"build/nm.txt"}
	})

	assertToolchainDeadCodeFindingPath(t, report, "src/orphan.cpp")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "src/live.cpp")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "src/public_api.cpp")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "src/callback.cpp")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "src/plugin.cpp")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "src/template.cpp")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "src/virtuals.cpp")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "generated/schema.cpp")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "third_party/vendor.cpp")
	assertToolchainDeadCodeFindingPathAbsent(t, report, "tests/helper_test.cpp")
}

func TestCPPToolchainDeadCodeDoesNotTreatStrippedArtifactNoiseAsDiscardProof(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main.cpp"), "int main() { return 0; }\n")
	writeFile(t, filepath.Join(dir, "src", "ambiguous.cpp"), "static int maybe_inlined_or_stripped() { return 1; }\n")
	writeFile(t, filepath.Join(dir, "build", "link.txt"), "strip: removed local symbols from src/ambiguous.o\n")

	report := runCPPToolchainDeadCodeScanWithConfig(t, dir, true, func(cfg *codeguard.Config) {
		graph := false
		cfg.Checks.QualityRules.DeadCode.CPP.Graph = &graph
		cfg.Checks.QualityRules.DeadCode.CPP.Reports = []string{"build/link.txt"}
	})

	assertFindingRuleAbsent(t, report, "Code Quality", goToolchainDeadCodeRuleID)
}

func runCPPToolchainDeadCodeScan(t *testing.T, dir string, enabled bool) codeguard.Report {
	t.Helper()
	cfg := cppToolchainDeadCodeConfig(dir, enabled)
	return runCPPToolchainDeadCodeScanConfig(t, cfg)
}

func runCPPToolchainDeadCodeScanWithConfig(t *testing.T, dir string, enabled bool, mutate func(*codeguard.Config)) codeguard.Report {
	t.Helper()
	cfg := cppToolchainDeadCodeConfig(dir, enabled)
	mutate(&cfg)
	return runCPPToolchainDeadCodeScanConfig(t, cfg)
}

func runCPPToolchainDeadCodeScanConfig(t *testing.T, cfg codeguard.Config) codeguard.Report {
	t.Helper()
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}

func cppToolchainDeadCodeConfig(dir string, enabled bool) codeguard.Config {
	disabled := false
	cfg := codeguard.ExampleConfig()
	cfg.Name = "cpp-toolchain-dead-code-test"
	cfg.Targets = []codeguard.TargetConfig{{
		Name:        "cpp",
		Path:        dir,
		Language:    "cpp",
		Entrypoints: []string{"src/main.cpp"},
	}}
	cfg.Checks.Quality = true
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
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
		graph := true
		cfg.Checks.QualityRules.DeadCode = codeguard.QualityDeadCodeConfig{
			Enabled: &enabled,
			Mode:    "toolchain",
			Level:   "warn",
			CPP: codeguard.CPPDeadCodeConfig{
				Entrypoints: []string{"src/main.cpp"},
				Graph:       &graph,
			},
		}
	}
	return cfg
}
