package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// contextGoMakeFixture is a Go module driven by a Makefile and a CI workflow.
func contextGoMakeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		"go.mod":                     "module example.com/gomake\n\ngo 1.23\n",
		"Makefile":                   ".PHONY: build test lint fmt\n\nbuild:\n\tgo build ./...\n\ntest:\n\tgo test ./...\n\nlint:\n\tgo vet ./...\n\nfmt:\n\tgofmt -w .\n\ndist/gomake: build\n",
		"cmd/gomake/main.go":         "package main\n\nfunc main() {}\n",
		"internal/calc/calc.go":      "package calc\n\n// Add returns a + b.\nfunc Add(a, b int) int { return a + b }\n",
		"internal/calc/calc_test.go": "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) { _ = Add(1, 2) }\n",
		"misc/notes.txt":             "scratch\n",
		".golangci.yml":              "version: \"2\"\n",
		".github/workflows/ci.yml":   "name: ci\non: [push]\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make test\n      - run: make deploy\n      - run: echo ${{ github.sha }}\n      - run: go test -race ./...\n",
	})
	return root
}

// contextNodeFixture is a Node package with scripts and a pnpm lockfile.
func contextNodeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		"package.json":         `{"name":"nodeapp","main":"src/index.js","scripts":{"build":"tsc -p .","test":"vitest run","lint":"eslint .","start":"node ./dist/index.js"},"prettier":{}}`,
		"pnpm-lock.yaml":       "lockfileVersion: '9.0'\n",
		"src/index.js":         "module.exports = {};\n",
		"src/index.test.js":    "test('ok', () => {});\n",
		"eslint.config.js":     "module.exports = [];\n",
		"docs/guide.md":        "# Guide\n",
		"docs/reference.md":    "# Reference\n",
		"tests/fixtures/x.js":  "x\n",
		"tests/app.spec.js":    "test('a', () => {});\n",
		"tests/fixtures/y.js":  "y\n",
		"tests/helpers/h.js":   "h\n",
		"tests/fixtures/z.txt": "z\n",
	})
	return root
}

// contextPythonFixture is a pyproject-managed Python package using uv.
func contextPythonFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		"pyproject.toml":           "[project]\nname = \"pyapp\"\n\n[project.scripts]\npyapp = \"pyapp.cli:main\"\n\n[tool.pytest.ini_options]\ntestpaths = [\"tests\"]\n\n[tool.ruff]\nline-length = 100\n",
		"uv.lock":                  "version = 1\n",
		"src/pyapp/__init__.py":    "",
		"src/pyapp/cli.py":         "def main():\n    return 0\n",
		"tests/test_cli.py":        "from pyapp.cli import main\n\n\ndef test_main():\n    assert main() == 0\n",
		"tests/conftest.py":        "",
		"tests/testdata/test_x.py": "fixture = 1\n",
	})
	return root
}

func runContextCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := cli.Run(append([]string{"context"}, args...), strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func readContextFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

var inlineCommandPattern = regexp.MustCompile("(?m)^- `([^`]+)`$")

// commandsBlock returns the bullet commands listed in the managed commands
// block.
func commandsBlock(t *testing.T, doc string) []string {
	t.Helper()
	start := strings.Index(doc, "<!-- codeguard:begin commands -->")
	end := strings.Index(doc, "<!-- codeguard:end commands -->")
	if start < 0 || end < start {
		t.Fatalf("commands block missing:\n%s", doc)
	}
	var commands []string
	for _, match := range inlineCommandPattern.FindAllStringSubmatch(doc[start:end], -1) {
		commands = append(commands, match[1])
	}
	return commands
}

func assertSameCommands(t *testing.T, got []string, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("commands = %q\nwant      %q", got, want)
	}
}

// assertNoFreeformProse checks the no-fabrication contract on the parts
// of a generated doc that are not fixed structure: every bullet either cites
// detected content or is a TODO.
func assertNoFreeformProse(t *testing.T, doc string) {
	t.Helper()
	for _, line := range strings.Split(doc, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "", strings.HasPrefix(trimmed, "#"), strings.HasPrefix(trimmed, "<!--"),
			strings.HasPrefix(trimmed, "- "), strings.HasPrefix(trimmed, "TODO:"),
			strings.HasSuffix(trimmed, ":"), trimmed == "Run from the repository root.":
		default:
			t.Errorf("generated line is neither structure, a detected fact bullet nor a TODO: %q", line)
		}
	}
}

func TestContextInitGoMakefileFixture(t *testing.T) {
	root := contextGoMakeFixture(t)
	code, stdout, stderr := runContextCLI(t, "init", "-path", root)
	if code != 0 {
		t.Fatalf("context init exit = %d, stderr:\n%s", code, stderr)
	}
	doc := readContextFile(t, filepath.Join(root, "AGENTS.md"))
	assertSameCommands(t, commandsBlock(t, doc), []string{
		"make build", "make test", "make lint", "make fmt",
		"go build ./...", "go vet ./...", "go test ./...",
		"make test", "go test -race ./...",
	})
	for _, want := range []string{
		"# gomake\n",
		"- `cmd/gomake/` (Go)",
		"- `cmd/`: Go main packages, one binary per subdirectory (1 Go file).",
		"- `misc/`: TODO: describe this directory (1 file).",
		"- `internal/`: 1 test file (Go).",
		"- golangci-lint: configured in `.golangci.yml`.",
		"- Go language version: 1.23 (`go.mod` go directive).",
		"CI steps in `.github/workflows/ci.yml`:",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("generated AGENTS.md missing %q:\n%s", want, doc)
		}
	}
	for _, banned := range []string{"make deploy", "github.sha", "dist/gomake", "npm", "pytest", "cargo"} {
		if strings.Contains(doc, banned) {
			t.Errorf("generated AGENTS.md invents or leaks %q:\n%s", banned, doc)
		}
	}
	assertNoFreeformProse(t, doc)
	if !strings.Contains(stdout, "agent-context check: legibility score") || !strings.Contains(stdout, "no agent-context findings") {
		t.Fatalf("init did not report a clean agent-context check:\n%s", stdout)
	}
}

func TestContextInitNodeFixture(t *testing.T) {
	root := contextNodeFixture(t)
	code, _, stderr := runContextCLI(t, "init", "-path", root, "-file", "CLAUDE.md")
	if code != 0 {
		t.Fatalf("context init exit = %d, stderr:\n%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); err == nil {
		t.Fatal("-file CLAUDE.md also wrote AGENTS.md")
	}
	doc := readContextFile(t, filepath.Join(root, "CLAUDE.md"))
	assertSameCommands(t, commandsBlock(t, doc), []string{
		"pnpm run build", "pnpm run lint", "pnpm run start", "pnpm run test",
	})
	for _, want := range []string{
		"# nodeapp\n",
		"- `src/index.js` (JavaScript)",
		"- `docs/`: Documentation (2 files).",
		"- ESLint: configured in `eslint.config.js`.",
		"- Prettier: configured in `package.json` (\"prettier\" key).",
		"- `src/`: 1 test file (JavaScript).",
		"- `tests/`: 1 test file (JavaScript).",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("generated CLAUDE.md missing %q:\n%s", want, doc)
		}
	}
	for _, banned := range []string{"make ", "go build", "dist/index.js", "`npm run"} {
		if strings.Contains(doc, banned) {
			t.Errorf("generated CLAUDE.md invents or leaks %q:\n%s", banned, doc)
		}
	}
	assertNoFreeformProse(t, doc)
}

func TestContextInitPythonFixture(t *testing.T) {
	root := contextPythonFixture(t)
	code, stdout, stderr := runContextCLI(t, "init", "-path", root, "-dry-run")
	if code != 0 {
		t.Fatalf("context init -dry-run exit = %d, stderr:\n%s", code, stderr)
	}
	assertSameCommands(t, commandsBlock(t, stdout), []string{"uv run pytest", "uv run ruff check ."})
	for _, want := range []string{
		"# pyapp\n",
		"- `src/pyapp/cli.py` (Python)",
		"- `tests/`: 1 test file (Python).",
		"- Ruff: configured in `pyproject.toml` [tool.ruff].",
		"- pytest: configured in `pyproject.toml` [tool.pytest].",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("generated doc missing %q:\n%s", want, stdout)
		}
	}
	for _, banned := range []string{"black", "mypy", "make ", "poetry"} {
		if strings.Contains(stdout, banned) {
			t.Errorf("generated doc invents %q:\n%s", banned, stdout)
		}
	}
	assertNoFreeformProse(t, stdout)
}

func TestContextInitDryRunWritesNothing(t *testing.T) {
	root := contextGoMakeFixture(t)
	code, stdout, _ := runContextCLI(t, "init", "-path", root, "-dry-run")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.HasPrefix(stdout, "# gomake\n") || strings.Contains(stdout, "agent-context check") {
		t.Fatalf("-dry-run stdout should be exactly the document:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("-dry-run wrote AGENTS.md (stat err %v)", err)
	}
}

func TestContextInitRefusesOverwriteWithoutForce(t *testing.T) {
	root := contextGoMakeFixture(t)
	path := filepath.Join(root, "AGENTS.md")
	writeFixture(t, root, map[string]string{"AGENTS.md": "# Mine\n\nRun `make test`, `make build`, `make lint` and `make fmt`.\n"})
	code, _, stderr := runContextCLI(t, "init", "-path", root)
	if code == 0 || !strings.Contains(stderr, "already exists") || !strings.Contains(stderr, "-force") {
		t.Fatalf("init over an existing file: exit %d, stderr:\n%s", code, stderr)
	}
	if got := readContextFile(t, path); !strings.HasPrefix(got, "# Mine") {
		t.Fatalf("existing file was modified without -force:\n%s", got)
	}
	code, _, stderr = runContextCLI(t, "init", "-path", root, "-force")
	if code != 0 {
		t.Fatalf("init -force exit = %d, stderr:\n%s", code, stderr)
	}
	if got := readContextFile(t, path); !strings.HasPrefix(got, "# gomake") {
		t.Fatalf("-force did not overwrite:\n%s", got)
	}
}

func TestContextUpdateReplacesOnlyManagedBlocks(t *testing.T) {
	root := contextGoMakeFixture(t)
	path := filepath.Join(root, "AGENTS.md")
	before := "# Hand-written title\r\n\nSome *human* prose, with trailing spaces.   \n\n" +
		"<!-- codeguard:begin commands -->\n- `make stale`\n<!-- codeguard:end commands -->\n\n" +
		"Middle text the human owns.\n\n" +
		"<!-- codeguard:begin custom -->\nleave me alone\n<!-- codeguard:end custom -->\n\n" +
		"  <!-- codeguard:begin tests -->\nold tests line\n  <!-- codeguard:end tests -->\nTrailing text without newline"
	writeFixture(t, root, map[string]string{"AGENTS.md": before})
	code, stdout, stderr := runContextCLI(t, "update", "-path", root)
	if code != 0 {
		t.Fatalf("update exit = %d, stderr:\n%s", code, stderr)
	}
	after := readContextFile(t, path)
	facts, err := codeguard.DetectContextFacts(root)
	if err != nil {
		t.Fatal(err)
	}
	generated := codeguard.RenderContextDoc(facts)
	commandsBody := between(t, generated, "<!-- codeguard:begin commands -->\n", "<!-- codeguard:end commands -->")
	testsBody := between(t, generated, "<!-- codeguard:begin tests -->\n", "<!-- codeguard:end tests -->")
	want := "# Hand-written title\r\n\nSome *human* prose, with trailing spaces.   \n\n" +
		"<!-- codeguard:begin commands -->\n" + commandsBody + "<!-- codeguard:end commands -->\n\n" +
		"Middle text the human owns.\n\n" +
		"<!-- codeguard:begin custom -->\nleave me alone\n<!-- codeguard:end custom -->\n\n" +
		"  <!-- codeguard:begin tests -->\n" + testsBody + "  <!-- codeguard:end tests -->\nTrailing text without newline"
	if after != want {
		t.Fatalf("update result mismatch\n--- got ---\n%q\n--- want ---\n%q", after, want)
	}
	if !strings.Contains(stdout, "updated ") || !strings.Contains(stdout, "commands, tests") {
		t.Errorf("update stdout should name the changed blocks:\n%s", stdout)
	}
	if !strings.Contains(stderr, "custom") || !strings.Contains(stderr, "overview") {
		t.Errorf("update stderr should note unknown and missing blocks:\n%s", stderr)
	}
	code, stdout, _ = runContextCLI(t, "update", "-path", root)
	if code != 0 || !strings.Contains(stdout, "is up to date") {
		t.Fatalf("second update should be a no-op: exit %d\n%s", code, stdout)
	}
}

func TestContextUpdateWithoutMarkersShowsDiffAndDoesNotWrite(t *testing.T) {
	root := contextGoMakeFixture(t)
	path := filepath.Join(root, "CLAUDE.md")
	before := "# Notes\n\nHand-written only.\n"
	writeFixture(t, root, map[string]string{"CLAUDE.md": before})
	code, stdout, stderr := runContextCLI(t, "update", "-path", root)
	if code == 0 {
		t.Fatal("update of a marker-less file should exit non-zero")
	}
	if got := readContextFile(t, path); got != before {
		t.Fatalf("marker-less file was rewritten:\n%s", got)
	}
	for _, want := range []string{"--- a/CLAUDE.md", "+++ b/CLAUDE.md", "+<!-- codeguard:begin commands -->", "+- `make test`"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("diff missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "-# Notes") || strings.Contains(stdout, "-Hand-written only.") {
		t.Errorf("proposal diff removes human text:\n%s", stdout)
	}
	if !strings.Contains(stderr, "no codeguard-managed blocks") || !strings.Contains(stderr, "codeguard:begin") {
		t.Errorf("stderr should ask for markers:\n%s", stderr)
	}
}

func TestContextUpdateDryRunPrintsDiffOnly(t *testing.T) {
	root := contextGoMakeFixture(t)
	path := filepath.Join(root, "AGENTS.md")
	before := "<!-- codeguard:begin commands -->\n- `make stale`\n<!-- codeguard:end commands -->\n"
	writeFixture(t, root, map[string]string{"AGENTS.md": before})
	code, stdout, _ := runContextCLI(t, "update", "-path", root, "-dry-run")
	if code != 0 || !strings.Contains(stdout, "-- `make stale`") || !strings.Contains(stdout, "+- `make test`") {
		t.Fatalf("dry-run diff unexpected (exit %d):\n%s", code, stdout)
	}
	if got := readContextFile(t, path); got != before {
		t.Fatalf("-dry-run wrote the file:\n%s", got)
	}
}

func TestContextUpdateRejectsMalformedMarkers(t *testing.T) {
	root := contextGoMakeFixture(t)
	before := "<!-- codeguard:begin commands -->\nnever closed\n"
	writeFixture(t, root, map[string]string{"AGENTS.md": before})
	code, _, stderr := runContextCLI(t, "update", "-path", root)
	if code == 0 || !strings.Contains(stderr, "never closed") {
		t.Fatalf("malformed markers: exit %d, stderr:\n%s", code, stderr)
	}
	if got := readContextFile(t, filepath.Join(root, "AGENTS.md")); got != before {
		t.Fatalf("malformed file was rewritten:\n%s", got)
	}
}

// TestGeneratedContextDocsScoreClean is the generator's contract with the
// lint side: a freshly generated file triggers no agent-context finding about
// itself (missing files, undocumented commands, link rot, size).
func TestGeneratedContextDocsScoreClean(t *testing.T) {
	fixtures := map[string]func(*testing.T) string{
		"go-make": contextGoMakeFixture,
		"node":    contextNodeFixture,
		"python":  contextPythonFixture,
	}
	for name, build := range fixtures {
		t.Run(name, func(t *testing.T) {
			root := build(t)
			facts, err := codeguard.DetectContextFacts(root)
			if err != nil {
				t.Fatal(err)
			}
			writeFixture(t, root, map[string]string{"AGENTS.md": codeguard.RenderContextDoc(facts)})
			result, err := codeguard.CheckContextDocs(context.Background(), root, codeguard.ContextRulesConfig{})
			if err != nil {
				t.Fatal(err)
			}
			for _, finding := range result.Findings {
				t.Errorf("generated doc produced finding %s: %s", finding.RuleID, finding.Message)
			}
			if result.Score < 85 {
				t.Errorf("legibility score = %d, want >= 85 (%+v)", result.Score, result.Components)
			}
		})
	}
}

// TestContextCheckFlagsUndocumentedCommandBeforeGeneration pins that the
// clean result above is earned: the same fixture with a hand-written doc that
// omits the canonical targets does trigger context.undocumented-commands.
func TestContextCheckFlagsUndocumentedCommandBeforeGeneration(t *testing.T) {
	root := contextGoMakeFixture(t)
	writeFixture(t, root, map[string]string{"AGENTS.md": "# Notes\n\nNothing about commands.\n"})
	result, err := codeguard.CheckContextDocs(context.Background(), root, codeguard.ContextRulesConfig{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, finding := range result.Findings {
		found = found || finding.RuleID == "context.undocumented-commands"
	}
	if !found {
		t.Fatalf("expected context.undocumented-commands on a doc missing the Makefile targets, got %+v", result.Findings)
	}
}

func TestContextCommandUsage(t *testing.T) {
	if code, _, stderr := runContextCLI(t); code == 0 || !strings.Contains(stderr, "context init") {
		t.Fatalf("bare `context` should print usage and fail: %d %s", code, stderr)
	}
	if code, _, stderr := runContextCLI(t, "bogus"); code == 0 || !strings.Contains(stderr, "unknown context command") {
		t.Fatalf("unknown subcommand: %d %s", code, stderr)
	}
}

func between(t *testing.T, text string, start string, end string) string {
	t.Helper()
	i := strings.Index(text, start)
	j := strings.Index(text, end)
	if i < 0 || j < i {
		t.Fatalf("markers %q..%q not found", start, end)
	}
	return text[i+len(start) : j]
}
