package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestDesignCheckFailsForCPPIncludeCycle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "include", "alpha.h"), "#pragma once\n#include \"beta.h\"\nstruct Alpha {};\n")
	writeFile(t, filepath.Join(dir, "include", "beta.h"), "#pragma once\n#include \"alpha.h\"\nstruct Beta {};\n")

	report, err := codeguard.Run(context.Background(), graphTestConfig("design-cpp-cycle", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Design Patterns", "design.cpp.import-cycle")
}

func TestDesignCheckFailsForCPPNamedModuleCycle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "alpha.cppm"), "export module alpha;\nimport beta;\nexport int alpha_value();\n")
	writeFile(t, filepath.Join(dir, "src", "beta.cppm"), "export module beta;\nimport alpha;\nexport int beta_value();\n")

	report, err := codeguard.Run(context.Background(), graphTestConfig("design-cpp-module-cycle", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Design Patterns", "design.cpp.import-cycle")
}

func TestDesignCheckUsesCPPGraphForGodModules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "include", "common.hpp"), "#pragma once\n")
	writeFile(t, filepath.Join(dir, "src", "alpha.cpp"), "#include \"../include/common.hpp\"\n")
	writeFile(t, filepath.Join(dir, "src", "beta.cpp"), "#include \"../include/common.hpp\"\n")
	writeFile(t, filepath.Join(dir, "src", "gamma.cpp"), "#include \"../include/common.hpp\"\n")

	cfg := graphTestConfig("design-cpp-god-module", dir, "c++")
	cfg.Checks.DesignRules.GodModuleThreshold = 2
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Design Patterns", "design.god-module")
}

func TestDesignCheckWarnsForCPPGenericNameAndQualifiedMethodCount(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "utils.cpp"), "struct Worker { void one(); void two(); void three(); };\nvoid Worker::one() {}\nvoid Worker::two() {}\nvoid Worker::three() {}\n")

	cfg := graphTestConfig("design-cpp-heuristics", dir, "cpp")
	cfg.Checks.DesignRules.MaxMethodsPerType = 2
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Design Patterns", "design.cpp.generic-module-name")
	assertFindingRulePresent(t, report, "Design Patterns", "design.cpp.max-methods-per-type")
}

func TestDiffModeUsesCPPGraphForHighImpactChanges(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "CodeGuard Test")
	writeFile(t, filepath.Join(dir, "include", "base.hpp"), "#pragma once\nconstexpr int value = 1;\n")
	writeFile(t, filepath.Join(dir, "include", "mid.hpp"), "#pragma once\n#include \"base.hpp\"\n")
	writeFile(t, filepath.Join(dir, "src", "top.cpp"), "#include \"../include/mid.hpp\"\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")
	writeFile(t, filepath.Join(dir, "include", "base.hpp"), "#pragma once\nconstexpr int value = 2;\n")

	cfg := graphTestConfig("design-cpp-change-impact", dir, "cpp")
	cfg.Checks.DesignRules.HighImpactChangeThreshold = 1
	report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{
		Mode: codeguard.ScanModeDiff, BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("run diff: %v", err)
	}

	assertFindingRulePresent(t, report, "Design Patterns", "design.high-impact-change")
	artifact := changeImpactArtifact(t, report)
	for _, entry := range artifact.Entries {
		if entry.File == "include/base.hpp" && entry.Language == "cpp" && entry.TransitiveDependents == 2 {
			return
		}
	}
	t.Fatalf("artifact missing C++ impact entry: %+v", artifact.Entries)
}
