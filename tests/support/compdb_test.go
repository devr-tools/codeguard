package support_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/cpp/compdb"
)

func TestLoadNormalizesCommandMetadataAndKeepsOnlyTargetLocalPaths(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeCompDBFile(t, filepath.Join(root, "src", "widget.cpp"), "int widget;\n")
	if err := os.MkdirAll(filepath.Join(root, "include"), 0o750); err != nil {
		t.Fatal(err)
	}
	writeCompDBFile(t, filepath.Join(root, "build", "compile_commands.json"), `[
  {
    "directory": "..",
    "file": "src/widget.cpp",
    "command": "malicious-wrapper clang++ -Iinclude -I`+outside+` -DNAME='hello world' -UOLD -std=c++20 -fplugin=evil.so -c src/widget.cpp"
  },
  {
    "directory": "..",
    "file": "../outside.cpp",
    "arguments": ["clang++", "-c", "../outside.cpp"]
  }
]`)

	db, err := compdb.Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(db.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(db.Entries))
	}
	entry := db.Entries[0]
	if entry.RelativeFile != "src/widget.cpp" || entry.Compiler != "malicious-wrapper" {
		t.Fatalf("entry = %#v", entry)
	}
	wantInclude, err := filepath.EvalSymlinks(filepath.Join(root, "include"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entry.IncludeDirs) != 1 || entry.IncludeDirs[0] != wantInclude {
		t.Fatalf("include dirs = %#v", entry.IncludeDirs)
	}
	if len(entry.Defines) != 1 || entry.Defines[0] != "NAME=hello world" {
		t.Fatalf("defines = %#v", entry.Defines)
	}
	if entry.Standard != "c++20" {
		t.Fatalf("standard = %q", entry.Standard)
	}
}

func TestFindRejectsConfiguredPathOutsideTarget(t *testing.T) {
	if _, err := compdb.Find(t.TempDir(), "../compile_commands.json"); err == nil {
		t.Fatal("expected escaping compile_commands path to be rejected")
	}
}

func TestFindRejectsCompilationDatabaseSymlinkOutsideTarget(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "compile_commands.json")
	writeCompDBFile(t, outside, "[]")
	if err := os.Symlink(outside, filepath.Join(root, "compile_commands.json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := compdb.Find(root, "compile_commands.json"); err == nil {
		t.Fatal("expected an out-of-target compilation database symlink to be rejected")
	}
}

func TestLoadRejectsMalformedCommandForm(t *testing.T) {
	root := t.TempDir()
	writeCompDBFile(t, filepath.Join(root, "source.cpp"), "int source;\n")
	writeCompDBFile(t, filepath.Join(root, "compile_commands.json"), `[{"directory":".","file":"source.cpp","command":"clang++ 'unterminated"}]`)
	db, err := compdb.Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(db.Entries) != 0 {
		t.Fatalf("entries = %#v, want malformed command entry skipped", db.Entries)
	}
}

func writeCompDBFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
