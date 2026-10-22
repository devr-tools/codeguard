package support_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func writeFacadeRepo(t *testing.T, module string, packages ...string) string {
	t.Helper()
	dir := t.TempDir()
	if module != "" {
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module "+module+"\n\ngo 1.23\n"), 0o600); err != nil {
			t.Fatalf("write go.mod: %v", err)
		}
	}
	for _, name := range packages {
		if err := os.MkdirAll(filepath.Join(dir, "pkg", name), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	return dir
}

func facadeFor(t *testing.T, dir string, configured []string) runnersupport.SDKFacade {
	t.Helper()
	cfg := core.Config{}
	cfg.Checks.DesignRules.SDKFacadePaths = configured
	return runnersupport.NewSDKFacadeResolver(cfg).For(core.TargetConfig{Name: "repo", Path: dir})
}

// TestSDKFacadeDiscoversPublicPackageDirs pins the zero-config heuristic: the
// facade is read off the repository layout, not off the literal
// "pkg/codeguard" the predicate used to hardcode.
func TestSDKFacadeDiscoversPublicPackageDirs(t *testing.T) {
	for _, name := range []string{"myapp", "codeguard", "sdk"} {
		dir := writeFacadeRepo(t, "example.com/"+name, name)
		facade := facadeFor(t, dir, nil)
		if !facade.MatchesFile("pkg/" + name + "/facade.go") {
			t.Fatalf("pkg/%s should be discovered as the facade", name)
		}
		if facade.MatchesFile("internal/engine/engine.go") {
			t.Fatalf("internal/ must never be facade")
		}
		if facade.MatchesFile("pkg/other/other.go") {
			t.Fatalf("pkg/other does not exist and must not match")
		}
	}
}

func TestSDKFacadeConfigOverridesDiscovery(t *testing.T) {
	dir := writeFacadeRepo(t, "example.com/myapp", "myapp", "widgets")
	facade := facadeFor(t, dir, []string{"pkg/myapp/**"})
	if !facade.MatchesFile("pkg/myapp/facade.go") {
		t.Fatalf("configured facade must match")
	}
	if facade.MatchesFile("pkg/widgets/widgets.go") {
		t.Fatalf("an explicit sdk_facade_paths list must exclude every other public package")
	}
}

// TestSDKFacadeFilePrefixIsDirectoryScoped guards against a bare prefix test
// matching a sibling directory whose name merely starts the same way.
func TestSDKFacadeFilePrefixIsDirectoryScoped(t *testing.T) {
	dir := writeFacadeRepo(t, "example.com/app", "api")
	facade := facadeFor(t, dir, nil)
	if facade.MatchesFile("pkg/apiclient/client.go") {
		t.Fatalf("pkg/apiclient must not match the pkg/api facade")
	}
}

func TestSDKFacadeMatchesImportIsModuleQualified(t *testing.T) {
	dir := writeFacadeRepo(t, "example.com/myapp", "myapp")
	facade := facadeFor(t, dir, nil)
	cases := map[string]bool{
		"example.com/myapp/pkg/myapp":       true,
		"example.com/myapp/pkg/myapp/sub":   true,
		"github.com/someone/else/pkg/myapp": false,
		"example.com/myapp/internal/engine": false,
		"":                                  false,
	}
	for importPath, want := range cases {
		if got := facade.MatchesImport(importPath); got != want {
			t.Fatalf("MatchesImport(%q) = %v, want %v", importPath, got, want)
		}
	}
}

// TestSDKFacadeMatchesImportWithoutGoMod keeps the pre-existing path-segment
// behaviour for targets with no readable module path.
func TestSDKFacadeMatchesImportWithoutGoMod(t *testing.T) {
	dir := writeFacadeRepo(t, "", "codeguard")
	facade := facadeFor(t, dir, nil)
	if !facade.MatchesImport("github.com/devr-tools/codeguard/pkg/codeguard") {
		t.Fatalf("suffix fallback must still recognise the facade import")
	}
	if facade.MatchesImport("github.com/devr-tools/codeguard/internal/cli") {
		t.Fatalf("internal import must not look like the facade")
	}
}

func TestSDKFacadeEmptyWhenNoPublicPackages(t *testing.T) {
	dir := writeFacadeRepo(t, "example.com/app")
	facade := facadeFor(t, dir, nil)
	if facade.MatchesFile("pkg/app/app.go") || facade.MatchesImport("example.com/app/pkg/app") {
		t.Fatalf("a repository with no pkg/ tree has no facade")
	}
}

func TestNormalizeSDKFacadePaths(t *testing.T) {
	got := runnersupport.NormalizeSDKFacadePaths([]string{" pkg/sdk/** ", "pkg/sdk", "./pkg/api/", "", "   ", "pkg/zz/*"})
	want := []string{"pkg/api/", "pkg/sdk/", "pkg/zz/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeSDKFacadePaths = %v, want %v", got, want)
	}
}
