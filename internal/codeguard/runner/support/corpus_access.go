package support

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"

	checkSupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func includeAll(string) bool { return true }

func (sc Context) corpusFiles(target core.TargetConfig) ([]string, error) {
	opts := FileWalkOptions{
		LogicalPath:        target.LogicalPath,
		ScanVendoredSource: sc.Cfg.ScanVendoredSource,
	}
	if sc.corpus != nil {
		return sc.corpus.list(target.Path, sc.Cfg.Exclude, opts)
	}
	return WalkFilesWithOptions(target.Path, sc.Cfg.Exclude, opts, includeAll)
}

func (sc Context) corpusRead(root string, rel string) ([]byte, error) {
	if sc.corpus != nil {
		return sc.corpus.read(root, rel)
	}
	return readCappedFile(filepath.Join(root, rel))
}

func ParseGoFile(sc Context, path string, data []byte) (*token.FileSet, *ast.File, error) {
	if sc.corpus != nil {
		return sc.corpus.parseGo(path, data)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, data, parser.ParseComments)
	return fset, file, err
}

func ParseScriptFile(sc Context, path string, data []byte, lang checkSupport.ScriptLanguage) (*checkSupport.SyntaxTree, error) {
	if sc.corpus != nil {
		return sc.corpus.parseScript(path, data, lang)
	}
	return checkSupport.ParseScriptSource(path, data, lang)
}
