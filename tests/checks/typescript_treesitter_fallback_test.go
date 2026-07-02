package checks_test

import (
	"context"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// The tree-sitter path must degrade to the regex scanners per file: oversize
// inputs, unparseable garbage, and error-heavy trees all keep today's
// findings (docs/treesitter-spike.md §6.2). Regex-path findings carry no
// "high" confidence, which is how these tests observe the fallback.

// scanTreesitterFallbackFixture writes one fixture into a temp target and
// scans it with parsers.treesitter=auto, returning the unsafe-html-sink
// findings (line -> confidence).
func scanTreesitterFallbackFixture(t *testing.T, name string, content string) map[int]string {
	t.Helper()
	disableTypeScriptSemanticEngine(t)
	dir := t.TempDir()
	writeFile(t, dir+"/"+name, content)
	found, confidence := scanTreesitterCorpus(t, dir, "auto")
	byLine := map[int]string{}
	for finding := range found {
		if finding.Rule == "unsafe-html-sink" {
			byLine[finding.Line] = confidence[finding]
		}
	}
	return byLine
}

func TestTreesitterFallbackOversizeFile(t *testing.T) {
	// A sink on line 1 followed by >256 KiB of padding: over the parse cap,
	// so the regex path must still report it (without high confidence).
	var sb strings.Builder
	sb.WriteString("el.innerHTML = payload;\n")
	padding := "// " + strings.Repeat("x", 125) + "\n"
	for sb.Len() <= 256*1024 {
		sb.WriteString(padding)
	}
	byLine := scanTreesitterFallbackFixture(t, "oversize.ts", sb.String())
	if confidence, ok := byLine[1]; !ok {
		t.Fatalf("oversize file lost its unsafe-html-sink finding; regex fallback did not run (findings: %v)", byLine)
	} else if confidence == "high" {
		t.Fatalf("oversize file finding reports confidence high; the tree path should have refused a %d byte file", sb.Len())
	}
}

func TestTreesitterFallbackUnparseableGarbage(t *testing.T) {
	// The file is dominated by bytes no grammar accepts, so ERROR nodes
	// cover far more than the tolerated ratio; the regex line scan still
	// sees the sink assignment.
	content := "el.innerHTML = payload;\n" + strings.Repeat("((((( ]]]] @@@ %%% ~~~\n", 40)
	byLine := scanTreesitterFallbackFixture(t, "garbage.ts", content)
	if confidence, ok := byLine[1]; !ok {
		t.Fatalf("garbage file lost its unsafe-html-sink finding; regex fallback did not run (findings: %v)", byLine)
	} else if confidence == "high" {
		t.Fatal("garbage file finding reports confidence high; the tree path should have rejected an error-heavy tree")
	}
}

func TestTreesitterFallbackErrorHeavyFile(t *testing.T) {
	// Valid statements interleaved with a majority of broken ones: the tree
	// parses but ERROR nodes cover most bytes, so the file must take the
	// regex path.
	var sb strings.Builder
	sb.WriteString("el.innerHTML = payload;\n")
	for i := 0; i < 30; i++ {
		sb.WriteString("const = = = ((( { ]]] broken syntax everywhere ;;; %%%\n")
	}
	byLine := scanTreesitterFallbackFixture(t, "error_heavy.ts", sb.String())
	if confidence, ok := byLine[1]; !ok {
		t.Fatalf("error-heavy file lost its unsafe-html-sink finding; regex fallback did not run (findings: %v)", byLine)
	} else if confidence == "high" {
		t.Fatal("error-heavy file finding reports confidence high; the tree path should have rejected an error-heavy tree")
	}
}

// TestTreesitterHealthyFileUsesTreePath is the control for the fallback
// tests: the same sink in a healthy file takes the tree path.
func TestTreesitterHealthyFileUsesTreePath(t *testing.T) {
	byLine := scanTreesitterFallbackFixture(t, "healthy.ts", "export function render(el: HTMLElement, payload: string): void {\n  el.innerHTML = payload;\n}\n")
	if confidence, ok := byLine[2]; !ok {
		t.Fatalf("healthy file missing its unsafe-html-sink finding (findings: %v)", byLine)
	} else if confidence != "high" {
		t.Fatalf("healthy file finding confidence = %q, want high (tree path)", confidence)
	}
}

// TestTreesitterConfigValidation pins the accepted parsers.treesitter values.
func TestTreesitterConfigValidation(t *testing.T) {
	cfg := treesitterScanConfig(t.TempDir(), "sometimes")
	if _, err := codeguard.Run(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "parsers.treesitter") {
		t.Fatalf("invalid parsers.treesitter value produced err = %v, want validation error", err)
	}
}
