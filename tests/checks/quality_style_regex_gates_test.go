package checks_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/quality"
)

// The style-profile regexes in checks/quality run over whole files, and the
// ones without a literal prefix sit behind strings.Contains pre-gates. A gate
// that is not a superset of its pattern silently zeroes a style count, so
// these tests compare every gated result against an ungated oracle compiled
// from the production expression itself (StyleRegexExprsForTest), over every
// .go file in the repository plus hand-built edge cases.

type styleOracle struct {
	pkgErrors, errorfWrap, errorf, errorsNew     *regexp.Regexp
	goFunc, pythonDef, scriptFunc, scriptBinding *regexp.Regexp
}

// originalGoFuncNameExpr is the whole-file pattern goDeclaredNames ran before
// its positional gate. The production code now holds only the ^-anchored form
// it tries at line starts, so the oracle rebuilds this one from it and the test
// pins that the rebuild is exactly the original.
const originalGoFuncNameExpr = `(?m)^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][\w]*)\s*\(`

func newStyleOracle(t testing.TB) styleOracle {
	t.Helper()
	exprs := quality.StyleRegexExprsForTest()
	compile := func(name string) *regexp.Regexp {
		expr, ok := exprs[name]
		if !ok {
			t.Fatalf("StyleRegexExprsForTest has no %s", name)
		}
		return regexp.MustCompile(expr)
	}
	if got := "(?m)" + exprs["goFuncNameAtStartPattern"]; got != originalGoFuncNameExpr {
		t.Fatalf("goFuncNameAtStartPattern under (?m) = %s, want the original %s", got, originalGoFuncNameExpr)
	}
	return styleOracle{
		pkgErrors:     compile("goPkgErrorsPattern"),
		errorfWrap:    compile("goErrorfWrapPattern"),
		errorf:        compile("goErrorfPattern"),
		errorsNew:     compile("goErrorsNewPattern"),
		goFunc:        regexp.MustCompile(originalGoFuncNameExpr),
		pythonDef:     compile("pythonDefNamePattern"),
		scriptFunc:    compile("scriptFuncNamePattern"),
		scriptBinding: compile("scriptBindingNamePattern"),
	}
}

// errorStyleCounts is goErrorStyleCounts with no gates.
func (o styleOracle) errorStyleCounts(source string) map[string]int {
	counts := map[string]int{}
	wraps := len(o.errorfWrap.FindAllString(source, -1))
	if wraps > 0 {
		counts["fmt.Errorf with %w wrapping"] = wraps
	}
	plain := len(o.errorf.FindAllString(source, -1)) - wraps + len(o.errorsNew.FindAllString(source, -1))
	if plain > 0 {
		counts["errors.New / unwrapped fmt.Errorf"] = plain
	}
	if pkg := len(o.pkgErrors.FindAllString(source, -1)); pkg > 0 {
		counts["github.com/pkg/errors wrapping"] = pkg
	}
	return counts
}

func oracleNames(source string, patterns ...*regexp.Regexp) []quality.DeclaredNameForTest {
	var out []quality.DeclaredNameForTest
	for _, pattern := range patterns {
		for _, match := range pattern.FindAllStringSubmatchIndex(source, -1) {
			out = append(out, quality.DeclaredNameForTest{
				Name: source[match[2]:match[3]],
				Line: 1 + strings.Count(source[:match[2]], "\n"),
			})
		}
	}
	return out
}

func (o styleOracle) assertEquivalent(t *testing.T, label string, source string) {
	t.Helper()
	if got, want := quality.GoErrorStyleCountsForTest(source), o.errorStyleCounts(source); !reflect.DeepEqual(got, want) {
		t.Errorf("%s: error style counts = %v, ungated = %v", label, got, want)
	}
	for _, tc := range []struct {
		language string
		want     []quality.DeclaredNameForTest
	}{
		{"go", oracleNames(source, o.goFunc)},
		{"python", oracleNames(source, o.pythonDef)},
		{"script", oracleNames(source, o.scriptFunc, o.scriptBinding)},
	} {
		if got := quality.DeclaredNamesForTest(tc.language, source); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %s declared names = %v, ungated = %v", label, tc.language, got, tc.want)
		}
	}
}

// repoGoSources returns every .go file in the repository, skipping VCS
// metadata and nested worktrees (which would only duplicate the tree).
func repoGoSources(t testing.TB) map[string]string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Dir(filepath.Dir(wd))
	sources := map[string]string{}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".claude", "worktrees", "node_modules", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		sources[path] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if len(sources) < 100 {
		t.Fatalf("found only %d .go files under %s; corpus walk is broken", len(sources), root)
	}
	return sources
}

func TestStyleRegexGatesMatchUngatedOnRepoGoFiles(t *testing.T) {
	oracle := newStyleOracle(t)
	pkgErrorsHits := 0
	for path, source := range repoGoSources(t) {
		oracle.assertEquivalent(t, path, source)
		pkgErrorsHits += len(oracle.pkgErrors.FindAllString(source, -1))
	}
	// The repository documents and tests pkg/errors detection, so the corpus
	// must exercise the admitted side of the gate, not only the rejected one.
	if pkgErrorsHits == 0 {
		t.Fatalf("no goPkgErrorsPattern match anywhere in the repository; the corpus no longer exercises the gate")
	}
}

func TestStyleRegexGatesMatchUngatedOnEdgeCases(t *testing.T) {
	oracle := newStyleOracle(t)
	cases := []string{
		"",
		"errors.",
		"errors.Wrap",
		"errors.Wrap(",
		"errors.Wrapf(err, \"x\")",
		"errors.WithStack(",
		"errors.WithMessage(",
		"errors.WithMessagef(",
		"errors.WithStack",
		"xerrors.Wrap(err)",
		"pkgerrors.Wrap(err)",
		"(errors.Wrap(err)",
		"\terrors.WithStack(err)\n",
		"import \"github.com/pkg/errors\"",
		"github.com/pkg/errors",
		"github.com/pkg/errorsx",
		"github.com/pkg/errors/v2",
		"xgithub.com/pkg/errors",
		"GITHUB.COM/PKG/ERRORS",
		"ERRORS.WRAP(",
		"errors . Wrap(",
		"errors.New(\"a\") fmt.Errorf(\"b %w\", err) fmt.Errorf(\"c\")",
		"errors.Wrap(errors.Wrap(errors.WithStack(x)))",
		"éerrors.Wrap(err)é",
		"func main() {}",
		"func (s *S) Method() {}\nfunc helper_name() {}\nfunc otherName() {}",
		"func\n(r recv)\nmultiLine\n()",
		"  func indented() {}",
		"funcx() {}",
		"func",
		"func\n",
		"\nfunc",
		"\n\nfunc a(",
		"func a(func b(",
		"func (\nfunc inner) outer(\nfunc last(",
		// The receiver group spans a line that would match on its own; FindAll
		// resumes after the outer match, so "inner" must not be reported.
		"func (\nfunc inner(x) outer(",
		"func (r R\nfunc x()) y()\nfunc z()",
		"func  \n\n  spaced \n (",
		"func a() {}\r\nfunc b_c() {}\r\n",
		"x func a()\nfunc b()",
		"func 1bad()\nfunc good()",
		"func (r *R) m() {}\nfunc (\nr *R,\n) n() {}",
		"def snake_case():\n\tdef camelCase():\n",
		"define x():",
		"function camelName() {}\nconst snake_name = 1\nlet otherName = 2\nexport var v_x = 3",
		"constant = 1\nletter = 2\nvariable = 3",
		"const a\n= 1",
		"functions are fun",
		"\xff\xfeerrors.Wrap(\xff",
	}
	for _, source := range cases {
		oracle.assertEquivalent(t, strings.ReplaceAll(source, "\n", `\n`), source)
	}
}

// BenchmarkGoRepoStyleProfile times goRepoStyleProfile's summation over every
// .go file in the repository (file reads excluded).
func BenchmarkGoRepoStyleProfile(b *testing.B) {
	sourceMap := repoGoSources(b)
	sources := make([]string, 0, len(sourceMap))
	for _, source := range sourceMap {
		sources = append(sources, source)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		quality.GoRepoStyleProfileForTest(sources)
	}
}
