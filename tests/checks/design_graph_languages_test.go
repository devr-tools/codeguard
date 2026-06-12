package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func graphTestConfig(name string, dir string, language string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: language}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	return cfg
}

func assertFindingRuleAbsent(t *testing.T, report codeguard.Report, section string, ruleID string) {
	t.Helper()
	for _, result := range report.Sections {
		if result.Name != section {
			continue
		}
		for _, finding := range result.Findings {
			if finding.RuleID == ruleID {
				t.Fatalf("section %q unexpectedly contains rule %q: %s", section, ruleID, finding.Message)
			}
		}
		return
	}
	t.Fatalf("section %q not found", section)
}

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

func TestDesignCheckFailsForRustModuleCycle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "lib.rs"), "mod reader;\nmod writer;\n")
	writeFile(t, filepath.Join(dir, "src", "reader.rs"), "use crate::writer::Writer;\n\npub struct Reader;\n")
	writeFile(t, filepath.Join(dir, "src", "writer.rs"), "use crate::reader::Reader;\n\npub struct Writer;\n")

	report, err := codeguard.Run(context.Background(), graphTestConfig("design-rust-cycle", dir, "rust"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "fail")
	assertFindingRulePresent(t, report, "Design Patterns", "design.rust.import-cycle")
}

func TestDesignCheckPassesForAcyclicRustModules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "lib.rs"), "mod reader;\nmod writer;\n")
	writeFile(t, filepath.Join(dir, "src", "reader.rs"), "use crate::writer::Writer;\n\npub struct Reader;\n")
	writeFile(t, filepath.Join(dir, "src", "writer.rs"), "pub struct Writer;\n")

	report, err := codeguard.Run(context.Background(), graphTestConfig("design-rust-no-cycle", dir, "rust"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Design Patterns", "design.rust.import-cycle")
}

func TestDesignCheckFailsForJavaImportCycle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "store", "Store.java"), "package store;\n\nimport web.Handler;\n\npublic class Store {}\n")
	writeFile(t, filepath.Join(dir, "web", "Handler.java"), "package web;\n\nimport store.Store;\n\npublic class Handler {}\n")

	report, err := codeguard.Run(context.Background(), graphTestConfig("design-java-cycle", dir, "java"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "fail")
	assertFindingRulePresent(t, report, "Design Patterns", "design.java.import-cycle")
}

func TestDesignCheckPassesForAcyclicJavaImports(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "store", "Store.java"), "package store;\n\npublic class Store {}\n")
	writeFile(t, filepath.Join(dir, "web", "Handler.java"), "package web;\n\nimport store.Store;\n\npublic class Handler {}\n")

	report, err := codeguard.Run(context.Background(), graphTestConfig("design-java-no-cycle", dir, "java"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Design Patterns", "design.java.import-cycle")
}
