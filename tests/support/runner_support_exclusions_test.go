package support_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// TestShouldExcludeDefaults pins the built-in scan exclusions.
//
// Two defects are locked out here. First, the defaults used to be globs
// anchored at the target root ("dist/**"), so dist/app.js was excluded but
// services/app/dist/app.js was scanned - every monorepo analyzed its own build
// output. Second, the common generated and dependency trees (.next, __pycache__,
// .venv, .terraform, tool caches) were not excluded at all; .next alone
// accounted for the overwhelming majority of findings on real Next.js
// repositories.
//
// The negative cases matter just as much: a scanner that silently skips real
// source reports a pass on code it never read, which is worse than reporting
// noise. Names that are sometimes legitimate source (build, out, bin, obj,
// target, coverage) must NOT be excluded on the name alone.
func TestShouldExcludeDefaults(t *testing.T) {
	cases := []struct {
		name string
		path string
		want bool
	}{
		// Depth independence: the original defect.
		{"dist at root", "dist/app.js", true},
		{"dist nested in a monorepo", "services/app/dist/app.js", true},
		{"dist deeply nested", "a/b/c/d/dist/chunk.js", true},
		{"git metadata", ".git/config", true},
		{"nested git submodule metadata", "third_party/lib/.git/config", true},
		{"codeguard state", ".codeguard/cache.json", true},
		{"nested gomodcache", "tools/.gomodcache/pkg/mod/x.go", true},

		// Dependency trees.
		{"node_modules", "node_modules/left-pad/index.js", true},
		{"nested node_modules", "web/node_modules/left-pad/index.js", true},
		{"bower_components", "static/bower_components/jquery/jquery.js", true},
		{"python site-packages", "lib/python3.11/site-packages/requests/api.py", true},
		{"dot venv", ".venv/lib/python3.11/site-packages/x.py", true},
		{"bare venv", "venv/bin/activate.py", true},
		{"cocoapods", "ios/Pods/Alamofire/Source/Alamofire.swift", true},
		{"carthage", "Carthage/Build/iOS/x.swift", true},
		{"dart pub cache", ".pub-cache/hosted/pub.dev/x.dart", true},
		{"vendor by default", "vendor/example.com/dep/dep.go", true},

		// JS/TS framework and bundler output.
		{"next output", ".next/server/pages/index.js", true},
		{"nested next output", "apps/web/.next/static/chunks/main.js", true},
		{"nuxt output", ".nuxt/dist/server.mjs", true},
		{"svelte-kit output", ".svelte-kit/output/client/app.js", true},
		{"angular cache", ".angular/cache/17.0.0/vite/x.js", true},
		{"astro output", ".astro/types.d.ts", true},
		{"docusaurus output", ".docusaurus/registry.js", true},
		{"nitro output", ".output/server/index.mjs", true},
		{"vercel output", ".vercel/output/functions/index.func/index.js", true},
		{"netlify output", ".netlify/functions-internal/x.js", true},
		{"turbo cache", ".turbo/cache/abc123/out.js", true},
		{"parcel cache", ".parcel-cache/abc.txt", true},
		{"vite cache", ".vite/deps/react.js", true},
		{"nyc output", ".nyc_output/out.json", true},

		// Tool caches. ".cache" is the far more common name that ".gocache" was
		// standing in for; a Go repo with a .build/go-cache tree produced 566
		// security findings against compiled build-cache objects.
		{"generic cache", ".cache/go-build/ab/abcdef", true},
		{"nested generic cache", "sub/.cache/go-build/ab/abcdef", true},
		{"swiftpm build cache", ".build/go-cache/ab/abcdef", true},
		{"mypy cache", ".mypy_cache/3.11/x.json", true},
		{"pytest cache", ".pytest_cache/v/cache/nodeids", true},
		{"ruff cache", ".ruff_cache/0.1.0/x", true},
		{"tox", ".tox/py311/lib/x.py", true},
		{"nox", ".nox/tests/lib/x.py", true},
		{"gradle cache", ".gradle/caches/modules-2/x.jar", true},
		{"terraform modules", ".terraform/modules/vpc/main.tf", true},
		{"nested terraform modules", "infra/prod/.terraform/providers/x", true},
		{"dart tool", ".dart_tool/package_config.json", true},
		{"sass cache", ".sass-cache/x.scssc", true},
		{"python eggs", ".eggs/x.egg/x.py", true},
		{"notebook checkpoints", "notebooks/.ipynb_checkpoints/x.ipynb", true},
		{"python bytecode", "pkg/__pycache__/mod.cpython-311.pyc", true},
		{"egg-info metadata", "src/mypkg.egg-info/PKG-INFO", true},
		{"coverage html report", "htmlcov/index.html", true},
		{"cdk synth output", "infrastructure/cdk.out/asset/index.js", true},

		// Generated files, which the binary guard cannot catch because they are
		// text. Only compound suffixes are listed: a bare ".map" could be a
		// data file, while ".js.map" is a source map and nothing else.
		{"minified js", "static/js/app.min.js", true},
		{"minified css", "static/css/app.min.css", true},
		{"webpack bundle", "public/vendor.bundle.js", true},
		{"js source map", "public/app.js.map", true},
		{"css source map", "public/app.css.map", true},

		// Names that are sometimes real source: never excluded by name alone.
		// walkFilesBounded skips these only with a corroborating sibling
		// manifest (see TestWalkFilesRequiresCorroborationForAmbiguousBuildDirs).
		{"go package named build", "internal/build/version.go", false},
		{"rust binary crate", "src/bin/server.rs", false},
		{"out directory", "out/index.js", false},
		{"cargo target name", "target/debug/app.rs", false},
		{"dotnet obj name", "obj/Debug/x.cs", false},
		{"coverage source package", "coverage/control.py", false},

		// Directories that must never be excluded: checks read them.
		{"github workflows", ".github/workflows/ci.yml", false},
		{"agent context", ".claude/knowledge/x.md", false},
		{"editor config", ".vscode/settings.json", false},
		{"ordinary source", "internal/codeguard/runner/support/utils.go", false},
		{"testdata", "tests/corpus/testdata/go/x.go", false},
		// Partial segment matches must not fire.
		{"distribution is not dist", "distribution/app.js", false},
		{"venv prefix is not venv", "venvtools/main.py", false},
		{"next suffix is not .next", "my.next/app.js", false},
		{"map is not a source map", "data/regions.map", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := runnersupport.ShouldExclude(tc.path, nil); got != tc.want {
				t.Fatalf("ShouldExclude(%q, nil) = %t, want %t", tc.path, got, tc.want)
			}
		})
	}
}

// TestShouldExcludeReinclude covers the escape hatch. The built-in default list
// is not user-editable, so a repository whose real source lives under an
// excluded name needs a way to opt back in; a "!" entry in the exclude list
// re-includes matching paths and wins over the defaults.
func TestShouldExcludeReinclude(t *testing.T) {
	excludes := []string{"!packages/ui/dist/**", "generated/**"}
	cases := []struct {
		path string
		want bool
	}{
		{"packages/ui/dist/index.ts", false},  // re-included
		{"packages/api/dist/index.js", true},  // still a default exclusion
		{"generated/schema.go", true},         // ordinary user exclusion
		{"packages/ui/src/button.tsx", false}, // never excluded
	}
	for _, tc := range cases {
		if got := runnersupport.ShouldExclude(tc.path, excludes); got != tc.want {
			t.Fatalf("ShouldExclude(%q, %v) = %t, want %t", tc.path, excludes, got, tc.want)
		}
	}
}

// TestWalkFilesSkipsNestedGeneratedTrees is the end-to-end form of the depth
// independence fix: a walk of a monorepo must return owned source only.
func TestWalkFilesSkipsNestedGeneratedTrees(t *testing.T) {
	root := writeTree(t, map[string]string{
		"apps/web/app/page.tsx":                        "export default function Page() {}\n",
		"apps/web/.next/static/chunks/main.js":         "self.webpackChunk=[];\n",
		"apps/web/dist/bundle.js":                      "var a=1;\n",
		"services/api/main.go":                         "package main\n",
		"services/api/dist/server":                     "compiled\n",
		"services/api/__pycache__/mod.cpython-311.pyc": "bytecode\n",
		"infra/.terraform/providers/aws/plugin":        "plugin\n",
		"tools/.cache/go-build/ab/abcdef":              "object code\n",
		"web/public/app.min.js":                        "var a=1;\n",
		"web/public/app.js.map":                        "{\"version\":3}\n",
	})

	got, err := runnersupport.WalkFiles(root, nil, func(string) bool { return true })
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	sort.Strings(got)
	want := []string{"apps/web/app/page.tsx", "services/api/main.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("walked files = %v, want only owned source %v", got, want)
	}
}

// TestWalkFilesRequiresCorroborationForAmbiguousBuildDirs is the other half of
// the judgement call: build, out, bin, obj, target and coverage are build output
// in most repositories and real source in some, so the walk skips one only when
// a sibling manifest proves which it is. Without that signal the directory is
// scanned, because silently dropping real source is the worse failure.
func TestWalkFilesRequiresCorroborationForAmbiguousBuildDirs(t *testing.T) {
	cases := []struct {
		name     string
		files    map[string]string
		wantPath string
		want     bool
	}{
		{
			name: "cargo target is build output",
			files: map[string]string{
				"Cargo.toml":          "[package]\n",
				"target/debug/app.rs": "fn main() {}\n",
			},
			wantPath: "target/debug/app.rs",
			want:     false,
		},
		{
			name: "target without a manifest stays scanned",
			files: map[string]string{
				"go.mod":            "module example.com/app\n",
				"target/handler.go": "package target\n",
			},
			wantPath: "target/handler.go",
			want:     true,
		},
		{
			name: "gradle build is build output",
			files: map[string]string{
				"build.gradle":            "plugins {}\n",
				"build/classes/Main.java": "class Main {}\n",
			},
			wantPath: "build/classes/Main.java",
			want:     false,
		},
		{
			name: "go package named build stays scanned",
			files: map[string]string{
				"go.mod":                    "module example.com/app\n",
				"internal/build/version.go": "package build\n",
			},
			wantPath: "internal/build/version.go",
			want:     true,
		},
		{
			name: "dotnet bin and obj are build output",
			files: map[string]string{
				"App.csproj":           "<Project/>\n",
				"bin/Debug/App.dll.cs": "// generated\n",
				"obj/Debug/App.g.cs":   "// generated\n",
				"Program.cs":           "class Program {}\n",
			},
			wantPath: "bin/Debug/App.dll.cs",
			want:     false,
		},
		{
			name: "rust src/bin stays scanned",
			files: map[string]string{
				"Cargo.toml":     "[package]\n",
				"src/bin/cli.rs": "fn main() {}\n",
			},
			wantPath: "src/bin/cli.rs",
			want:     true,
		},
		{
			name: "jest coverage output is build output",
			files: map[string]string{
				"package.json":       "{}\n",
				"coverage/lcov.info": "TN:\n",
			},
			wantPath: "coverage/lcov.info",
			want:     false,
		},
		{
			// coverage.py's own repository keeps a coverage/ *source* package
			// next to its pyproject.toml, which is why Python markers are
			// deliberately absent from the coverage entry.
			name: "python coverage source package stays scanned",
			files: map[string]string{
				"pyproject.toml":      "[project]\n",
				"coverage/control.py": "class Coverage: pass\n",
			},
			wantPath: "coverage/control.py",
			want:     true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeTree(t, tc.files)
			got, err := runnersupport.WalkFiles(root, nil, func(string) bool { return true })
			if err != nil {
				t.Fatalf("walk: %v", err)
			}
			found := false
			for _, file := range got {
				if file == tc.wantPath {
					found = true
				}
			}
			if found != tc.want {
				t.Fatalf("walk included %q = %t, want %t (walked %v)", tc.wantPath, found, tc.want, got)
			}
		})
	}
}

// TestWalkFilesReincludeOverridesDefaultExclusion verifies the escape hatch end
// to end: the built-in list is not user-editable, so a "!" exclude entry must be
// able to bring a default-excluded tree back into the walk.
func TestWalkFilesReincludeOverridesDefaultExclusion(t *testing.T) {
	root := writeTree(t, map[string]string{
		"packages/ui/src/button.tsx": "export const Button = () => null;\n",
		"packages/ui/dist/index.ts":  "export * from './button';\n",
		"packages/api/dist/index.js": "var a=1;\n",
	})

	got, err := runnersupport.WalkFiles(root, []string{"!packages/ui/dist/**"}, func(string) bool { return true })
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	sort.Strings(got)
	want := []string{"packages/ui/dist/index.ts", "packages/ui/src/button.tsx"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("walked files = %v, want %v", got, want)
	}
}

// TestLooksBinary pins the shared binary classifier that now guards every
// per-file findings scan, not just the secrets pass.
func TestLooksBinary(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"empty", nil, false},
		{"source", []byte("package main\n\nfunc main() {}\n"), false},
		{"utf8 text", []byte("// café ☕\n"), false},
		{"object code", []byte("\x7fELF\x02\x01\x01\x00\x00\x00md5.New()"), true},
		{"nul late in the window", append(bytes.Repeat([]byte("a"), 4096), 0x00), true},
		// The sniff window is bounded at 8 KiB, so a NUL past it is not seen.
		{"nul beyond the sniff window", append(bytes.Repeat([]byte("a"), 8<<10), 0x00), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := runnersupport.LooksBinary(tc.data); got != tc.want {
				t.Fatalf("LooksBinary(%q...) = %t, want %t", tc.data[:min(len(tc.data), 8)], got, tc.want)
			}
		})
	}
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return root
}
