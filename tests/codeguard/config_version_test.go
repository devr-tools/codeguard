package codeguard_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/version"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func writeConfigYAML(t testing.TB, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "codeguard.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// A written config records which codeguard produced it, so a config checked
// into a repository carries its own provenance.
func TestWriteConfigStampsCodeguardVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "codeguard.yaml")
	if err := codeguard.WriteConfigFile(path, codeguard.ExampleConfig()); err != nil {
		t.Fatalf("write config: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "codeguard_version: "+version.Number) {
		t.Fatalf("written yaml does not record the codeguard version:\n%s", data)
	}

	loaded, err := codeguard.LoadConfigFile(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.CodeguardVersion != version.Number {
		t.Fatalf("loaded version = %q, want %q", loaded.CodeguardVersion, version.Number)
	}
}

// Writing is what stamps, so rewriting a config produced by an older release
// refreshes the record rather than preserving a stale one.
func TestWriteConfigRefreshesStaleVersionStamp(t *testing.T) {
	cfg := codeguard.ExampleConfig()
	cfg.CodeguardVersion = "0.0.1"
	path := filepath.Join(t.TempDir(), "codeguard.yaml")
	if err := codeguard.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	loaded, err := codeguard.LoadConfigFile(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.CodeguardVersion != version.Number {
		t.Fatalf("loaded version = %q, want the writing binary's %q", loaded.CodeguardVersion, version.Number)
	}
}

// Loading must report what the file says, not what the running binary is, so
// the stamp can be used to spot a config written by a different release.
func TestLoadConfigPreservesRecordedVersion(t *testing.T) {
	path := writeConfigYAML(t, `codeguard_version: 0.0.1
name: recorded
targets:
  - name: repo
    path: .
    language: go
output:
  format: text
`)
	loaded, err := codeguard.LoadConfigFile(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.CodeguardVersion != "0.0.1" {
		t.Fatalf("loaded version = %q, want the recorded 0.0.1", loaded.CodeguardVersion)
	}
}

// The stamp is informational: a config without one, or one written by a newer
// release, must never fail to load or validate.
func TestConfigVersionStampNeverBlocksLoading(t *testing.T) {
	tests := []struct {
		name  string
		stamp string
	}{
		{name: "absent", stamp: ""},
		{name: "older release", stamp: "codeguard_version: 0.0.1\n"},
		{name: "newer release", stamp: "codeguard_version: 99.0.0-rc1\n"},
		{name: "development build", stamp: "codeguard_version: devel\n"},
		{name: "unparseable", stamp: "codeguard_version: not-a-version\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfigYAML(t, tt.stamp+`name: stamped
targets:
  - name: repo
    path: .
    language: go
output:
  format: text
`)
			loaded, err := codeguard.LoadConfigFile(path)
			if err != nil {
				t.Fatalf("load config: %v", err)
			}
			if err := codeguard.ValidateConfig(loaded); err != nil {
				t.Fatalf("validate config: %v", err)
			}
		})
	}
}
