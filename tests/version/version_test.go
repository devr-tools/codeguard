package version_test

import (
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/version"
)

func TestModuleVersionFromBuildInfo(t *testing.T) {
	for _, tt := range []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{name: "nil"},
		{name: "empty", info: &debug.BuildInfo{}},
		{name: "development", info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}},
		{name: "unknown", info: &debug.BuildInfo{Main: debug.Module{Version: "unknown"}}},
		{name: "module version", info: &debug.BuildInfo{Main: debug.Module{Version: "v1.5.1"}}, want: "v1.5.1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := version.ModuleVersionFromBuildInfo(tt.info); got != tt.want {
				t.Fatalf("ModuleVersionFromBuildInfo() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuiltCommandReportsInjectedVersion(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	binary := filepath.Join(t.TempDir(), "codeguard")
	build := exec.Command("go", "build", "-ldflags", "-X github.com/devr-tools/codeguard/internal/version.Number=v9.8.7", "-o", binary, "./cmd/codeguard")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build command: %v\n%s", err, output)
	}
	output, err := exec.Command(binary, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("run version: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(string(output)); got != "v9.8.7" {
		t.Fatalf("version output = %q, want v9.8.7", got)
	}
}

func TestDevelopmentVersionFromBuildInfo(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abcdef1234567890"},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	if got, want := version.DevelopmentVersionFromBuildInfo(info), "devel+abcdef12.dirty"; got != want {
		t.Fatalf("DevelopmentVersionFromBuildInfo() = %q, want %q", got, want)
	}
}

func TestResolvePreservesInjectedVersion(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "v1.5.1"}}
	if got := version.Resolve("v9.9.9", info); got != "v9.9.9" {
		t.Fatalf("Resolve() = %q, want linker-injected version", got)
	}
}
