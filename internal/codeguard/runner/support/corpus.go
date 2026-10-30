package support

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"

	checkSupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
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
	mu                  sync.Mutex
	targets             map[string]*targetListing
	maxFiles            int
	reads               map[string]*fileRead
	readBytes           int
	maxReadBytes        int
	maxReadEntries      int
	readBudgetExceeded  bool
	readWork            chan struct{}
	readFile            func(string) ([]byte, error)
	asts                map[string]*goParse
	goASTBytes          int
	maxGoASTBytes       int
	maxGoASTEntries     int
	goASTBudgetExceeded bool
	goParseWork         chan struct{}
	scripts             map[string]*scriptParse
	scriptBytes         int
	scriptCount         int
	scriptParse         chan struct{}
	diagnostics         []core.Diagnostic
	diagnosticSeen      map[string]struct{}
	readErrorCount      int
	// walkFailures are reported on every section that finalizes after them,
	// unlike diagnostics, which are handed to the first section only.
	walkFailures []core.Diagnostic
}

// The corpus is an optimization, not the owner of repository contents, so its
// budgets bound what a scan *retains*, never what a scan analyses. Past a
// budget the corpus stops caching and serves the same work uncached: the bytes
// (or AST) are still produced and handed to the caller, they are just dropped
// once the caller is done with them. Retained memory therefore stays bounded
// exactly as it was when exhaustion meant "skip", while coverage is unaffected
// — the price of exceeding a budget is CPU and I/O, not missing findings.
//
// Getting this wrong is a security bug, not just a performance one: codeguard
// is a scanner, and a repository that floods the walk with generated files
// (a build cache, a bundled .next tree) would otherwise starve analysis of the
// real source and report fewer findings, with the only signal an informational
// diagnostic that the default text report does not even render.
//
// Budgets are expressed in RETAINED BYTES. A go/parser tree built with
// ParseComments costs far more than its input — measured at 9.0x source over
// this repository (711 files) and 8.4x over a 13.8k-file module cache — so the
// Go AST budget converts source length through goASTBytesPerSourceByte instead
// of counting source bytes directly. The previous constant counted source, so
// a nominal "64 MiB" Go AST budget admitted roughly 576 MiB of live AST.
const maxCorpusReadBytes = 128 << 20
const maxCorpusGoASTBytes = 256 << 20
const maxCorpusReadEntries = 50_000
const maxCorpusGoASTEntries = 25_000

// goASTBytesPerSourceByte is the measured heap cost of a retained
// ParseComments AST (plus its FileSet) per byte of source.
const goASTBytesPerSourceByte = 9

// Floors keep a usable cache even under a very small GOMEMLIMIT; exceeding a
// budget only costs speed, so a small cache is always safe.
const minCorpusReadBytes = 8 << 20
const minCorpusGoASTBytes = 16 << 20

// corpusRetentionBudget scales a default retention budget down to its share of
// an explicit GOMEMLIMIT. It never scales up: the defaults are chosen for a CI
// runner, and Go does not see a container memory limit unless GOMEMLIMIT is
// set, so inferring headroom from host memory would be wrong exactly where it
// matters. With no limit set, debug.SetMemoryLimit(-1) reports math.MaxInt64
// and the default applies unchanged.
func corpusRetentionBudget(def int, minimum int, share int64) int {
	limit := debug.SetMemoryLimit(-1)
	if limit <= 0 || limit == math.MaxInt64 {
		return def
	}
	scaled := limit / share
	if scaled >= int64(def) {
		return def
	}
	if scaled < int64(minimum) {
		return minimum
	}
	return int(scaled)
}

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
		maxReadBytes:    corpusRetentionBudget(maxCorpusReadBytes, minCorpusReadBytes, 8),
		maxReadEntries:  maxCorpusReadEntries,
		readWork:        make(chan struct{}, 2),
		readFile:        readCappedFile,
		asts:            map[string]*goParse{},
		maxGoASTBytes:   corpusRetentionBudget(maxCorpusGoASTBytes, minCorpusGoASTBytes, 4),
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
			c.recordWalkTruncation(root, c.maxFiles)
		}
		if entry.err != nil {
			c.recordWalkError(root, entry.err)
		}
	})
	return entry.files, entry.err
}

// read returns the bytes of root/rel, reading each file at most once per scan
// while the retention budget allows it to be cached. Once the budget is full
// the file is still read and returned, just not retained, so no caller ever
// loses content because the cache ran out of room.
func (c *fileCorpus) read(root string, rel string) ([]byte, error) {
	key := filepath.Clean(root) + "\x00" + rel
	path := filepath.Join(root, rel)
	c.mu.Lock()
	entry, ok := c.reads[key]
	if !ok {
		if len(c.reads) >= c.maxReadEntries {
			c.noteReadBudgetLocked("read_entries", c.maxReadEntries)
			c.mu.Unlock()
			return c.readUncached(path)
		}
		entry = &fileRead{}
		c.reads[key] = entry
	}
	c.mu.Unlock()

	entry.once.Do(func() {
		entry.data, entry.err = c.readUncached(path)
		c.mu.Lock()
		defer c.mu.Unlock()
		if entry.err == nil && c.readBytes+len(entry.data) <= c.maxReadBytes {
			c.readBytes += len(entry.data)
			return
		}
		// The slot is dropped either because the read failed or because keeping
		// these bytes would exceed the retention budget. Note the asymmetry
		// with the pre-budget cache: entry.data is deliberately left intact, so
		// this caller and anyone racing on the same slot still receive the file
		// contents. Only the retention is declined. Dropping the slot rather
		// than latching a scan-wide "exhausted" flag also keeps admission
		// best-fit: a later, smaller file can still use the headroom that an
		// oversized one could not.
		if entry.err == nil {
			c.noteReadBudgetLocked("read_bytes", c.maxReadBytes)
		}
		if c.reads[key] == entry {
			delete(c.reads, key)
		}
	})
	return entry.data, entry.err
}

// readUncached performs the file read itself. Reads stay serialized through
// readWork whether or not the result will be retained, so an overflowing scan
// cannot multiply the number of large buffers in flight at once.
func (c *fileCorpus) readUncached(path string) ([]byte, error) {
	c.readWork <- struct{}{}
	defer func() { <-c.readWork }()
	return c.readFile(path)
}

// parseGo returns a shared, read-only Go AST for the given source. The cache key
// includes the content hash so patched (diff-mode) content is reparsed rather
// than serving a stale tree. Callers must treat the returned *ast.File and
// *token.FileSet as immutable, which the AST inspection in the check sections
// already does.
func (c *fileCorpus) parseGo(path string, data []byte) (*token.FileSet, *ast.File, error) {
	key := path + "\x00" + hashBytes(data)
	retained := len(data) * goASTBytesPerSourceByte
	c.mu.Lock()
	entry, ok := c.asts[key]
	if !ok {
		switch {
		case len(c.asts) >= c.maxGoASTEntries:
			c.noteGoASTBudgetLocked("go_ast_entries", c.maxGoASTEntries)
		case c.goASTBytes+retained > c.maxGoASTBytes:
			c.noteGoASTBudgetLocked("go_ast_bytes", c.maxGoASTBytes)
		default:
			entry = &goParse{}
			c.asts[key] = entry
			c.goASTBytes += retained
		}
		if entry == nil {
			// Over budget: parse without retaining. The caller gets a real
			// tree that is collected as soon as it stops using it, instead of
			// an error that would have silently removed this file from every
			// AST-driven rule.
			c.mu.Unlock()
			return c.parseGoUncached(path, data)
		}
	}
	c.mu.Unlock()

	entry.once.Do(func() {
		entry.fset, entry.file, entry.err = c.parseGoUncached(path, data)
	})
	return entry.fset, entry.file, entry.err
}

// parseGoUncached runs the parse itself. It keeps the goParseWork gate so the
// transient parser working set stays bounded whether or not the tree is kept.
func (c *fileCorpus) parseGoUncached(path string, data []byte) (*token.FileSet, *ast.File, error) {
	c.goParseWork <- struct{}{}
	defer func() { <-c.goParseWork }()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, data, parser.ParseComments)
	return fset, file, err
}

// noteReadBudgetLocked and noteGoASTBudgetLocked record one overflow of a
// retention budget. They take the kind that actually tripped: the previous code
// latched a scan-wide "exhausted" flag and then reported every later miss as an
// entry-budget overflow, so a scan that filled the byte budget was told its
// 50,000-entry limit had been reached after caching only a few thousand files.
func (c *fileCorpus) noteReadBudgetLocked(kind string, limit int) {
	c.readBudgetExceeded = true
	c.recordBudgetLocked(kind, limit)
}

func (c *fileCorpus) noteGoASTBudgetLocked(kind string, limit int) {
	c.goASTBudgetExceeded = true
	c.recordBudgetLocked(kind, limit)
}

// recordBudgetLocked emits at most one diagnostic per budget kind. The level is
// informational because for the retention budgets it genuinely is: analysis
// continues over every file, only the caching stops. The walk's own file-count
// truncation is the exception -- it really does drop files from the scan -- and
// recordWalkTruncation reports that at warn level instead.
func (c *fileCorpus) recordBudgetLocked(kind string, limit int) {
	if _, exists := c.diagnosticSeen[kind]; exists {
		return
	}
	c.diagnosticSeen[kind] = struct{}{}
	c.diagnostics = append(c.diagnostics, core.Diagnostic{
		ID:      "scan.corpus-budget",
		Level:   "info",
		Kind:    "analysis",
		Message: "repository analysis exceeded a bounded corpus cache budget; the remaining files were analysed without caching, which is slower but reports the same findings",
		Metadata: map[string]string{
			"budget":            kind,
			"limit":             fmt.Sprintf("%d", limit),
			"retained_bytes":    fmt.Sprintf("%d", c.retainedBytesForLocked(kind)),
			"retained_entries":  fmt.Sprintf("%d", c.retainedEntriesForLocked(kind)),
			"degradation":       "uncached",
			"findings_affected": "no",
		},
	})
}

// recordWalkTruncation reports the one corpus bound that still costs coverage:
// walkFilesBounded stops enumerating at maxScanFileCount, so files past the cap
// are never offered to any check. That is a real reduction in what was scanned,
// so it is reported as a warning with its own id rather than folded into the
// informational cache-budget diagnostic.
func (c *fileCorpus) recordWalkTruncation(root string, limit int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.diagnosticSeen["files"]; exists {
		return
	}
	c.diagnosticSeen["files"] = struct{}{}
	c.diagnostics = append(c.diagnostics, core.Diagnostic{
		ID:      "scan.file-limit",
		Level:   "warn",
		Kind:    "analysis",
		Message: fmt.Sprintf("repository walk stopped at the %d file limit; files beyond it were not scanned, so findings may be missing - narrow the target or add generated directories to exclude", limit),
		Path:    root,
		// Operational, so the section reports warn rather than a clean pass it
		// has not earned: the walk did not finish, and a reader who sees only
		// the status must not read it as "nothing to report here".
		Operational: true,
		Metadata: map[string]string{
			"budget":            "files",
			"limit":             fmt.Sprintf("%d", limit),
			"degradation":       "truncated",
			"findings_affected": "yes",
		},
	})
}

// recordWalkError reports a target walk that failed. filepath.WalkDir stops at
// the first error, so the listing is at best partial and at worst empty; an
// empty listing yields no findings, which would otherwise read as a clean pass
// for every file-driven section. It is a failing operational diagnostic for
// that reason. The listing is memoized per target, so this is recorded once
// per target per scan, and takeDiagnostics then hands it to every section that
// finalizes afterwards (see walkFailures).
func (c *fileCorpus) recordWalkError(root string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := "walk\x00" + filepath.Clean(root)
	if _, exists := c.diagnosticSeen[key]; exists {
		return
	}
	c.diagnosticSeen[key] = struct{}{}
	c.walkFailures = append(c.walkFailures, core.Diagnostic{
		ID:          "scan.target-walk",
		Level:       "fail",
		Kind:        "infrastructure",
		Message:     fmt.Sprintf("could not walk target %s: %v; files under it were not scanned", root, err),
		Path:        root,
		Operational: true,
		Metadata: map[string]string{
			"degradation":       "unscanned",
			"findings_affected": "yes",
		},
	})
}

// maxReadErrorDiagnostics bounds how many per-file read failures are reported
// individually, so a target full of unreadable files cannot flood the report.
const maxReadErrorDiagnostics = 20

// recordReadError reports a file that the walk listed but that could not then
// be read, so it was dropped from every check that asked for it. It warns
// rather than fails: the rest of the target was scanned. Each file is reported
// once per scan; past maxReadErrorDiagnostics a single summary diagnostic notes
// that further failures were not itemised.
func (c *fileCorpus) recordReadError(root string, rel string, err error) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	key := "read\x00" + filepath.Clean(root) + "\x00" + rel
	if _, exists := c.diagnosticSeen[key]; exists {
		return
	}
	c.diagnosticSeen[key] = struct{}{}
	c.readErrorCount++
	if c.readErrorCount > maxReadErrorDiagnostics {
		if _, exists := c.diagnosticSeen["read-limit"]; exists {
			return
		}
		c.diagnosticSeen["read-limit"] = struct{}{}
		c.diagnostics = append(c.diagnostics, core.Diagnostic{
			ID:          "scan.file-read",
			Level:       "warn",
			Kind:        "infrastructure",
			Message:     fmt.Sprintf("more than %d files could not be read; further read failures are not listed individually", maxReadErrorDiagnostics),
			Operational: true,
			Metadata: map[string]string{
				"degradation":       "unscanned",
				"findings_affected": "yes",
			},
		})
		return
	}
	c.diagnostics = append(c.diagnostics, core.Diagnostic{
		ID:          "scan.file-read",
		Level:       "warn",
		Kind:        "infrastructure",
		Message:     fmt.Sprintf("could not read %s: %v; the file was not scanned", rel, err),
		Path:        filepath.ToSlash(filepath.Join(root, rel)),
		Operational: true,
		Metadata: map[string]string{
			"degradation":       "unscanned",
			"findings_affected": "yes",
		},
	})
}

func (c *fileCorpus) retainedBytesForLocked(kind string) int {
	if kind == "go_ast_bytes" || kind == "go_ast_entries" {
		return c.goASTBytes
	}
	return c.readBytes
}

func (c *fileCorpus) retainedEntriesForLocked(kind string) int {
	if kind == "go_ast_bytes" || kind == "go_ast_entries" {
		return len(c.asts)
	}
	return len(c.reads)
}

func (c *fileCorpus) takeDiagnostics() []core.Diagnostic {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	diagnostics := append([]core.Diagnostic(nil), c.diagnostics...)
	c.diagnostics = nil
	// Walk failures are not drained: every section that finalizes after one is
	// recorded may have consumed the failed listing, so each must report it
	// rather than leave all but the first looking like a clean pass.
	diagnostics = append(diagnostics, c.walkFailures...)
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
