package support_test

import (
	"strings"
	"testing"

	checksupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

func TestScriptLanguageForPath(t *testing.T) {
	cases := map[string]checksupport.ScriptLanguage{
		"src/app.ts":      checksupport.ScriptLangTypeScript,
		"src/app.mts":     checksupport.ScriptLangTypeScript,
		"src/app.cts":     checksupport.ScriptLangTypeScript,
		"src/View.tsx":    checksupport.ScriptLangTSX,
		"src/app.js":      checksupport.ScriptLangJavaScript,
		"src/View.jsx":    checksupport.ScriptLangJavaScript,
		"src/app.mjs":     checksupport.ScriptLangJavaScript,
		"src/app.cjs":     checksupport.ScriptLangJavaScript,
		"src/main.cpp":    checksupport.ScriptLangCPP,
		"include/app.hpp": checksupport.ScriptLangCPP,
		"src/widget.ixx":  checksupport.ScriptLangCPP,
		"src/widget.cppm": checksupport.ScriptLangCPP,
		"src/widget.c++m": checksupport.ScriptLangCPP,
		"include/app.inl": checksupport.ScriptLangCPP,
		"include/app.h":   "",
		"src/main.go":     "",
		"src/app.ts.bak":  "",
	}
	for path, want := range cases {
		if got := checksupport.ScriptLanguageForPath(path); got != want {
			t.Errorf("ScriptLanguageForPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestParseScriptSourceParsesAndQueries(t *testing.T) {
	source := []byte("let x: any;\nconst y = value as unknown as number;\n")
	tree, err := checksupport.ParseScriptSource("fixture.ts", source, checksupport.ScriptLangTypeScript)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	query := checksupport.CompileScriptQuery(`(predefined_type) @any.type`)
	hits, err := tree.Query(query)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	lines := make([]int, 0, len(hits))
	for _, hit := range hits {
		for _, capture := range hit.Captures {
			if capture.Name == "any.type" && capture.Text == "any" {
				lines = append(lines, capture.Line)
			}
		}
	}
	if len(lines) != 1 || lines[0] != 1 {
		t.Fatalf("explicit-any capture lines = %v, want [1]", lines)
	}
}

func TestParseScriptSourceRejectsOversizeFile(t *testing.T) {
	data := []byte(strings.Repeat("const filler = 1;\n", 1+checksupport.MaxTreeSitterFileBytes/18))
	if len(data) <= checksupport.MaxTreeSitterFileBytes {
		t.Fatalf("fixture is %d bytes; want > %d", len(data), checksupport.MaxTreeSitterFileBytes)
	}
	if _, err := checksupport.ParseScriptSource("big.ts", data, checksupport.ScriptLangTypeScript); err == nil {
		t.Fatal("oversize file parsed; want size-cap refusal")
	}
}

func TestParseScriptSourceRejectsErrorHeavySource(t *testing.T) {
	garbage := []byte("\x00\x01\x02 ((((( ]]]] @@@ %%% not typescript at all ~~~\n" + strings.Repeat("=== ((( ]]] ;;; %%%\n", 20))
	if _, err := checksupport.ParseScriptSource("garbage.ts", garbage, checksupport.ScriptLangTypeScript); err == nil {
		t.Fatal("error-heavy source accepted; want error-ratio refusal")
	}
}

func TestParseScriptSourceAcceptsLocallyDamagedSource(t *testing.T) {
	// Tree-sitter recovers from a locally-contained mistake (a missing
	// expression yields a one-byte ERROR); a small ERROR island must not
	// force the whole file onto the regex path. (Cascading damage such as an
	// unclosed paren swallows the rest of the file into one ERROR node and
	// correctly falls back — TestParseScriptSourceRejectsErrorHeavySource.)
	source := []byte("const a: any = 1;\nconst broken = ;\n" + strings.Repeat("export const pad = 1;\n", 30))
	if _, err := checksupport.ParseScriptSource("damaged.ts", source, checksupport.ScriptLangTypeScript); err != nil {
		t.Fatalf("locally damaged source rejected: %v", err)
	}
}

func TestParseScriptSourceRejectsUnknownLanguage(t *testing.T) {
	if _, err := checksupport.ParseScriptSource("file.rb", []byte("puts 1"), checksupport.ScriptLanguage("ruby")); err == nil {
		t.Fatal("unknown script language accepted; want error")
	}
}

func TestParseScriptSourceParsesCPP(t *testing.T) {
	source := []byte("#include <regex>\nint scan(const std::string& value) { std::regex digits(\"[0-9]+\"); return digits.mark_count(); }\n")
	if _, err := checksupport.ParseScriptSource("fixture.cpp", source, checksupport.ScriptLangCPP); err != nil {
		t.Fatalf("parse cpp: %v", err)
	}
}
