# Design spike: tree-sitter as the non-Go parsing substrate

- Status: spike complete; phase 2 (TypeScript behind `parsers.treesitter`)
  shipped — see §9
- Date: 2026-07-02
- Prototype: `internal/codeguard/checks/support/treesitter/` (isolated Go module)
- Decision: **conditional GO** — adopt tree-sitter via the pure-Go runtime
  `github.com/odvcencio/gotreesitter`, vendored and pinned, behind a
  `ParserProvider` seam, TypeScript first, with the hand-rolled parsers kept
  as fallback. **NO-GO on every CGo-based option.**

## 1. Why this spike

Go files get a real AST (`Context.ParseGoFile` → `go/parser`, cached once per
scan in `runner/support/corpus.go`). Every other language rides on hand-rolled
approximations in `internal/codeguard/checks/support/`:

- `parser_clike*.go` — a masker + regex-head function finder for TS/JS, Java,
  Rust (and C# via the same machinery),
- `python_lexer*.go` / `python_parser*.go` — an indentation-tracking Python
  approximation,
- ~60 rules that run regexes over `StripTypeScriptCommentsAndStrings` (or
  masked) text.

Concrete fidelity limits found while reading the current code:

| Limitation | Where | Consequence |
|---|---|---|
| Template literals are blanked *including* `${...}` interpolations | `typescript_scan.go` `handleTemplate` | Rules using `ctx.code` are blind to real code inside interpolations (false negatives, demonstrated below) |
| No regex-literal state in the stripper | `typescript_scan.go` | Regex literal bodies leak into the "code" view (false positives); a quote inside a regex (`/["']/`) corrupts the string state for the rest of the file |
| Two maskers with different semantics | `parser_clike_lexer_strings.go` `scanInterpolation` keeps `${}` visible; the stripper blanks it | Rule behavior depends on which helper a check happens to use |
| Function discovery is line-anchored regex | `parser_clike_lang.go` (`tsFunctionHead`, `tsMethodHead`, …) | Object-literal methods, IIFEs, and functions not at line start are invisible; generic parameter lists must fit on one line (`<[^>\n]*>`); TS decorators are not modeled at all (only Java annotations are) |
| Statements are physical lines | `parser_clike.go` `populateCLikeBody` | Multiline expressions fragment into unrelated "statements" |
| Regexes cannot see syntactic roles | e.g. `tsExplicitAnyPattern`, `typeScriptUnsafeHTMLPattern` | `any` the identifier vs `any` the type; `.innerHTML ===` (comparison) vs `.innerHTML =` (assignment); `satisfies any` and `.innerHTML +=` missed entirely |

These are architectural, not bugs: each fix to the masker adds another state
to a hand-maintained lexer per language. Tree-sitter replaces that with
maintained grammars and a declarative query language.

## 2. Constraints that shape the decision

1. **Supply-chain minimalism.** The root `go.mod` is one dependency
   (`gopkg.in/yaml.v3`). This is a deliberate stance for a security tool and
   any recommendation must preserve its spirit: few deps, pinned, auditable,
   vendorable.
2. **`CGO_ENABLED=0` everywhere.** `.goreleaser.yaml` cross-builds
   darwin/linux × amd64/arm64 from one runner with `CGO_ENABLED=0`; the
   GitHub Action (`action.yml`) installs via `go install`, which compiles on
   the user's machine. CGo would require per-OS builders (or
   zig-cc/osxcross), and would break `go install` for any user without a C
   toolchain (Windows in particular).
3. **Scan performance.** Sections already parallelize per file; per-file
   parse cost lands on the critical path once per scan, and the corpus layer
   can cache trees the way it caches Go ASTs.

## 3. Options matrix

| | (a) official CGo bindings | (b) smacker/go-tree-sitter | (c) WASM: wazero + tree-sitter.wasm | (d) pure-Go parsers per language | (e) status quo++ | **(f) gotreesitter (pure-Go runtime)** |
|---|---|---|---|---|---|---|
| Packages | `tree-sitter/go-tree-sitter` + one module per grammar | single module, ~40 grammars vendored | `tetratelabs/wazero` (zero-dep) + custom wasm builds | esbuild internals (not importable), goja/tdewolff (JS only), gpython (~py3.4) | none | `odvcencio/gotreesitter` |
| CGo / cross-compile | **CGo — breaks goreleaser matrix and `go install`** | same | pure Go, cross-compiles | pure Go | pure Go | **pure Go; `CGO_ENABLED=0 GOOS=windows/linux` verified in this spike** |
| Grammar coverage | all 6 target langs (official modules) | all 6, but stale | whatever we compile to wasm ourselves | TS: none importable; Python: outdated; Rust/Java/C#: none | n/a | 206 embedded grammars incl. TS/TSX/JS/Python/Rust/Java/C#; 116 hand-ported external scanners (Python indentation etc.) |
| Dependency weight | ~8 modules + go.sum noise (grammar test deps, upstream issue #49) | 1 module, never tagged (pseudo-versions only) | wazero (0 deps) + a wasm toolchain we own | varies | 0 | **2 runtime modules: gotreesitter + `golang.org/x/sync` (`yaml.v3` already a root dep)** |
| Maintenance | active-ish; v0.25.0 tag deleted upstream (issue #50); known cgo handle leaks (#55) | dead since Aug 2024 | we own the wasm build + FFI shim forever; only existing Go packages are 3-commit prototypes (malivvan, ngavinsir) | mixed-to-dead | all on us, forever | active (v0.20.8 July 2026, 30 releases); **single maintainer, pre-1.0** |
| License | MIT throughout | MIT, but vendored grammar C lacks upstream LICENSE files (attribution gap) | Apache-2.0 (wazero) + MIT | MIT/BSD | n/a | MIT (grammar blobs derived from MIT grammars) |
| Parse perf (this spike / published) | **measured: ~5-8 MB/s native** | similar | ~4.7x slower than native under wazero (00f.net 2026 runtime bench) → ~1-1.7 MB/s est. | fast where they exist | regex ~17 MB/s | **measured: ~0.9-1.4 MB/s parse; query-on-cached-tree faster than CGo (no FFI)** |
| Verdict | NO-GO (constraint 2) | NO-GO (dead + constraint 2) | NO-GO for now (no mature package; we'd own a wasm FFI layer; slower than (f) anyway) | NO-GO (can't cover the matrix) | fallback only | **candidate** |

Notes on (c): wazero itself is excellent (zero-dep, Apache-2.0, maintained),
but tree-sitter's C API passes `TSNode` structs by value, which does not
cross the wasm C ABI — every existing attempt ended up building custom wasm
shims. That is a multi-week engineering project we would own forever, for a
result measured slower than the pure-Go runtime that already exists.

Notes on (d): esbuild has the best TS parser in Go but its AST is
`internal/` and the author declined to export it (esbuild #92); goja and
tdewolff/parse are JS-only (no TS/JSX); nothing credible exists for
Rust/Java/C#. This path cannot cover the language matrix.

## 4. Prototype

Location: `internal/codeguard/checks/support/treesitter/` — **a separate Go
module on purpose**, so its dependencies never touch the root `go.mod`
(81 bytes)/`go.sum`. `go build ./...`, `go vet ./...`, `go test ./...` and
golangci-lint at the root skip nested modules automatically; the root gate
stayed green throughout the spike. The repo self-scan
(`make codeguard-ci`) does walk into the directory, so
`.codeguard/codeguard.yaml` gains three narrowly-scoped waivers
(`ci.test-file-location`, `supply_chain.lockfile-drift`,
`quality.ai.hallucinated-import`) whose reasons all reduce to "this is a
nested module the self-scanner does not model"; they should be removed with
the spike.

What it contains:

- `rules.go` — the two rules under test, written once as tree-sitter
  queries + a small engine-independent classifier:
  - `quality.typescript.explicit-any` → `(predefined_type) @any.type`,
    keep text == `any` (type positions only, by construction);
  - `security.typescript.unsafe-html-sink` → assignment / augmented
    assignment to `.innerHTML`/`.outerHTML`, `.insertAdjacentHTML(...)`
    calls, and `document.write/writeln(...)` calls.
- `baseline.go` — byte-for-byte replica of the production implementation
  (same regexes, same `support.StripTypeScriptCommentsAndStrings`, same
  per-line dedupe), importing the real `support` package via a `replace`.
- `engine_purego.go` — gotreesitter engine (always built).
- `engine_cgo.go` — official CGo bindings engine (`//go:build cgo`), used as
  the native-runtime correctness and performance reference.
- `testdata/adversarial.ts` — self-describing corpus: `EXPECT` markers are
  ground truth, `BASELINE-FP`/`BASELINE-FN` markers pin the current regex
  behavior, and the tests fail if either implementation drifts.
- `testdata/realistic.ts` — ~350 lines of plausible TS for benchmarks.

How to run (needs the working GOROOT, see repo notes):

```sh
cd internal/codeguard/checks/support/treesitter
go test -v .                       # both engines (host has a C compiler)
CGO_ENABLED=0 go test -v .         # pure-Go engine only
go test -run '^$' -bench . -benchmem -count 3 .
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build .   # cross-compile proof
```

## 5. Results

### 5.1 Precision (adversarial corpus; 13 ground-truth findings after the
phase-2 additions S11/S12 — originally 11)

| Implementation | Reported | TP | FP | FN | Precision | Recall |
|---|---|---|---|---|---|---|
| Current regex + stripper | 11 | 7 | 4 | 6 | **63.6%** | **53.8%** |
| tree-sitter (pure-Go engine) | 13 | 13 | 0 | 0 | **100%** | **100%** |
| tree-sitter (CGo engine) | 13 | 13 | 0 | 0 | 100% | 100% |

The individual cases (all pinned by `TestBaselinePrecisionGap`):

- Baseline **false positives**: regex literal body (`/: any\b/`), `any` as a
  legal identifier in parameter and call positions (`values.filter(any)`),
  and `el.innerHTML === ""` (comparison matched as assignment).
- Baseline **false negatives**: real code inside template interpolations
  (`` `rows=${(rows as any).length}` `` and an assignment inside `${...}`),
  `satisfies any` (TS 4.9+ syntax postdates the pattern list),
  `el.innerHTML += chunk` (compound assignment), and a formatter-split
  `document\n  .write(...)` receiver.
- Parity cases (comments, plain strings, multiline generics) behave the same
  in both implementations.

On the realistic corpus both engines and the baseline agree exactly
(4 findings): the precision gap is specifically about adversarial
constructs, and the two tree-sitter runtimes agree with each other
node-for-node on every corpus in the spike (`TestRealisticParity`,
`TestEnginesMatchGroundTruth`).

Known shared limitation: all of these are syntactic scanners. A property
named `innerHTML` on a non-DOM object is flagged by both regex and
tree-sitter; type-aware analysis is out of scope for this spike either way.

### 5.2 Performance (Apple arm64, 8 cores, Go 1.26.4; medians of 3 runs)

Full scan = parse + both rules. Cached tree = both rule queries against an
already-parsed tree (the steady-state cost per additional rule pack under a
corpus tree cache).

| Benchmark | 10 KB file | 200 KB file |
|---|---|---|
| Current regex (strip + 2 regexes) | 0.62 ms (17.6 MB/s) | 13.1 ms (16.5 MB/s) |
| CGo: parse only | 1.99 ms (5.4 MB/s) | 27.6 ms (7.9 MB/s) |
| CGo: full scan | 2.09 ms (5.2 MB/s) | 40.7 ms (5.3 MB/s) |
| CGo: rules on cached tree | 0.69 ms | 13.8 ms |
| pure-Go: parse only | 12.5 ms (0.87 MB/s) | ~163 ms (1.3 MB/s) |
| pure-Go: full scan | 12.1 ms (0.89 MB/s) | ~158 ms (1.4 MB/s) |
| pure-Go: rules on cached tree | **0.39 ms (28 MB/s)** | ~14.4 ms |

Readings:

- **Parse is the whole cost; queries are cheap.** Once a tree exists, the
  pure-Go query engine is *faster* than the CGo one (no FFI boundary per
  node): 0.39 ms vs 0.69 ms per 10 KB. Caching trees in the corpus (like Go
  ASTs today) makes each *additional* tree-sitter rule cheaper than each
  additional regex pass.
- **Pure-Go parse is ~6x native tree-sitter and ~20x the regex pass.** For
  scale: a 2,000-file / 20 MB TS monorepo costs ~15-22 s of single-core
  parse time, divided across cores by the existing per-file parallelism, and
  paid once per scan regardless of rule count. Typical diff-scoped scans
  touch far fewer files. This is real but budgetable; it is also the
  number most likely to improve (young project, and the TS grammar is one of
  tree-sitter's heaviest).
- **Memory is the sharper edge:** the pure-Go parse allocates ~5.4 MB per
  10 KB file (~120 MB for the 200 KB input). Mitigations: keep the existing
  capped-read policy, add a per-file size cutoff for tree parsing (fall back
  to regex above it), and drop trees eagerly after a file's sections finish.
- First parse per language lazily loads the grammar blob (~tens of ms,
  once per process; amortized in benchmarks).

### 5.3 Binary size

| Build | Size |
|---|---|
| codeguard today (root, CGO_ENABLED=0) | 14.1 MB |
| spike test binary, TS grammar only (`-tags grammar_subset,grammar_subset_typescript`) | 9.3 MB |
| spike test binary, all 206 grammars embedded (default) | 31.5 MB |

gotreesitter's `grammar_subset_<lang>` build tags are the difference between
+~3-5 MB (the runtime plus the 4-6 grammars we need) and +~22 MB (the full
registry). We must build with the subset tags; goreleaser gains a
`flags: -tags=...` line and nothing else changes.

## 6. Integration architecture

### 6.1 ParserProvider in `checks/support`

Mirror the existing Go path (`Context.ParseGoFile` → corpus `sync.Once`
cache). Add one hook and one engine-neutral tree handle:

```go
// checks/support/context.go
type Context struct {
    ...
    ParseGoFile     func(path string, data []byte) (*token.FileSet, *ast.File, error)
    ParseScriptFile func(path string, data []byte, lang ScriptLanguage) (*SyntaxTree, error)
}

// checks/support/treeprovider.go (root module; engine behind it)
type SyntaxTree struct { ... }          // wraps *gotreesitter.Tree + source
func (t *SyntaxTree) Query(q *CompiledQuery) []QueryHit
```

- The runner wires `ParseScriptFile` to a corpus-level cache identical in
  shape to `corpus.parseGo` (map keyed by path+len, `sync.Once` per entry),
  so N rules on one file pay for one parse. Trees for a file are released
  once all sections complete (memory point above).
- Checks compile their queries once (package-level `sync.OnceValue`) and ask
  the tree for hits; the spike's `rules.go` shows the shape — a query string
  plus a small classifier is a full rule.
- `support.ScriptRegexFindings` stays; a new
  `support.ScriptQueryFindings(env, file, tree, spec)` sits beside it with
  the same `FindingInput` output, so migrated and unmigrated rules coexist
  in one section.

### 6.2 Fallback story

- If `ParseScriptFile` is nil (unit-test contexts), returns an error, the
  file exceeds the size cap, or the tree's root contains error nodes above a
  threshold, the check falls back to the current regex path for that file.
  Tree-sitter's error recovery makes hard failures rare, but the fallback
  keeps behavior total.
- The hand-rolled parsers are not deleted until a language has been
  default-on for several releases (see phases). Config gains
  `parsers.treesitter: auto|off|<per-language>` for emergency opt-out.

### 6.3 Per-language migration order

1. **TypeScript/TSX/JS** — largest rule count, worst regex pain (templates,
   generics, decorators, JSX), grammar exercised by this spike.
2. **Python** — indentation-sensitive; gotreesitter ships a hand-ported
   Python external scanner. Replaces `python_lexer*`/`python_parser*`
   (f-strings, decorators, multi-line statements become free).
3. **Rust, then Java** — retire the `parser_clike` lifetime/annotation
   hacks.
4. **C#** — last; lowest rule coverage today.

Each language migrates rule-by-rule with a differential test in the spike
style: adversarial corpus with `EXPECT` markers, old and new implementations
pinned side by side.

## 7. goreleaser / cross-compile impact

With the pure-Go engine: **none**, beyond adding the grammar-subset build
tags. `CGO_ENABLED=0` stays; the same single-runner matrix builds
darwin/linux × amd64/arm64 (spike verified `GOOS=windows GOARCH=amd64` and
`GOOS=linux GOARCH=arm64` compile with CGO disabled); `go install` and the
GitHub Action keep working on machines without a C toolchain; SBOM/cosign
steps are unchanged (the SBOM finally has something to list).

For contrast, any CGo option would have required per-OS builders or a
zig/osxcross toolchain and would have broken `go install` on Windows — that
alone is disqualifying given how the Action is distributed.

## 8. Supply-chain assessment

What actually lands in the ship graph: `github.com/odvcencio/gotreesitter`
(MIT) and `golang.org/x/sync` (BSD-3, Go project). `gopkg.in/yaml.v3` is
already a root dependency at the same version. Test-only indirects
(`kr/pretty`, `check.v1`) do not ship.

Honest risk statement: this is a young (2026), pre-1.0, single-maintainer
project, and it would become the largest body of third-party code in a
security tool that today has effectively none. The parse tables are
generated from upstream tree-sitter grammars, but the runtime is a from-
scratch reimplementation. Mitigations, in order:

1. **Pin exactly** (`v0.20.8`), never `@latest`; go.sum hashes enforce
   integrity; Dependabot already watches the repo and version bumps get the
   same review as any PR.
2. **Vendor** (`go mod vendor` at the root once it ships) so every byte is
   in-tree, reviewable in PRs, buildable offline, and captured by the
   existing SBOM/cosign release pipeline. Given the size, an alternative is
   a fork under the org (`devr-tools/gotreesitter`) that we sync
   deliberately — fork-on-first-ship is the recommended posture: it converts
   "single external maintainer" into "upstream we pull from at our pace".
3. **Differential testing in CI** of the spike module (not the root): the
   pure-Go engine vs the official CGo bindings over the corpora must agree
   exactly — this spike already does it for two rules; extend the corpus as
   rules migrate. This checks the reimplementation against the reference C
   runtime continuously.
4. **Grammar subset tags** keep unreviewed grammar blobs out of the binary
   (only TS/TSX/JS/Python/Rust/Java/C# get embedded).
5. License: MIT core; the six target grammars are MIT upstream. Vendoring
   includes the license texts, closing the attribution gap smacker suffers
   from.

## 9. Recommendation: conditional GO, phased

The precision result (60%/54.5% → 100%/100% on constructs that occur in
ordinary code review diffs) is the payoff; the pure-Go runtime removes the
only structural blocker (CGo); the costs are parse time (~20x regex,
budgetable, cache-once-per-scan), memory (needs a size cap), binary size
(+~3-5 MB with subset tags), and one seriously-taken dependency (pin +
vendor/fork + differential CI).

- **Phase 0 (done)** — this spike: isolated module, two rules, adversarial
  corpus, benchmarks, CGo reference engine.
- **Phase 1 — validate (no shipping change)**: extend the spike's
  differential harness to large real-world TS corpora (e.g. vendored
  snapshots of a few OSS repos); measure parse-failure rate, pure-Go vs CGo
  parity at node level, memory highwater; file/raise upstream perf+memory
  issues; decide vendor-vs-fork. Exit: parity ≥ 99.99% of nodes, no
  unexplained divergence. *(The differential-validation slice was folded
  into phase 2's tests: per-rule EXPECT-marker corpora pin tree vs regex
  behavior in `tests/checks/`, and the spike module's CGo-vs-pure-Go
  harness stays as the runtime-parity reference. The large-real-world-
  corpus sweep and vendor-vs-fork decision remain open.)*
- **Phase 2 — ship TS behind a flag (done — this change)**: root module
  takes `github.com/odvcencio/gotreesitter` pinned at v0.20.8 (not yet
  vendored/forked — that is the remaining phase-2 supply-chain follow-up,
  §8 item 2); grammar-subset build tags
  (`grammar_subset,grammar_subset_{typescript,tsx,javascript}`) in the
  Makefile and goreleaser keep the release binary delta at ~+5 MB
  (13.6 → 18.9 MB; an untagged `go build`/`go install` embeds all 206
  grammars at ~+27 MB but stays fully functional);
  `Context.ParseScriptFile` + `checks/support.SyntaxTree` seam with a
  corpus-level `sync.Once` tree cache shaped like `parseGo`; migrated
  explicit-any, non-null-assertion, double-assertion, and unsafe-html-sink
  (TS/TSX plus the JS mirror for the sink; the other three are TS-only
  syntax, so JS keeps regex) with tree findings at `confidence: high`;
  `parsers.treesitter: off` by default, `auto` opt-in; per-file regex
  fallback on oversize (> 256 KiB, bounding the ~0.5 MB-heap-per-KB parse
  cost), parse failure, or ERROR nodes covering > 25% of the bytes.
  Differential EXPECT-marker corpora live in
  `tests/checks/testdata/treesitter/` and a tree-path corpus group in
  `tests/corpus/` (`typescript-treesitter`). The new
  rule-health/suppression-stats telemetry compares FP rates in the field.
- **Phase 3 — default-on TS, migrate Python**: flip the default once
  phase-2 telemetry shows strictly fewer suppressions and scan-time growth
  < 2x on the reference repos; port Python rules; delete nothing yet.
- **Phase 4 — Rust/Java, retire hand-rolled parsers per language** after
  two releases of default-on stability each.
- **Standing exit criteria** (any phase): upstream unresponsive to a
  correctness/security issue for 30 days without a workaround → freeze at
  pinned fork and reassess (worst case: the wazero path or CGo-with-split-
  builders remain documented alternatives; the ParserProvider seam makes the
  engine swappable).

## Appendix: raw benchmark output

Medians quoted in §5.2; full `-count 3` output from the clean run:

```
BenchmarkCGoParseOnly/small-10KB-8              1941462 ns/op    5.57 MB/s      11120 B/op        8 allocs/op
BenchmarkCGoParseOnly/large-200KB-8            27559760 ns/op    7.85 MB/s     221424 B/op        8 allocs/op
BenchmarkCGoRulesOnCachedTree/small-10KB-8       688321 ns/op   15.72 MB/s      13160 B/op      583 allocs/op
BenchmarkCGoRulesOnCachedTree/large-200KB-8    13803966 ns/op   15.68 MB/s     263986 B/op    11531 allocs/op
BenchmarkFullScan/baseline-regex/small-10KB-8    615877 ns/op   17.57 MB/s      33886 B/op       56 allocs/op
BenchmarkFullScan/baseline-regex/large-200KB-8 13113272 ns/op   16.51 MB/s     678413 B/op      921 allocs/op
BenchmarkFullScan/official/small-10KB-8         2090234 ns/op    5.18 MB/s      24288 B/op      591 allocs/op
BenchmarkFullScan/official/large-200KB-8       40696362 ns/op    5.32 MB/s     485421 B/op    11539 allocs/op
BenchmarkFullScan/gotreesitter/small-10KB-8    12145828 ns/op    0.89 MB/s    6182034 B/op     3889 allocs/op
BenchmarkFullScan/gotreesitter/large-200KB-8  157953161 ns/op    1.37 MB/s  126142602 B/op    61015 allocs/op
BenchmarkPureGoParseOnly/small-10KB-8          12452328 ns/op    0.87 MB/s    5406970 B/op      806 allocs/op
BenchmarkPureGoParseOnly/large-200KB-8        163024821 ns/op    1.33 MB/s  119563025 B/op       79 allocs/op
BenchmarkPureGoRulesOnCachedTree/small-10KB-8    385460 ns/op   28.08 MB/s     427514 B/op     3000 allocs/op
BenchmarkPureGoRulesOnCachedTree/large-200KB-8 14407905 ns/op   15.02 MB/s    8552496 B/op    59860 allocs/op
```

Research references: official bindings issues #49/#50/#55
(github.com/tree-sitter/go-tree-sitter); smacker/go-tree-sitter (stale since
2024-08, untagged); alexaandru/go-sitter-forest; goreleaser CGo cookbook and
goreleaser-cross; wazero (v1.12.0, zero-dep) and its emscripten import
support; 00f.net 2026 wasm-runtime benchmark (~4.7x native for wazero);
esbuild issue #92 (AST not exported); odvcencio/gotreesitter v0.20.8
(pkg.go.dev; 206 grammars, 116 external scanners; MIT).
