package support_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/runner/cpptooling"
	"github.com/devr-tools/codeguard/internal/codeguard/trust"
)

func TestCheckFormatUsesBuiltInToolAndReportsFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	root := t.TempDir()
	bin := t.TempDir()
	writeCPPToolExecutable(t, filepath.Join(bin, "clang-format"), "#!/bin/sh\ncase \"$*\" in *bad.cpp*) echo 'bad.cpp: formatting differs'; exit 1;; esac\n")
	t.Setenv("PATH", bin)
	issues, err := cpptooling.CheckFormat(context.Background(), root, core.CPPToolingConfig{}, []string{"good.cpp", "bad.cpp", "../escape.cpp"})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].Path != "bad.cpp" || !strings.Contains(issues[0].Message, "formatting differs") {
		t.Fatalf("issues = %#v", issues)
	}
}

func TestCommandOverrideUsesConfigCommandTrustGate(t *testing.T) {
	previous := trust.Current()
	trust.Set(trust.Policy{})
	defer trust.Set(previous)
	_, err := cpptooling.CheckFormat(context.Background(), t.TempDir(), core.CPPToolingConfig{ClangFormatCommand: "/bin/sh"}, nil)
	if err == nil || !strings.Contains(err.Error(), "refusing to run config-supplied command") {
		t.Fatalf("error = %v", err)
	}
}

func TestCheckSyntaxRebuildsSafeArgumentsAndNeverRunsDatabaseCompiler(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	root := t.TempDir()
	bin := t.TempDir()
	argsFile := filepath.Join(root, "args.txt")
	pwned := filepath.Join(root, "pwned")
	writeCPPToolExecutable(t, filepath.Join(bin, "clang++"), "#!/bin/sh\nprintf '%s\\n' \"$@\" > \""+argsFile+"\"\n")
	writeCPPToolExecutable(t, filepath.Join(bin, "database-compiler"), "#!/bin/sh\ntouch \""+pwned+"\"\n")
	t.Setenv("PATH", bin)
	writeCPPToolFile(t, filepath.Join(root, "include", "widget.h"), "#pragma once\n")
	writeCPPToolFile(t, filepath.Join(root, "src", "widget.cpp"), "int widget;\n")
	writeCPPToolFile(t, filepath.Join(root, "compile_commands.json"), `[{"directory":".","file":"src/widget.cpp","arguments":["database-compiler","-Iinclude","-DVALUE=1","-std=c++20","-fplugin=evil.so","@evil.rsp","-o","pwned","-c","src/widget.cpp"]}]`)
	issues, err := cpptooling.CheckSyntax(context.Background(), root, core.CPPToolingConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues = %#v", issues)
	}
	if _, statErr := os.Stat(pwned); !os.IsNotExist(statErr) {
		t.Fatalf("database compiler unexpectedly ran: %v", statErr)
	}
	// #nosec G304 -- argsFile is constructed beneath the test's temporary root.
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(args)
	assertCPPToolArguments(t, root, text)
	assertCPPToolArgumentsExcluded(t, text)
}

func assertCPPToolArguments(t *testing.T, root, arguments string) {
	t.Helper()
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"-fsyntax-only", "-std=c++20", "-I" + filepath.Join(canonicalRoot, "include"), "-DVALUE=1", filepath.Join(canonicalRoot, "src", "widget.cpp")} {
		if !strings.Contains(arguments, want) {
			t.Fatalf("args %q missing %q", arguments, want)
		}
	}
}

func assertCPPToolArgumentsExcluded(t *testing.T, arguments string) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(arguments), "\n")
	lineSet := make(map[string]struct{}, len(lines))
	for _, argument := range lines {
		lineSet[argument] = struct{}{}
	}
	for _, forbidden := range []string{"database-compiler", "-fplugin=evil.so", "@evil.rsp", "-o", "pwned"} {
		if _, ok := lineSet[forbidden]; ok {
			t.Fatalf("unsafe database argument %q reached compiler: %q", forbidden, arguments)
		}
	}
}

func writeCPPToolExecutable(t *testing.T, path, content string) {
	t.Helper()
	writeCPPToolFile(t, path, content)
	// #nosec G302 -- this fixture must be executable to stand in for the external tool.
	if err := os.Chmod(path, 0o750); err != nil {
		t.Fatal(err)
	}
}

func writeCPPToolFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
