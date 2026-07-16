package support

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// ScriptLanguage names a tree-sitter grammar used for script files. It is
// deliberately engine-neutral: checks pass it to Context.ParseScriptFile and
// never see the underlying runtime.
type ScriptLanguage string

const (
	ScriptLangTypeScript ScriptLanguage = "typescript"
	ScriptLangTSX        ScriptLanguage = "tsx"
	ScriptLangJavaScript ScriptLanguage = "javascript"
	ScriptLangPython     ScriptLanguage = "python"
)

// ScriptLanguageForPath maps a file path onto the grammar that parses it:
// .ts/.mts/.cts use the TypeScript grammar, .tsx the TSX grammar (JSX and
// type annotations are grammatically incompatible, so upstream ships two),
// .js/.jsx/.mjs/.cjs the JavaScript grammar (which includes JSX), and .py
// the Python grammar. It returns "" for non-script files.
func ScriptLanguageForPath(path string) ScriptLanguage {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".mts", ".cts":
		return ScriptLangTypeScript
	case ".tsx":
		return ScriptLangTSX
	case ".js", ".jsx", ".mjs", ".cjs":
		return ScriptLangJavaScript
	case ".py":
		return ScriptLangPython
	default:
		return ""
	}
}

// SyntaxTree is the engine-neutral handle for one parsed script file. It
// wraps the gotreesitter tree plus the exact source bytes it was parsed
// from, so query hits can carry node text without the caller re-slicing.
// Trees are immutable after parse and safe for concurrent Query calls.
type SyntaxTree struct {
	lang     ScriptLanguage
	language *gotreesitter.Language
	tree     *gotreesitter.Tree
	source   []byte
}

// Language reports which grammar parsed the tree.
func (t *SyntaxTree) Language() ScriptLanguage { return t.lang }

// ScriptSyntaxTree resolves the parsed tree for a script file through the
// Context's ParseScriptFile hook. It returns nil whenever the tree path is
// unavailable — hook not wired (parsers.treesitter "off" or a bare unit-test
// Context), non-script path, or any parse-level refusal (oversize, parse
// error, error-heavy tree) — and callers then use their regex path.
func ScriptSyntaxTree(env Context, file string, source string) *SyntaxTree {
	if env.ParseScriptFile == nil {
		return nil
	}
	lang := ScriptLanguageForPath(file)
	if lang == "" {
		return nil
	}
	tree, err := env.ParseScriptFile(file, []byte(source), lang)
	if err != nil {
		return nil
	}
	return tree
}

// scriptGrammar returns the gotreesitter language for a ScriptLanguage. The
// grammar registry caches decoded grammars process-wide, so repeated lookups
// are cheap after the first (which decodes the embedded blob).
//
// Each case here must have a matching grammar_subset_<lang> build tag in the
// release tag set (Makefile GRAMMAR_TAGS and .goreleaser.yaml): under a
// grammar_subset build a grammar whose tag is absent is not registered or
// embedded, and its accessor PANICS rather than returning nil — hence the
// registry probe below, which turns a missing grammar into an error so every
// rule using it takes its regex fallback instead of losing the section to
// the safeRun panic recovery.
func scriptGrammar(lang ScriptLanguage) (*gotreesitter.Language, error) {
	if grammars.DetectLanguageByName(string(lang)) == nil {
		return nil, fmt.Errorf("tree-sitter grammar %q is not embedded in this build", lang)
	}
	var language *gotreesitter.Language
	switch lang {
	case ScriptLangTypeScript:
		language = grammars.TypescriptLanguage()
	case ScriptLangTSX:
		language = grammars.TsxLanguage()
	case ScriptLangJavaScript:
		language = grammars.JavascriptLanguage()
	case ScriptLangPython:
		language = grammars.PythonLanguage()
	default:
		return nil, fmt.Errorf("no tree-sitter grammar for script language %q", lang)
	}
	if language == nil {
		return nil, fmt.Errorf("tree-sitter grammar %q is not embedded in this build", lang)
	}
	return language, nil
}
