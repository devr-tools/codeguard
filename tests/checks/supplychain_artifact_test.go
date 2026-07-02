package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestSupplyChainPublishesNormalizedManifestArtifact(t *testing.T) {
	dir := t.TempDir()
	writeSupplyChainArtifactFixtures(t, dir)

	cacheEnabled := false
	contextOff := false
	cfg := codeguard.Config{
		Name: "supply-chain-artifact",
		Targets: []codeguard.TargetConfig{{
			Name:     "repo",
			Path:     dir,
			Language: "go",
		}},
		Checks: codeguard.CheckConfig{
			SupplyChain: true,
			Context:     &contextOff,
		},
		Output: codeguard.OutputConfig{Format: "json"},
		Cache:  codeguard.CacheConfig{Enabled: &cacheEnabled},
	}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	artifact := requireSupplyChainArtifact(t, report, 5)
	manifests := manifestByPath(artifact.SupplyChain.Manifests)
	assertGoManifest(t, manifests["go.mod"])
	assertWebManifest(t, manifests["web/package.json"])
	assertPythonManifests(t, manifests["requirements.txt"], manifests["pyproject.toml"])
	assertCargoManifest(t, manifests["rust/Cargo.toml"])
}

func writeSupplyChainArtifactFixtures(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/supplychain\n\ngo 1.23.0\n\nrequire github.com/stretchr/testify v1.9.0\n")
	writeFile(t, filepath.Join(dir, "go.sum"), "github.com/stretchr/testify v1.9.0 h1:test\n")
	writeFile(t, filepath.Join(dir, "web", "package.json"), `{
  "name": "frontend",
  "packageManager": "pnpm@9.0.0",
  "dependencies": {"react": "18.2.0"},
  "devDependencies": {"vitest": "^1.6.0"},
  "peerDependencies": {"typescript": "~5.4.0"}
}`)
	writeFile(t, filepath.Join(dir, "web", "pnpm-lock.yaml"), "lockfileVersion: '9.0'\n")
	writeFile(t, filepath.Join(dir, "node_modules", "react", "package.json"), `{"name":"react","version":"18.2.0","license":"MIT"}`)
	writeFile(t, filepath.Join(dir, "node_modules", "vitest", "package.json"), `{"name":"vitest","version":"1.6.0","license":"MIT"}`)
	writeFile(t, filepath.Join(dir, "node_modules", "typescript", "package.json"), `{"name":"typescript","version":"5.4.0","license":"Apache-2.0"}`)
	writeFile(t, filepath.Join(dir, "requirements.txt"), "requests==2.31.0\nflask>=3.0\n")
	writeFile(t, filepath.Join(dir, "pyproject.toml"), `[project]
name = "backend"
dependencies = [
  "pydantic==2.7.0",
]

[project.optional-dependencies]
dev = ["pytest>=8.0"]
`)
	writeFile(t, filepath.Join(dir, "poetry.lock"), "# lock\n")
	writeFile(t, filepath.Join(dir, "rust", "Cargo.toml"), `[package]
name = "worker"
version = "0.1.0"

[dependencies]
serde = "1.0"
tokio = { version = "=1.38.0" }
`)
	writeFile(t, filepath.Join(dir, "rust", "Cargo.lock"), "# lock\n")
}

func requireSupplyChainArtifact(t *testing.T, report codeguard.Report, manifestCount int) codeguard.Artifact {
	t.Helper()
	var artifact codeguard.Artifact
	found := 0
	for _, candidate := range report.Artifacts {
		if candidate.Kind == "supply_chain" {
			artifact = candidate
			found++
		}
	}
	if found != 1 {
		t.Fatalf("supply_chain artifacts = %d, want 1 (all artifacts: %#v)", found, report.Artifacts)
	}
	if artifact.SupplyChain == nil {
		t.Fatal("expected supply_chain payload")
	}
	if got := len(artifact.SupplyChain.Manifests); got != manifestCount {
		t.Fatalf("manifest count = %d, want %d", got, manifestCount)
	}
	return artifact
}

func manifestByPath(manifests []codeguard.SupplyChainManifest) map[string]codeguard.SupplyChainManifest {
	indexed := map[string]codeguard.SupplyChainManifest{}
	for _, manifest := range manifests {
		indexed[manifest.Path] = manifest
	}
	return indexed
}

func assertGoManifest(t *testing.T, manifest codeguard.SupplyChainManifest) {
	t.Helper()
	if manifest.Ecosystem != "go" {
		t.Fatalf("go.mod ecosystem = %q", manifest.Ecosystem)
	}
	if got := manifest.Dependencies[0].Pinned; !got {
		t.Fatalf("expected go dependency to be pinned")
	}
}

func assertWebManifest(t *testing.T, manifest codeguard.SupplyChainManifest) {
	t.Helper()
	if got := manifest.PackageManager; got != "pnpm" {
		t.Fatalf("package manager = %q, want pnpm", got)
	}
	if got := manifest.Lockfiles[0]; got != "web/pnpm-lock.yaml" {
		t.Fatalf("lockfile = %q, want web/pnpm-lock.yaml", got)
	}
	webDeps := map[string]codeguard.SupplyChainDependency{}
	for _, dep := range manifest.Dependencies {
		webDeps[dep.Name] = dep
	}
	if got := webDeps["react"].License; got != "MIT" {
		t.Fatalf("react license = %q, want MIT", got)
	}
	if got := webDeps["typescript"].LicenseSource; got != "node_modules" {
		t.Fatalf("typescript license source = %q, want node_modules", got)
	}
}

func assertPythonManifests(t *testing.T, requirements codeguard.SupplyChainManifest, pyproject codeguard.SupplyChainManifest) {
	t.Helper()
	requirementsDeps := map[string]codeguard.SupplyChainDependency{}
	for _, dep := range requirements.Dependencies {
		requirementsDeps[dep.Name] = dep
	}
	if got := requirementsDeps["requests"].Pinned; !got {
		t.Fatalf("expected requests dependency to be pinned")
	}
	if got := requirementsDeps["flask"].Pinned; got {
		t.Fatalf("expected flask dependency to be unpinned")
	}
	if got := pyproject.Name; got != "backend" {
		t.Fatalf("pyproject name = %q, want backend", got)
	}
	if got := pyproject.PackageManager; got != "" {
		t.Fatalf("pyproject package manager = %q, want empty", got)
	}
}

func assertCargoManifest(t *testing.T, manifest codeguard.SupplyChainManifest) {
	t.Helper()
	cargoDeps := map[string]codeguard.SupplyChainDependency{}
	for _, dep := range manifest.Dependencies {
		cargoDeps[dep.Name] = dep
	}
	if got := cargoDeps["serde"].Pinned; got {
		t.Fatalf("expected serde cargo dependency to be unpinned")
	}
	if got := cargoDeps["tokio"].Pinned; !got {
		t.Fatalf("expected tokio cargo dependency to be pinned")
	}
}
