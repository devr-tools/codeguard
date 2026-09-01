package support

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sync"

	checkSupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	errCorpusReadBudget  = errors.New("scan corpus read budget exhausted")
	errCorpusGoASTBudget = errors.New("scan corpus Go AST budget exhausted")
)

// readCappedFile reads path but refuses to buffer more than maxScanFileBytes,
// bounding memory even if a file grew past the walk-time size filter (TOCTOU) or
// is read outside the walk. It reads one byte past the cap to distinguish an
// exactly-cap-sized file from an oversized one.
func readCappedFile(path string) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // path enumerated by WalkFiles under the scan root
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxScanFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxScanFileBytes {
		return nil, fmt.Errorf("file %q exceeds the %d byte scan limit", path, maxScanFileBytes)
	}
	return data, nil
}

// fileCorpus memoizes, for the lifetime of a single scan, the expensive work
// that would otherwise be repeated by every check section: the per-target
// directory walk, the individual file reads, and Go AST parses. Every by-value
// copy of Context shares one *fileCorpus, so a file is walked, read, and parsed
// at most once per scan no matter how many sections inspect it.
//
// All methods are safe for concurrent use so that sections can run in parallel.
// Each cached slot carries its own sync.Once, so concurrent callers racing on a
// cold slot compute it exactly once and every caller observes the same result.
type fileCorpus struct {
	mu              sync.Mutex
	targets         map[string]*targetListing
	maxFiles        int
	reads           map[string]*fileRead
	readBytes       int
	maxReadBytes    int
	maxReadEntries  int
	readExhausted   bool
	readWork        chan struct{}
	readFile        func(string) ([]byte, error)
	asts            map[string]*goParse
	goASTBytes      int
	maxGoASTBytes   int
	maxGoASTEntries int
	goASTExhausted  bool
	goParseWork     chan struct{}
	scripts         map[string]*scriptParse
	scriptBytes     int
	scriptCount     int
	scriptParse     chan struct{}
	diagnostics     []core.Diagnostic
	diagnosticSeen  map[string]struct{}
}

// The corpus is an optimization, not the owner of repository contents. Bound
// its retained source and Go AST working sets so a full scan can keep making
// progress under GOMEMLIMIT instead of pinning every file until report output.
// Once a budget is reached, later uncached work is skipped and surfaced as an
// informational scan diagnostic rather than repeatedly allocating overflow
// data in concurrent check sections.
const maxCorpusReadBytes = 128 << 20
const maxCorpusGoASTSourceBytes = 64 << 20
const maxCorpusReadEntries = 50_000
const maxCorpusGoASTEntries = 25_000

// maxTreeSitterScanBytes bounds the source represented by retained script
// trees during one scan. Parsing is also serialized because the pure-Go
// runtime's transient heap is much larger than its input.
const maxTreeSitterScanBytes = 256 * 1024
const maxTreeSitterScanFiles = 64

type targetListing struct {
	once  sync.Once
	files []string
	err   error
}

type fileRead struct {
	once sync.Once
	data []byte
	err  error
}

type goParse struct {
	once sync.Once
	fset *token.FileSet
	file *ast.File
	err  error
}

type scriptParse struct {
	once sync.Once
	tree *checkSupport.SyntaxTree
	err  error
}

func newFileCorpus() *fileCorpus {
	return &fileCorpus{
		targets:         map[string]*targetListing{},
		maxFiles:        maxScanFileCount,
		reads:           map[string]*fileRead{},
		maxReadBytes:    maxCorpusReadBytes,
		maxReadEntries:  maxCorpusReadEntries,
		readWork:        make(chan struct{}, 2),
		readFile:        readCappedFile,
		asts:            map[string]*goParse{},
		maxGoASTBytes:   maxCorpusGoASTSourceBytes,
		maxGoASTEntries: maxCorpusGoASTEntries,
		goParseWork:     make(chan struct{}, 2),
		scripts:         map[string]*scriptParse{},
		scriptParse:     make(chan struct{}, 1),
		diagnosticSeen:  make(map[string]struct{}),
	}
}

// list returns every non-excluded file under root, walking the tree only once
// per target. Callers apply their own include filter to the returned slice; the
// walk itself is identical regardless of the filter, so sharing it is safe.
func (c *fileCorpus) list(root string, excludes []string, opts FileWalkOptions) ([]string, error) {
	key := fmt.Sprintf("%s\x00%s\x00%t", filepath.Clean(root), opts.LogicalPath, opts.ScanVendoredSource)
	c.mu.Lock()
	entry, ok := c.targets[key]
	if !ok {
		entry = &targetListing{}
		c.targets[key] = entry
	}
	c.mu.Unlock()

	entry.once.Do(func() {
		var truncated bool
		entry.files, truncated, entry.err = walkFilesBounded(root, excludes, opts, includeAll, c.maxFiles)
		if truncated {
			c.recordBudget("files", c.maxFiles)
		}
	})
	return entry.files, entry.err
}

// read returns the bytes of root/rel, reading each file at most once per scan.
func (c *fileCorpus) read(root string, rel string) ([]byte, error) {
	key := filepath.Clean(root) + "\x00" + rel
	c.mu.Lock()
	entry, ok := c.reads[key]
	if !ok {
		if c.readExhausted || len(c.reads) >= c.maxReadEntries {
			c.readExhausted = true
			c.recordBudgetLocked("read_entries", c.maxReadEntries)
			c.mu.Unlock()
			return nil, errCorpusReadBudget
		}
		entry = &fileRead{}
		c.reads[key] = entry
	}
	c.mu.Unlock()

	entry.once.Do(func() {
		c.readWork <- struct{}{}
		entry.data, entry.err = c.readFile(filepath.Join(root, rel))
		<-c.readWork
		c.mu.Lock()
		defer c.mu.Unlock()
		if entry.err == nil && c.readBytes+len(entry.data) <= c.maxReadBytes {
			c.readBytes += len(entry.data)
			return
		}
		if entry.err == nil {
			entry.data = nil
			entry.err = errCorpusReadBudget
			c.readExhausted = true
			c.recordBudgetLocked("read_bytes", c.maxReadBytes)
		}
		if c.reads[key] == entry {
			delete(c.reads, key)
		}
	})
	return entry.data, entry.err
}

// parseGo returns a shared, read-only Go AST for the given source. The cache key
// includes the content hash so patched (diff-mode) content is reparsed rather
// than serving a stale tree. Callers must treat the returned *ast.File and
// *token.FileSet as immutable, which the AST inspection in the check sections
// already does.
func (c *fileCorpus) parseGo(path string, data []byte) (*token.FileSet, *ast.File, error) {
	key := path + "\x00" + hashBytes(data)
	c.mu.Lock()
	entry, ok := c.asts[key]
	if !ok {
		if c.goASTExhausted || len(c.asts) >= c.maxGoASTEntries || c.goASTBytes+len(data) > c.maxGoASTBytes {
			c.goASTExhausted = true
			if len(c.asts) >= c.maxGoASTEntries {
				c.recordBudgetLocked("go_ast_entries", c.maxGoASTEntries)
			} else {
				c.recordBudgetLocked("go_ast_bytes", c.maxGoASTBytes)
			}
			c.mu.Unlock()
			return nil, nil, errCorpusGoASTBudget
		}
		entry = &goParse{}
		c.asts[key] = entry
		c.goASTBytes += len(data)
	}
	c.mu.Unlock()

	entry.once.Do(func() {
		c.goParseWork <- struct{}{}
		defer func() { <-c.goParseWork }()
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, data, parser.ParseComments)
		entry.fset, entry.file, entry.err = fset, file, err
	})
	return entry.fset, entry.file, entry.err
}

func (c *fileCorpus) recordBudget(kind string, limit int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recordBudgetLocked(kind, limit)
}

func (c *fileCorpus) recordBudgetLocked(kind string, limit int) {
	if _, exists := c.diagnosticSeen[kind]; exists {
		return
	}
	c.diagnosticSeen[kind] = struct{}{}
	c.diagnostics = append(c.diagnostics, core.Diagnostic{
		ID:      "scan.corpus-budget",
		Level:   "info",
		Kind:    "analysis",
		Message: "repository analysis reached a bounded corpus budget; remaining uncached work was skipped",
		Metadata: map[string]string{
			"budget": kind,
			"limit":  fmt.Sprintf("%d", limit),
		},
	})
}

func (c *fileCorpus) takeDiagnostics() []core.Diagnostic {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	diagnostics := append([]core.Diagnostic(nil), c.diagnostics...)
	c.diagnostics = nil
	return diagnostics
}

// parseScript returns a shared tree-sitter syntax tree for the given script
// source, mirroring parseGo: keyed by path plus content hash (so diff-mode
// patched content reparses) with a sync.Once per slot, so N rules across N
// sections pay for exactly one parse per file per scan. Returned trees are
// read-only; SyntaxTree queries are safe for concurrent use.
func (c *fileCorpus) parseScript(path string, data []byte, lang checkSupport.ScriptLanguage) (*checkSupport.SyntaxTree, error) {
	key := path + "\x00" + string(lang) + "\x00" + hashBytes(data)
	c.mu.Lock()
	entry, ok := c.scripts[key]
	if !ok {
		if c.scriptCount >= maxTreeSitterScanFiles || c.scriptBytes+len(data) > maxTreeSitterScanBytes {
			c.mu.Unlock()
			return nil, fmt.Errorf("tree-sitter scan budget of %d files or %d bytes exhausted", maxTreeSitterScanFiles, maxTreeSitterScanBytes)
		}
		entry = &scriptParse{}
		c.scripts[key] = entry
		c.scriptCount++
		c.scriptBytes += len(data)
	}
	c.mu.Unlock()

	entry.once.Do(func() {
		c.scriptParse <- struct{}{}
		defer func() { <-c.scriptParse }()
		entry.tree, entry.err = checkSupport.ParseScriptSource(path, data, lang)
	})
	return entry.tree, entry.err
}
