package checks_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestSupplyChainParsesStaticCMakeDependencies(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "CMakeLists.txt"), `
include(FetchContent)
file(WRITE "${CMAKE_CURRENT_LIST_DIR}/executed-by-cmake" "unsafe")
set(FMT_TAG "10.2.1")

FetchContent_Declare(
  fmt
  GIT_REPOSITORY https://github.com/fmtlib/fmt.git
  GIT_TAG ${FMT_TAG}
)
FetchContent_MakeAvailable(fmt)

ExternalProject_Add(zlib
  URL "https://zlib.net/fossils/zlib-1.3.1.tar.gz"
  URL_HASH SHA256=38ef96b8
)

CPMAddPackage(
  NAME Catch2
  GITHUB_REPOSITORY catchorg/Catch2
  VERSION 3.5.2
)

find_package(OpenSSL 3.2.1 EXACT REQUIRED)
find_package(Boost 1.84...<1.90 REQUIRED)
FetchContent_Declare(floating GIT_REPOSITORY https://example.com/floating.git GIT_TAG main)
FetchContent_Declare(dynamic GIT_REPOSITORY ${PRIVATE_MIRROR}/dynamic.git GIT_TAG ${DYNAMIC_TAG})
`)

	report, err := codeguard.Run(context.Background(), supplyChainTestConfig(dir, "cmake-static"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	manifest := requireSupplyChainArtifact(t, report, 1).SupplyChain.Manifests[0]
	if manifest.Ecosystem != "cmake" || manifest.PackageManager != "cmake" {
		t.Fatalf("unexpected CMake manifest identity: %#v", manifest)
	}
	assertStaticCMakeDependencies(t, manifest.Dependencies)
	if len(manifest.AnalysisLimitations) != 1 || !strings.Contains(manifest.AnalysisLimitations[0], "did not execute CMake") {
		t.Fatalf("unexpected CMake analysis limitations: %v", manifest.AnalysisLimitations)
	}

	messages := supplyChainRuleMessages(report, "supply_chain.unpinned-dependency")
	if len(messages) != 3 {
		t.Fatalf("CMake unpinned findings = %d, want 3: %v", len(messages), messages)
	}
	if _, err := os.Stat(filepath.Join(dir, "executed-by-cmake")); !os.IsNotExist(err) {
		t.Fatalf("CMake parser executed project code: %v", err)
	}
}

func assertStaticCMakeDependencies(t *testing.T, declared []codeguard.SupplyChainDependency) {
	t.Helper()
	dependencies := cppDependencyByName(declared)
	expectedPins := map[string]bool{
		"fmt": true, "zlib": true, "Catch2": true, "OpenSSL": true,
		"Boost": false, "floating": false, "dynamic": false,
	}
	for name, pinned := range expectedPins {
		dependency, ok := dependencies[name]
		if !ok || dependency.Pinned != pinned {
			t.Fatalf("unexpected CMake dependency %q: %#v", name, dependency)
		}
	}
	if dependencies["Boost"].Requirement != "1.84...<1.90" {
		t.Fatalf("CMake version range was not preserved: %#v", dependencies["Boost"])
	}
	if dependencies["fmt"].Requirement != "https://github.com/fmtlib/fmt.git@10.2.1" || dependencies["fmt"].Line != 6 {
		t.Fatalf("unexpected resolved fmt dependency: %#v", dependencies["fmt"])
	}
	if dependencies["zlib"].Version != "1.3.1" || dependencies["zlib"].Line != 13 {
		t.Fatalf("unexpected URL dependency: %#v", dependencies["zlib"])
	}
}

func TestSupplyChainParsesCPMCompactReference(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "dependencies.cmake"), `CPMAddPackage("gh:gabime/spdlog@1.14.1")`)

	report, err := codeguard.Run(context.Background(), supplyChainTestConfig(dir, "cmake-cpm-compact"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	dependency := requireSupplyChainArtifact(t, report, 1).SupplyChain.Manifests[0].Dependencies[0]
	if dependency.Name != "spdlog" || dependency.Version != "1.14.1" || !dependency.Pinned {
		t.Fatalf("unexpected compact CPM dependency: %#v", dependency)
	}
}

func TestSupplyChainParsesDeclarativeConanfilePython(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "conanfile.py"), `from conan import ConanFile
from pathlib import Path
Path(__file__).with_name("executed-by-conan").write_text("unsafe")

FMT = "fmt/10.2.1"
BUILD_TOOLS = (
    "cmake/3.29.0",
)

class App(ConanFile):
    requires = (
        FMT,
        "openssl/[>=3.0 <4]",
    )
    tool_requires = BUILD_TOOLS

    def requirements(self):
        self.requires(
            "zlib/1.3.1",
            transitive_headers=True,
        )
        self.requires(make_reference())
`)
	writeFile(t, filepath.Join(dir, "conan.lock"), `{"version":"0.5","requires":[],"build_requires":[]}`)

	report, err := codeguard.Run(context.Background(), supplyChainTestConfig(dir, "conan-python-static"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	manifest := requireSupplyChainArtifact(t, report, 1).SupplyChain.Manifests[0]
	dependencies := cppDependencyByName(manifest.Dependencies)
	for _, name := range []string{"fmt", "cmake", "openssl", "zlib"} {
		if _, ok := dependencies[name]; !ok {
			t.Fatalf("missing Conan dependency %q in %#v", name, manifest.Dependencies)
		}
	}
	if dependencies["cmake"].Scope != "build" || dependencies["cmake"].Line != 7 {
		t.Fatalf("unexpected Conan tool requirement: %#v", dependencies["cmake"])
	}
	if dependencies["openssl"].Pinned {
		t.Fatalf("Conan version range should be unpinned: %#v", dependencies["openssl"])
	}
	if dependencies["zlib"].Line != 19 {
		t.Fatalf("unexpected self.requires line: %#v", dependencies["zlib"])
	}
	if len(manifest.AnalysisLimitations) != 1 || !strings.Contains(manifest.AnalysisLimitations[0], "did not execute Python") {
		t.Fatalf("unexpected Conan analysis limitations: %v", manifest.AnalysisLimitations)
	}
	if messages := supplyChainRuleMessages(report, "supply_chain.unpinned-dependency"); len(messages) != 1 || !strings.Contains(messages[0], "openssl") {
		t.Fatalf("unexpected Conan Python unpinned findings: %v", messages)
	}
	if _, err := os.Stat(filepath.Join(dir, "executed-by-conan")); !os.IsNotExist(err) {
		t.Fatalf("Conan parser executed project code: %v", err)
	}
}

func TestSupplyChainDoesNotTreatArbitraryPythonAsConanManifest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "setup.py"), `self.requires("not-a-conan-reference/1.0.0")`)

	report, err := codeguard.Run(context.Background(), supplyChainTestConfig(dir, "non-conan-python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, artifact := range report.Artifacts {
		if artifact.Kind == "supply_chain" {
			t.Fatalf("arbitrary Python produced a supply-chain artifact: %#v", artifact)
		}
	}
}

func cppDependencyByName(dependencies []codeguard.SupplyChainDependency) map[string]codeguard.SupplyChainDependency {
	indexed := make(map[string]codeguard.SupplyChainDependency, len(dependencies))
	for _, dependency := range dependencies {
		indexed[dependency.Name] = dependency
	}
	return indexed
}
