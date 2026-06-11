package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigPathPrefersProjectRootThenDotCodeguard(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir tempdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	if got := DefaultConfigPath(); got != "codeguard.yaml" {
		t.Fatalf("default config path = %q, want %q", got, "codeguard.yaml")
	}

	dotCodeguardPath := filepath.Join(dir, ".codeguard", "codeguard.json")
	if err := os.MkdirAll(filepath.Dir(dotCodeguardPath), 0o755); err != nil {
		t.Fatalf("mkdir .codeguard: %v", err)
	}
	if err := os.WriteFile(dotCodeguardPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write .codeguard config: %v", err)
	}

	if got := DefaultConfigPath(); got != filepath.Join(".codeguard", "codeguard.json") {
		t.Fatalf("default config path = %q, want %q", got, filepath.Join(".codeguard", "codeguard.json"))
	}

	rootPath := filepath.Join(dir, "codeguard.yml")
	if err := os.WriteFile(rootPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write root config: %v", err)
	}

	if got := DefaultConfigPath(); got != "codeguard.yml" {
		t.Fatalf("default config path = %q, want %q", got, "codeguard.yml")
	}
}

func TestResolveConfigPathForDirectory(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".codeguard")
	configPath := filepath.Join(configDir, "config.yaml")

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir .codeguard: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	resolved, err := ResolveConfigPath(configDir)
	if err != nil {
		t.Fatalf("ResolveConfigPath returned error: %v", err)
	}
	if resolved != configPath {
		t.Fatalf("ResolveConfigPath = %q, want %q", resolved, configPath)
	}
}
