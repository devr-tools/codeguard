package support_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// gitignoreRepo creates a real git working tree (so tracked-file lookups via
// `git ls-files` behave as in production) holding files, a map from
// slash-separated path to content.
func gitignoreRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	runIgnoreGit(t, root, "init", "-q")
	for name, content := range files {
		writeScanFile(t, root, filepath.FromSlash(name), content)
	}
	return root
}

func runIgnoreGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func walkRespectingGitignore(t *testing.T, root string, excludes []string) []string {
	t.Helper()
	files, err := runnersupport.WalkFilesWithOptions(root, excludes, runnersupport.FileWalkOptions{RespectGitignore: true}, func(string) bool { return true })
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return withoutGitignoreFiles(files)
}

// withoutGitignoreFiles drops the .gitignore files themselves, which are
// ordinary (scanned) files but would clutter every expectation below.
func withoutGitignoreFiles(files []string) []string {
	out := make([]string, 0, len(files))
	for _, file := range files {
		if filepath.Base(file) != ".gitignore" {
			out = append(out, filepath.ToSlash(file))
		}
	}
	sort.Strings(out)
	return out
}

func assertWalk(t *testing.T, got []string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if len(want) == 0 {
		want = []string{}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("walked files mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

// TestGitignoreWalkSemantics pins the gitignore(5) behaviours the walk
// implements. Every case also keeps at least one file that must survive: a
// matcher that over-matches silently drops real source from the scan, which
// is the failure a scanner can least afford.
func TestGitignoreWalkSemantics(t *testing.T) {
	// A file whose name ends in a space, ignored by the escaped "space\ "
	// pattern; the unescaped trailing space of other lines is trimmed.
	trailingSpaceName := "space" + " "
	cases := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{
			name: "unanchored name matches at any depth",
			files: map[string]string{
				".gitignore":         "*.log\nscratch.txt\n",
				"app.ts":             "x",
				"debug.log":          "x",
				"pkg/deep/trace.log": "x",
				"pkg/scratch.txt":    "x",
				"pkg/keep.txt":       "x",
			},
			want: []string{"app.ts", "pkg/keep.txt"},
		},
		{
			name: "leading slash anchors to the ignore file directory",
			files: map[string]string{
				".gitignore":        "/root-only.txt\n",
				"root-only.txt":     "x",
				"sub/root-only.txt": "x",
			},
			want: []string{"sub/root-only.txt"},
		},
		{
			name: "middle slash anchors too",
			files: map[string]string{
				".gitignore":      "docs/draft.md\n",
				"docs/draft.md":   "x",
				"x/docs/draft.md": "x",
				"docs/final.md":   "x",
			},
			want: []string{"docs/final.md", "x/docs/draft.md"},
		},
		{
			name: "negation re-includes a file excluded earlier",
			files: map[string]string{
				".gitignore":   "*.log\n!keep.log\n",
				"a.log":        "x",
				"keep.log":     "x",
				"sub/keep.log": "x",
			},
			want: []string{"keep.log", "sub/keep.log"},
		},
		{
			name: "last matching line wins",
			files: map[string]string{
				".gitignore": "!keep.log\n*.log\n",
				"keep.log":   "x",
				"main.go":    "x",
			},
			want: []string{"main.go"},
		},
		{
			name: "a file under an excluded directory cannot be re-included",
			files: map[string]string{
				".gitignore":    "out/\n!out/keep.txt\n",
				"out/keep.txt":  "x",
				"out/other.txt": "x",
				"src/main.go":   "x",
			},
			want: []string{"src/main.go"},
		},
		{
			name: "trailing slash matches directories only",
			files: map[string]string{
				".gitignore":         "generated/\n",
				"generated/api.ts":   "x",
				"pkg/generated/x.ts": "x",
				"lib/generated":      "a file, not a directory",
			},
			want: []string{"lib/generated"},
		},
		{
			name: "double-star forms",
			files: map[string]string{
				".gitignore":               "**/fixtures/*.json\nlogs/**\na/**/z.txt\n",
				"fixtures/one.json":        "x",
				"deep/x/fixtures/two.json": "x",
				"deep/x/fixtures/keep.ts":  "x",
				"logs/today/run.txt":       "x",
				"a/z.txt":                  "x",
				"a/b/c/z.txt":              "x",
				"a/b/c/y.txt":              "x",
			},
			want: []string{"a/b/c/y.txt", "deep/x/fixtures/keep.ts"},
		},
		{
			name: "brackets, escapes, and comments",
			files: map[string]string{
				".gitignore":      "# a comment line\nfile[0-9].tmp\n[!k]eep.txt\n\\#literal\nspace\\ \n",
				"file1.tmp":       "x",
				"fileA.tmp":       "x",
				"keep.txt":        "x",
				"deep.txt":        "x",
				"#literal":        "x",
				trailingSpaceName: "x",
				"space":           "x",
			},
			want: []string{"fileA.tmp", "keep.txt", "space"},
		},
		{
			name: "nested gitignore overrides its parent",
			files: map[string]string{
				".gitignore":               "*.txt\n",
				"top.txt":                  "x",
				"sub/.gitignore":           "!important.txt\n/local.md\n",
				"sub/important.txt":        "x",
				"sub/other.txt":            "x",
				"sub/local.md":             "x",
				"sub/deeper/local.md":      "x",
				"sub/deeper/important.txt": "x",
			},
			want: []string{"sub/deeper/important.txt", "sub/deeper/local.md", "sub/important.txt"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := gitignoreRepo(t, tc.files)
			assertWalk(t, walkRespectingGitignore(t, root, nil), tc.want...)
		})
	}
}

// TestGitignoreWalkInfoExclude covers the repository-local exclude file, which
// applies like a root .gitignore but with lower precedence than any
// .gitignore.
func TestGitignoreWalkInfoExclude(t *testing.T) {
	root := gitignoreRepo(t, map[string]string{
		".gitignore":     "!notes/keep.md\n",
		"notes/keep.md":  "x",
		"notes/draft.md": "x",
		"main.go":        "x",
	})
	writeScanFile(t, root, filepath.Join(".git", "info", "exclude"), "notes/*.md\n")
	assertWalk(t, walkRespectingGitignore(t, root, nil), "main.go", "notes/keep.md")
}

// TestGitignoreWalkScansTrackedIgnoredFiles pins the decision to follow git:
// an ignore rule does not apply to a file that is already tracked, so a
// committed file under an ignored directory is still scanned while its
// untracked siblings are not.
func TestGitignoreWalkScansTrackedIgnoredFiles(t *testing.T) {
	root := gitignoreRepo(t, map[string]string{
		".gitignore":         "build/\n*.gen.ts\n",
		"build/committed.js": "x",
		"build/output.js":    "x",
		"src/api.gen.ts":     "x",
		"src/other.gen.ts":   "x",
		"src/main.ts":        "x",
	})
	runIgnoreGit(t, root, "add", "-f", "build/committed.js", "src/api.gen.ts")
	assertWalk(t, walkRespectingGitignore(t, root, nil), "build/committed.js", "src/api.gen.ts", "src/main.ts")
}

// TestGitignoreWalkExplicitTargetWins covers a target or -folder that points
// into an ignored directory: the user asked for it, so it is scanned in full.
// Ignore rules still apply below a target that is not itself ignored.
func TestGitignoreWalkExplicitTargetWins(t *testing.T) {
	root := gitignoreRepo(t, map[string]string{
		".gitignore":                   "public/monaco/\n*.log\n",
		"public/index.ts":              "x",
		"public/monaco/vs/tsWorker.js": "x",
		"public/monaco/debug.log":      "x",
	})
	assertWalk(t, walkRespectingGitignore(t, filepath.Join(root, "public", "monaco"), nil), "debug.log", "vs/tsWorker.js")
	assertWalk(t, walkRespectingGitignore(t, filepath.Join(root, "public"), nil), "index.ts")

	// The same through -folder narrowing of a configured target.
	cfg := core.Config{Targets: []core.TargetConfig{{Name: "repo", Path: root}}}
	if err := runnersupport.ApplyTargetPath(&cfg, filepath.Join(root, "public", "monaco")); err != nil {
		t.Fatalf("ApplyTargetPath: %v", err)
	}
	files, err := runnersupport.ListTargetFiles(runnersupport.Context{Cfg: cfg}, cfg.Targets[0])
	if err != nil {
		t.Fatalf("ListTargetFiles: %v", err)
	}
	assertWalk(t, withoutGitignoreFiles(files), "debug.log", "vs/tsWorker.js")
}

// TestGitignoreWalkConfigSwitch covers respect_gitignore: unset means on, and
// false walks ignored paths as before the option existed.
func TestGitignoreWalkConfigSwitch(t *testing.T) {
	root := gitignoreRepo(t, map[string]string{
		".gitignore":  "bundle/\n",
		"bundle/a.js": "x",
		"src/b.ts":    "x",
	})
	target := core.TargetConfig{Name: "repo", Path: root}
	off := false
	on := true
	for _, tc := range []struct {
		name    string
		setting *bool
		want    []string
	}{
		{"default", nil, []string{"src/b.ts"}},
		{"explicit true", &on, []string{"src/b.ts"}},
		{"false", &off, []string{"bundle/a.js", "src/b.ts"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files, err := runnersupport.ListTargetFiles(runnersupport.Context{Cfg: core.Config{RespectGitignore: tc.setting}}, target)
			if err != nil {
				t.Fatalf("ListTargetFiles: %v", err)
			}
			assertWalk(t, withoutGitignoreFiles(files), tc.want...)
		})
	}
}

// TestGitignoreWalkComposesWithExcludes: a path is walked only when neither
// the ignore rules nor the configured excludes drop it, and a "!" exclude
// entry re-includes a path whichever of them excluded it.
func TestGitignoreWalkComposesWithExcludes(t *testing.T) {
	root := gitignoreRepo(t, map[string]string{
		".gitignore":        "gen/\n",
		"gen/keep/api.ts":   "x",
		"gen/drop/api.ts":   "x",
		"src/main.ts":       "x",
		"src/legacy/old.ts": "x",
	})
	got := walkRespectingGitignore(t, root, []string{"src/legacy/**", "!gen/keep/**"})
	assertWalk(t, got, "gen/keep/api.ts", "src/main.ts")
}

// TestGitignoreWalkNestedRepository: a nested work tree (submodule, vendored
// checkout) uses its own ignore rules, not its parent's.
func TestGitignoreWalkNestedRepository(t *testing.T) {
	root := gitignoreRepo(t, map[string]string{
		".gitignore": "*.md\n",
		"README.md":  "x",
		"main.go":    "x",
	})
	nested := filepath.Join(root, "third_party", "lib")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	runIgnoreGit(t, nested, "init", "-q")
	writeScanFile(t, nested, ".gitignore", "*.tmp\n")
	writeScanFile(t, nested, "NOTES.md", "x")
	writeScanFile(t, nested, "cache.tmp", "x")
	assertWalk(t, walkRespectingGitignore(t, root, nil), "main.go", "third_party/lib/NOTES.md")
}

// TestGitignoreWalkOutsideRepository: without a git work tree there are no
// ignore rules to honour, even if a stray .gitignore file exists.
func TestGitignoreWalkOutsideRepository(t *testing.T) {
	root := t.TempDir()
	writeScanFile(t, root, ".gitignore", "*.ts\n")
	writeScanFile(t, root, "a.ts", "x")
	assertWalk(t, walkRespectingGitignore(t, root, nil), "a.ts")
}
