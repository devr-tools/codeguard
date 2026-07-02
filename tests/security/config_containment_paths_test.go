package security_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// writeConfig writes an example config into dir with the caller-supplied cache
// and ai-cache paths, leaving the baseline path empty so each artifact path can
// be exercised in isolation.
func writeConfig(t *testing.T, dir, cachePath, aiCachePath string) string {
	t.Helper()
	cfg := service.ExampleConfig()
	cfg.Baseline.Path = ""
	cfg.Cache.Path = cachePath
	cfg.AI.Cache.Path = aiCachePath
	path := filepath.Join(dir, "codeguard.yaml")
	if err := service.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadConfigRejectsCachePathEscape(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "../escape-cache.json", "")

	if _, err := service.LoadConfigFile(path); err == nil {
		t.Fatal("expected cache.path escaping the config directory to be rejected")
	}
}

func TestLoadConfigRejectsAICachePathEscape(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "", "../../escape-ai-cache.json")

	if _, err := service.LoadConfigFile(path); err == nil {
		t.Fatal("expected ai.cache.path escaping the config directory to be rejected")
	}
}

func TestLoadConfigAllowsContainedCachePaths(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, ".codeguard/cache.json", ".codeguard/ai-cache.json")

	cfg, err := service.LoadConfigFile(path)
	if err != nil {
		t.Fatalf("contained cache paths should load, got %v", err)
	}
	if want := filepath.Join(dir, ".codeguard/cache.json"); cfg.Cache.Path != want {
		t.Fatalf("cache.path = %q, want %q", cfg.Cache.Path, want)
	}
	if want := filepath.Join(dir, ".codeguard/ai-cache.json"); cfg.AI.Cache.Path != want {
		t.Fatalf("ai.cache.path = %q, want %q", cfg.AI.Cache.Path, want)
	}
}

// A symlink committed inside the config directory must not be usable to redirect
// an artifact write outside it. The lexical containment check passes here (the
// path is textually under the config dir), so this is the regression guard for
// the symlink-aware resolution.
func TestLoadConfigRejectsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlinks unsupported on this platform: %v", err)
	}
	path := writeConfig(t, dir, "link/evil-cache.json", "")

	_, err := service.LoadConfigFile(path)
	if err == nil {
		t.Fatal("expected a cache.path routed through an in-config symlink to be rejected")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected a symlink containment error, got %v", err)
	}
}
