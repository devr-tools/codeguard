package govulncheck

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoverModulesFromWorkspaceWithoutRootModule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, root, "go.work", "go 1.24\nuse (\n\t./services/api\n\t./libs/auth\n)\nreplace (\nexample.com/old => ./libs/replacement\n)\n")
	writeTestFile(t, root, "services/api/go.mod", "module example.com/api\n\ngo 1.24\n")
	writeTestFile(t, root, "libs/auth/go.mod", "module example.com/auth\n\ngo 1.24\n")
	writeTestFile(t, root, "libs/replacement/go.mod", "module example.com/replacement\n\ngo 1.24\n")

	workspace, err := DiscoverWorkspace(root)
	if err != nil {
		t.Fatalf("DiscoverWorkspace() error = %v", err)
	}
	if workspace.RootModule != "" {
		t.Fatalf("RootModule = %q, want empty", workspace.RootModule)
	}
	if len(workspace.Modules) != 2 {
		t.Fatalf("Modules = %#v, want two active use modules", workspace.Modules)
	}
	if got := workspace.Modules[0].ModulePath; got != "example.com/api" {
		t.Errorf("first module = %q, want deterministic example.com/api", got)
	}
	if got := workspace.Replacements["example.com/old"]; got != filepath.Join(root, "libs/replacement") {
		t.Errorf("replacement = %q", got)
	}
}

func TestScanWorkspaceKeepsPartialResultsAndDeduplicatesAdvisories(t *testing.T) {
	t.Parallel()
	workspace := Workspace{Modules: []Module{
		{Dir: "/workspace/a", ModulePath: "example.com/a"},
		{Dir: "/workspace/b", ModulePath: "example.com/b"},
		{Dir: "/workspace/broken", ModulePath: "example.com/broken"},
	}}
	execute := func(_ context.Context, module Module) ([]Vulnerability, error) {
		if module.ModulePath == "example.com/broken" {
			return nil, errors.New("packages unavailable")
		}
		return []Vulnerability{{AdvisoryID: "GO-2099-0001", Package: module.ModulePath + "/pkg", CallStack: []string{"entry", "sink"}}}, nil
	}

	result := ScanWorkspace(context.Background(), workspace, ScanOptions{Concurrency: 2, ModuleTimeout: time.Second, Execute: execute})
	if len(result.Vulnerabilities) != 1 {
		t.Fatalf("Vulnerabilities = %#v, want one deduplicated advisory", result.Vulnerabilities)
	}
	if got := len(result.Vulnerabilities[0].Occurrences); got != 2 {
		t.Fatalf("Occurrences = %d, want both module occurrences", got)
	}
	if result.Modules[2].Status != ModuleFailed {
		t.Fatalf("broken status = %q, want failed", result.Modules[2].Status)
	}
}

func TestScanWorkspaceTimesOutOneModuleWithoutDiscardingOthers(t *testing.T) {
	t.Parallel()
	workspace := Workspace{Modules: []Module{
		{Dir: "/workspace/fast", ModulePath: "example.com/fast"},
		{Dir: "/workspace/slow", ModulePath: "example.com/slow"},
	}}
	execute := func(ctx context.Context, module Module) ([]Vulnerability, error) {
		if module.ModulePath == "example.com/slow" {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return []Vulnerability{{AdvisoryID: "GO-2099-0002"}}, nil
	}
	result := ScanWorkspace(context.Background(), workspace, ScanOptions{Concurrency: 2, ModuleTimeout: 10 * time.Millisecond, Execute: execute})
	if len(result.Vulnerabilities) != 1 {
		t.Fatalf("Vulnerabilities = %#v, want fast module result", result.Vulnerabilities)
	}
	if result.Modules[1].Status != ModuleTimedOut {
		t.Fatalf("slow status = %q, want timed_out", result.Modules[1].Status)
	}
}

func TestDiscoverModulesUsesNearestNestedModule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/root\n\ngo 1.24\n")
	writeTestFile(t, root, "nested/go.mod", "module example.com/nested\n\ngo 1.24\n")

	workspace, err := DiscoverWorkspace(filepath.Join(root, "nested", "pkg"))
	if err != nil {
		t.Fatalf("DiscoverWorkspace() error = %v", err)
	}
	if len(workspace.Modules) != 1 || workspace.Modules[0].ModulePath != "example.com/nested" {
		t.Fatalf("Modules = %#v, want nearest nested module", workspace.Modules)
	}
}

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
