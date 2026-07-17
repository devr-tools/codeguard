package checks_test

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestCPPToolingRunsFormatterAndSanitizedCompilerForExplicitTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	root := t.TempDir()
	bin := t.TempDir()
	writeFile(t, filepath.Join(root, "widget.cpp"), "int widget;\n")
	writeFile(t, filepath.Join(root, "compile_commands.json"), `[{"directory":".","file":"widget.cpp","arguments":["untrusted-database-compiler","-std=c++20","-c","widget.cpp"]}]`)
	writeExecutableFile(t, filepath.Join(bin, "clang-format"), "#!/bin/sh\necho 'widget.cpp: formatting differs'\nexit 1\n")
	writeExecutableFile(t, filepath.Join(bin, "clang++"), "#!/bin/sh\necho 'widget.cpp:1:1: error: expected declaration'\nexit 1\n")
	t.Setenv("PATH", bin)
	cfg := qualityOnlyConfig("cpp-tooling", root, "cpp")
	cfg.Checks.QualityRules.CPPTooling = codeguard.CPPToolingConfig{ClangFormatMode: "required", CompilerMode: "required"}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if messages := findingMessagesForRule(report, "quality.cpp.clang-format"); len(messages) != 1 || !strings.Contains(messages[0], "formatting differs") {
		t.Fatalf("clang-format messages = %#v", messages)
	}
	if messages := findingMessagesForRule(report, "quality.cpp.compiler-parse"); len(messages) != 1 || !strings.Contains(messages[0], "expected declaration") {
		t.Fatalf("compiler messages = %#v", messages)
	}
}

func TestCPPToolingAutoSkipsUnavailableTools(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "widget.cpp"), "int widget;\n")
	t.Setenv("PATH", t.TempDir())
	cfg := qualityOnlyConfig("cpp-tooling-auto", root, "cpp")
	cfg.Checks.QualityRules.CPPTooling = codeguard.CPPToolingConfig{ClangFormatMode: "auto", CompilerMode: "auto"}
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if messages := findingMessagesForRule(report, "quality.cpp.clang-format"); len(messages) != 0 {
		t.Fatalf("clang-format auto messages = %#v", messages)
	}
	if messages := findingMessagesForRule(report, "quality.cpp.compiler-parse"); len(messages) != 0 {
		t.Fatalf("compiler auto messages = %#v", messages)
	}
}

func TestCPPToolingRequiredReportsMissingCompilationDatabase(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	root := t.TempDir()
	bin := t.TempDir()
	writeFile(t, filepath.Join(root, "widget.cpp"), "int widget;\n")
	writeExecutableFile(t, filepath.Join(bin, "clang++"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin)
	cfg := qualityOnlyConfig("cpp-tooling-required", root, "cpp")
	cfg.Checks.QualityRules.CPPTooling = codeguard.CPPToolingConfig{CompilerMode: "required"}
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	messages := findingMessagesForRule(report, "quality.cpp.compiler-parse")
	if len(messages) != 1 || !strings.Contains(messages[0], "compile_commands.json not found") {
		t.Fatalf("compiler messages = %#v", messages)
	}
}

func TestCPPToolingRejectsInvalidMode(t *testing.T) {
	cfg := qualityOnlyConfig("cpp-tooling-invalid", t.TempDir(), "cpp")
	cfg.Checks.QualityRules.CPPTooling.ClangFormatMode = "sometimes"
	if _, err := codeguard.Run(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "clang_format_mode") {
		t.Fatalf("error = %v", err)
	}
}
