package support

import (
	"os"
	"path/filepath"
	"strings"
)

// buildOutputDirMarkers maps an *ambiguous* directory name to the sibling files
// that prove the directory is a build system's output directory rather than
// hand-written source.
//
// Why a corroborating signal instead of a plain name match (which is what
// unambiguousGeneratedDirs in utils_match.go does): every name here is real
// source somewhere. A Go package can be called build; a Rust binary crate lives
// in src/bin; plenty of repositories keep their build tooling in build/ and
// their application in out/. Silently dropping those from the scan would turn a
// false positive problem into a false *negative* problem, which for a security
// scanner is strictly worse.
//
// The markers are chosen so the directory name is that build system's output
// convention, not merely "a project of this kind exists here":
//
//   - target   cargo, maven and sbt all emit into ./target
//   - build    out-of-source cmake, gradle, meson, setuptools and dune emit
//     into ./build. Note package.json is deliberately NOT a marker:
//     a JS repo with a build/ directory very often keeps build
//     *scripts* there (build/webpack.config.js), and we would rather
//     scan create-react-app output than hide those.
//   - out      next export and tsc outDir conventions; also MSBuild
//   - bin/obj  MSBuild emits both beside the project file. A Go or Rust repo has
//     no project file, so src/bin stays scanned.
//   - coverage jest/nyc, cargo-llvm-cov and Go coverage tooling default to
//     ./coverage. Python markers are deliberately absent: coverage.py's
//     own repository has a coverage/ *source* package next to its
//     pyproject.toml, which is exactly the trap this table avoids.
//
// A leading "*" makes the marker a suffix match against the sibling directory's
// entries (used for MSBuild project files, whose stem is the project name).
var buildOutputDirMarkers = map[string][]string{
	"target": {"Cargo.toml", "pom.xml", "build.sbt"},
	"build": {
		"CMakeLists.txt",
		"build.gradle",
		"build.gradle.kts",
		"gradlew",
		"settings.gradle",
		"settings.gradle.kts",
		"meson.build",
		"setup.py",
		"pyproject.toml",
		"dune-project",
	},
	"_build":   {"dune-project", "mix.exs"},
	"out":      {"next.config.js", "next.config.mjs", "next.config.ts", "tsconfig.json", "*.sln", "*.csproj"},
	"bin":      {"*.csproj", "*.fsproj", "*.vbproj", "*.sln"},
	"obj":      {"*.csproj", "*.fsproj", "*.vbproj", "*.sln"},
	"coverage": {"package.json", "Cargo.toml", "go.mod"},
}

// isCorroboratedBuildOutputDir reports whether dir - whose base name is one of
// the ambiguous names in buildOutputDirMarkers - is really build output, judged
// by the manifest files sitting beside it.
//
// This lives in the directory walk rather than in ShouldExclude because the
// decision needs the filesystem, and ShouldExclude is a pure path predicate
// used for path-only filtering. Consequence to be aware of: these names are
// skipped during a repository walk but are not considered excluded by
// ShouldExclude, so a path-only caller (diff-mode filtering) still sees them.
// That asymmetry errs toward scanning, which is the safe direction.
func isCorroboratedBuildOutputDir(dir string) bool {
	markers, ok := buildOutputDirMarkers[filepath.Base(dir)]
	if !ok {
		return false
	}
	parent := filepath.Dir(dir)
	var suffixes []string
	for _, marker := range markers {
		if suffix, isSuffix := strings.CutPrefix(marker, "*"); isSuffix {
			suffixes = append(suffixes, suffix)
			continue
		}
		if info, err := os.Stat(filepath.Join(parent, marker)); err == nil && !info.IsDir() {
			return true
		}
	}
	return len(suffixes) > 0 && dirHasEntryWithSuffix(parent, suffixes)
}

// dirHasEntryWithSuffix reports whether dir directly contains a file whose name
// ends in any of the given suffixes. The result is intentionally not memoized:
// codeguard also runs as a long-lived MCP server, where a cached directory
// listing could serve a stale answer across scans, and the read is cheap next
// to reading and regexing every file the directory contains.
func dirHasEntryWithSuffix(dir string, suffixes []string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		for _, suffix := range suffixes {
			if len(name) > len(suffix) && strings.HasSuffix(name, suffix) {
				return true
			}
		}
	}
	return false
}
