package support

import (
	"fmt"

	"github.com/odvcencio/gotreesitter"
)

// MaxTreeSitterFileBytes is the per-file size cap for tree-sitter parsing.
// The pure-Go runtime allocates roughly 0.5-0.6 MB of transient heap per KB
// of TypeScript parsed (docs/treesitter-spike.md §5.2 measured ~5.4 MB for a
// 10 KB file and ~120 MB for 200 KB), so 256 KiB bounds a single parse to on
// the order of 150 MB transient heap while comfortably covering real source
// files. Larger files fall back to the regex path.
const MaxTreeSitterFileBytes = 256 * 1024

// maxTreeSitterErrorByteRatio is the fraction of the file that may sit under
// ERROR nodes before the tree is rejected. Tree-sitter recovers from local
// damage, so rule queries still work around a small ERROR island; past 25%
// of the bytes the tree no longer models enough of the file and the regex
// path has better recall.
const maxTreeSitterErrorByteRatio = 0.25

// ParseScriptSource parses one script file with the embedded tree-sitter
// grammar for lang. It refuses (returning an error so callers use their
// regex fallback) when the file exceeds MaxTreeSitterFileBytes, the parser
// fails outright, or ERROR nodes cover more than maxTreeSitterErrorByteRatio
// of the source. The runner memoizes results per scan
// (runner/support.ParseScriptFile); this function is the uncached engine
// binding beneath that cache.
func ParseScriptSource(path string, data []byte, lang ScriptLanguage) (*SyntaxTree, error) {
	if len(data) > MaxTreeSitterFileBytes {
		return nil, fmt.Errorf("script file %q exceeds the %d byte tree-sitter limit", path, MaxTreeSitterFileBytes)
	}
	language, err := scriptGrammar(lang)
	if err != nil {
		return nil, err
	}
	parser := gotreesitter.NewParser(language)
	tree, err := parser.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("tree-sitter parse %q: %w", path, err)
	}
	if ratio := errorByteRatio(tree.RootNode(), len(data)); ratio > maxTreeSitterErrorByteRatio {
		return nil, fmt.Errorf("tree-sitter parse %q: error nodes cover %.0f%% of the file", path, 100*ratio)
	}
	return &SyntaxTree{lang: lang, language: language, tree: tree, source: data}, nil
}

// errorByteRatio reports the fraction of the source covered by ERROR nodes.
// Nested errors are not double-counted: once a node is an ERROR its whole
// span counts and its subtree is skipped.
func errorByteRatio(root *gotreesitter.Node, totalBytes int) float64 {
	if root == nil || totalBytes == 0 {
		return 0
	}
	errorBytes := countErrorBytes(root)
	return float64(errorBytes) / float64(totalBytes)
}

func countErrorBytes(node *gotreesitter.Node) int {
	if node.IsError() {
		return int(node.EndByte() - node.StartByte())
	}
	total := 0
	for i := 0; i < node.NamedChildCount(); i++ {
		total += countErrorBytes(node.NamedChild(i))
	}
	return total
}
