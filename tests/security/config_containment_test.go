package security_test

import (
	"path/filepath"
	"strings"
	"testing"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func writeConfigWithBaseline(t *testing.T, dir, baselinePath string) string {
	t.Helper()
	cfg := service.ExampleConfig()
	cfg.Baseline.Path = baselinePath
	cfg.Cache.Path = ""
	path := filepath.Join(dir, "codeguard.yaml")
	if err := service.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadConfigRejectsBaselinePathEscape(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigWithBaseline(t, dir, "../escape.json")

	_, err := service.LoadConfigFile(path)
	if err == nil {
		t.Fatal("expected config load to reject a baseline path escaping the config directory")
	}
	if !strings.Contains(err.Error(), "escape") {
		t.Fatalf("expected containment error, got %v", err)
	}
}

func TestLoadConfigAllowsContainedBaselinePath(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigWithBaseline(t, dir, ".codeguard/baseline.json")

	cfg, err := service.LoadConfigFile(path)
	if err != nil {
		t.Fatalf("contained baseline path should load, got %v", err)
	}
	want := filepath.Join(dir, ".codeguard/baseline.json")
	if cfg.Baseline.Path != want {
		t.Fatalf("baseline path = %q, want resolved %q", cfg.Baseline.Path, want)
	}
}

func TestLoadConfigRejectsAbsolutePathOutsideConfigDir(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigWithBaseline(t, dir, "/etc/codeguard-baseline.json")

	if _, err := service.LoadConfigFile(path); err == nil {
		t.Fatal("expected an absolute path outside the config directory to be rejected")
	}
}
