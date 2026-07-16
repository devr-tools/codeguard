package performance

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// Framework-aware TypeScript/JavaScript rules
// (performance_rules.detect_framework_patterns). Every rule is gated on
// file-level framework evidence — an import/require of react or express — so
// non-framework code never matches. Import evidence is matched on the RAW
// source: the module name lives in a string literal that the stripper blanks.
var (
	tsReactImportEvidence   = regexp.MustCompile(`from\s+["']react["']|require\(\s*["']react["']\s*\)`)
	tsExpressImportEvidence = regexp.MustCompile(`from\s+["']express["']|require\(\s*["']express["']\s*\)`)
	// tsComponentStart marks a React component or custom hook definition: a
	// function/const whose name starts with a capital letter or `use`.
	tsComponentStart = regexp.MustCompile(`\bfunction\s+(?:[A-Z]|use[A-Z])[\w$]*\s*\(|\bconst\s+(?:[A-Z]|use[A-Z])[\w$]*\s*(?::[^=]*)?=\s*(?:React\.memo\(\s*|memo\(\s*|forwardRef\(\s*)?(?:async\s+)?\(`)
	// tsHookWrapperStart marks a memo/effect wrapper region; work inside it
	// does not run on every render, so the render rule stays quiet there.
	tsHookWrapperStart = regexp.MustCompile(`\buse(?:Memo|Callback|Effect|LayoutEffect)\s*\(`)
	tsArrayChainCall   = regexp.MustCompile(`\.(?:sort|filter|map)\s*\(`)
	tsExpensiveCreate  = regexp.MustCompile(`\bnew\s+Array\s*\(|\bJSON\.parse\s*\(`)
	tsMiddlewareStart  = regexp.MustCompile(`\b(?:app|router)\.use\s*\(`)
	// tsCPUHeavySyncCall is the deliberate shortlist of CPU-heavy synchronous
	// APIs (bcrypt, crypto KDFs, zlib, child_process); generic *Sync calls in
	// handlers stay with performance.{ts,js}.sync-io-in-handler. Bare names
	// cover both qualified (bcrypt.hashSync) and destructured (hashSync) call
	// forms; the zlib alternative catches less common zlib *Sync helpers.
	tsCPUHeavySyncCall = regexp.MustCompile(`\b(?:hashSync|compareSync|pbkdf2Sync|scryptSync|execSync|gzipSync|gunzipSync|deflateSync|inflateSync|brotliCompressSync|brotliDecompressSync)\s*\(|\bzlib\.\w+Sync\s*\(`)
)

// tsFrameworkScan carries the framework-aware state for one file scan. A nil
// pointer (toggle disabled, or no framework evidence in the file) turns every
// hook into a no-op. Component regions are brace-delimited (function bodies);
// hook-wrapper and middleware regions are paren-delimited, because
// useMemo(() => expr, [deps]) and app.use(...) bodies may never open a brace.
type tsFrameworkScan struct {
	react       bool
	express     bool
	parenDepth  int
	components  []int
	hookRegions []int
	middlewares []int
}

func newTSFrameworkScan(rules core.PerformanceRulesConfig, source string) *tsFrameworkScan {
	if !toggleEnabled(rules.DetectFrameworkPatterns) {
		return nil
	}
	react := tsReactImportEvidence.MatchString(source)
	express := tsExpressImportEvidence.MatchString(source)
	if !react && !express {
		return nil
	}
	return &tsFrameworkScan{react: react, express: express}
}

// observe is called once per line after the generic checks, with the brace
// depth before (s.depth) and after (next) the line, mirroring the region
// convention of the generic scan.
func (f *tsFrameworkScan) observe(s *tsPerformanceScan, lineNo int, line string, next int) {
	if f == nil {
		return
	}
	nextParen := f.parenDepth + strings.Count(line, "(") - strings.Count(line, ")")
	startsComponent := f.react && tsComponentStart.MatchString(line)
	startsHook := f.react && tsHookWrapperStart.MatchString(line)
	f.checkReactRender(s, lineNo, line, startsComponent, startsHook)
	if startsComponent && next > s.depth {
		f.components = append(f.components, s.depth)
	}
	if startsHook && nextParen > f.parenDepth {
		f.hookRegions = append(f.hookRegions, f.parenDepth)
	}
	if f.express && tsMiddlewareStart.MatchString(line) && nextParen > f.parenDepth {
		f.middlewares = append(f.middlewares, f.parenDepth)
	}
	f.components = popNestedRegions(f.components, next)
	f.hookRegions = popNestedRegions(f.hookRegions, nextParen)
	f.middlewares = popNestedRegions(f.middlewares, nextParen)
	f.parenDepth = nextParen
}

// checkReactRender implements performance.{typescript,javascript}.react-expensive-render:
// inside a component (or custom hook) body, a chain of two or more array
// methods (.sort/.filter/.map) or an expensive construction (new Array,
// JSON.parse) that reruns on every render. Lines inside (or starting) a
// useMemo/useCallback/useEffect wrapper are exempt.
func (f *tsFrameworkScan) checkReactRender(s *tsPerformanceScan, lineNo int, line string, startsComponent bool, startsHook bool) {
	if !f.react || (len(f.components) == 0 && !startsComponent) {
		return
	}
	if len(f.hookRegions) > 0 || startsHook {
		return
	}
	if len(tsArrayChainCall.FindAllString(line, -1)) >= 2 {
		s.addFinding("performance.typescript.react-expensive-render", "performance.javascript.react-expensive-render", lineNo,
			"array method chain in the component body reruns on every render; memoize it with useMemo or compute it outside the component")
		return
	}
	if tsExpensiveCreate.MatchString(line) {
		s.addFinding("performance.typescript.react-expensive-render", "performance.javascript.react-expensive-render", lineNo,
			"expensive construction (new Array/JSON.parse) in the component body reruns on every render; wrap it in useMemo or hoist it out of the component")
	}
}

// reportExpressSyncMiddleware implements
// performance.{typescript,javascript}.express-sync-middleware: a CPU-heavy
// synchronous call inside an app.use/router.use middleware region in a file
// with express evidence. It returns true when it reported, so the caller can
// skip the generic sync-io rule and one line never reports twice.
func (f *tsFrameworkScan) reportExpressSyncMiddleware(s *tsPerformanceScan, lineNo int, line string) bool {
	if f == nil || !f.express {
		return false
	}
	if len(f.middlewares) == 0 && !tsMiddlewareStart.MatchString(line) {
		return false
	}
	if !tsCPUHeavySyncCall.MatchString(line) {
		return false
	}
	s.addFinding("performance.typescript.express-sync-middleware", "performance.javascript.express-sync-middleware", lineNo,
		"CPU-heavy synchronous call inside Express middleware blocks the event loop for every request; use the async API (bcrypt.hash, crypto.pbkdf2, async zlib, exec) or move the work off the request path")
	return true
}

func popNestedRegions(regions []int, next int) []int {
	for len(regions) > 0 && next <= regions[len(regions)-1] {
		regions = regions[:len(regions)-1]
	}
	return regions
}
