package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

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
