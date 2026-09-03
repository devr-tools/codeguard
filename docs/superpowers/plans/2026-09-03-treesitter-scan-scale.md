# Tree-Sitter Scan Scale Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the tree-sitter parsing path reachable across a whole repository by replacing scan-lifetime tree retention with a bounded resident cache and heap-aware parse admission, and by counting every fallback.

**Architecture:** `fileCorpus` keeps the `parseScript` signature and its content-hash keying, but its `scripts` map becomes an LRU bounded by measured retained tree bytes, and parse admission reserves transient heap instead of taking a capacity-1 token. Checks are untouched: `support.ScriptSyntaxTree` still returns nil on any refusal and every rule keeps its regex fallback.

**Tech Stack:** Go, `github.com/odvcencio/gotreesitter`, existing corpus diagnostics, `tests/corpus` expectation groups.

**Spec:** `docs/superpowers/specs/2026-09-03-treesitter-scan-scale-design.md`

**Branch:** `feat/treesitter-scan-scale`

## Global Constraints

- No rule semantics change; no rule migrates in this branch.
- `parsers.treesitter` default stays `off`, and the `off` path must be byte-identical to today.
- Tree cache keying stays `path + language + content hash`.
- A tree handed to a section must stay valid for that section's use even if evicted from the cache.
- Fallback must stay silent in findings but visible in diagnostics.
- Per-file `MaxTreeSitterFileBytes` stays 256 KiB.
- Run concurrency-touching tests with `-race`.
- New tests go under `tests/` as external packages (CLAUDE.md). The two exceptions in this plan are the *existing* in-package files `corpus_memory_test.go` and `corpus_script_test.go`, which already assert unexported corpus budgets and are permitted by `ci_rules.allowed_test_paths` in `.codeguard/codeguard.yaml`; extend them rather than adding new in-package test files.

**Local toolchain:** `export GOROOT=/opt/homebrew/opt/go/libexec && export PATH=$GOROOT/bin:$PATH`, or use `make` targets which run `env -u GOROOT go`. Use `GOCACHE=/private/tmp/codeguard-go-cache`.

---

### Task 1: Measure retained tree cost

**Files:**
- Add: `tests/checks/treeprovider_cost_test.go` (measurement needs only the exported `ParseScriptSource`, so it belongs in the external test tree per CLAUDE.md)
- Modify: `docs/superpowers/specs/2026-09-03-treesitter-scan-scale-design.md` (record the measured factors)

**Interfaces:**
- Produces: measured `retainedBytesPerSourceByte` and `transientBytesPerSourceByte` factors for the TypeScript, TSX, JavaScript, and Python grammars.

- [ ] **Step 1: Write a benchmark that separates retained from transient cost**

Parse fixtures of ~1 KB, ~10 KB, ~100 KB and ~250 KB. For transient cost, sample `runtime.MemStats.TotalAlloc` delta across one parse. For retained cost, hold the tree, force `runtime.GC()`, and read `HeapAlloc` against a baseline with the tree dropped. Report bytes per source byte for both.

- [ ] **Step 2: Run it and record the numbers**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/checks -run TestTreeRetainedCost -v -count=1`

- [ ] **Step 3: Write the measured factors into the spec's Resident Cache Model section**

Replace the derivation placeholders with the measured values and the date, so the ceiling defaults below are traceable.

---

### Task 2: Bounded resident tree cache

**Files:**
- Modify: `internal/codeguard/runner/support/corpus.go`
- Modify: `internal/codeguard/runner/support/corpus_access.go` (only if the parse entry point needs a lease-aware signature)
- Test: `internal/codeguard/runner/support/corpus_memory_test.go`
- Test: `internal/codeguard/runner/support/corpus_script_test.go`

**Interfaces:**
- Replaces: `maxTreeSitterScanBytes`, `maxTreeSitterScanFiles`.
- Adds: `maxResidentTreeBytes` ceiling plus LRU eviction over `c.scripts`.
- Preserves: `parseScript(path, data, lang) (*checkSupport.SyntaxTree, error)`.

- [ ] **Step 1: Write failing tests for coverage past the old budgets**

Assert that parsing 300 distinct 8 KiB script files all succeed (no budget error), that the resident set stays at or under the ceiling, that a re-request of an evicted key re-parses and returns an equivalent tree, and that two goroutines racing a cold key parse once.

```go
for i := range 300 {
	tree, err := corpus.parseScript(fmt.Sprintf("file%d.ts", i), source, checkSupport.ScriptLangTypeScript)
	if err != nil {
		t.Fatalf("file %d refused by a scan-wide budget: %v", i, err)
	}
	_ = tree
}
if resident := corpus.residentTreeBytes(); resident > maxResidentTreeBytes {
	t.Fatalf("resident tree bytes %d exceed ceiling %d", resident, maxResidentTreeBytes)
}
```

- [ ] **Step 2: Run the focused tests and verify red**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./internal/codeguard/runner/support -run 'TestCorpus.*(Script|Tree)' -count=1 -race`

- [ ] **Step 3: Implement the LRU**

Track resident bytes using the Task 1 retained factor. On admission, evict least-recently-used entries until the new tree fits. Eviction drops the cache's reference only; a caller that already holds the tree keeps it alive. Keep the per-slot `sync.Once` semantics for cold-key races, and keep entries keyed by content hash. Record an eviction/re-parse counter for Task 4.

- [ ] **Step 4: Verify green with race detection**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./internal/codeguard/runner/support -count=1 -race`

---

### Task 3: Heap-aware parse admission

**Files:**
- Modify: `internal/codeguard/runner/support/corpus.go`
- Test: `internal/codeguard/runner/support/corpus_memory_test.go`

**Interfaces:**
- Replaces: `scriptParse chan struct{}` of capacity 1.
- Adds: an in-flight transient-heap reservation ceiling and a worker cap.

- [ ] **Step 1: Write failing concurrency and admission tests**

Assert that several small-file parses overlap (observed concurrency > 1), that a parse whose reservation exceeds the remaining ceiling waits rather than proceeding, and that total in-flight reserved bytes never exceed the ceiling.

- [ ] **Step 2: Run and verify red**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./internal/codeguard/runner/support -run 'TestScriptParseAdmission' -count=1 -race`

- [ ] **Step 3: Implement reservation-based admission**

Reserve `len(data) * transientFactor` from Task 1 against the ceiling, block until it fits, release on completion. Cap concurrent parses at `min(4, runtime.NumCPU())` independently of the byte ceiling. A reservation larger than the whole ceiling is admitted alone rather than deadlocking.

- [ ] **Step 4: Verify green**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./internal/codeguard/runner/support -count=1 -race`

---

### Task 4: Fallback telemetry

**Files:**
- Modify: `internal/codeguard/runner/support/corpus.go`
- Modify: `internal/codeguard/checks/support/treeprovider_parse.go` (typed refusal reasons)
- Modify: `internal/codeguard/checks/support/treeprovider.go` (`ScriptSyntaxTree` reason pass-through)
- Test: `internal/codeguard/runner/support/corpus_script_test.go`
- Test: `internal/codeguard/report/diagnostics_test.go`

**Interfaces:**
- Produces: per-scan counts of tree-path refusals by reason (`oversize`, `parse_error`, `error_heavy`, `grammar_missing`, `reparse`) and language.
- Consumes: existing `core.Diagnostic` emission via `recordBudget`'s sibling path.

- [ ] **Step 1: Write failing telemetry tests**

Assert that an oversized file, a syntactically broken file, and an error-heavy file each increment their own counter with the right language, that counters aggregate across sections, and that they surface as informational diagnostics which do not become findings and do not change exit status.

- [ ] **Step 2: Run and verify red**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./internal/codeguard/runner/support ./internal/codeguard/report -run 'Tree.*(Fallback|Diagnostic)' -count=1`

- [ ] **Step 3: Implement typed refusal reasons and counting**

Return a typed reason from `ParseScriptSource` refusals instead of bare `fmt.Errorf` strings, thread it through `parseScript`, and tally it on the corpus. Keep `ScriptSyntaxTree`'s nil-on-refusal contract so no check changes.

- [ ] **Step 4: Verify green**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./internal/codeguard/runner/support ./internal/codeguard/report -count=1`

---

### Task 5: Coverage and performance evidence

**Files:**
- Test: `tests/corpus/` (extend the `typescript-treesitter` group)
- Modify: `docs/treesitter-spike.md` (Phase 2 → Phase 3 status and the coverage numbers)
- Modify: `docs/features.md` (Parsers section)
- Modify: `docs/checks.md` (tree-path coverage note)

- [ ] **Step 1: Grow the corpus group past the old budgets**

Add enough TypeScript fixtures to the `typescript-treesitter` group that the old 64-file / 256 KiB budgets would have been exhausted, with expectations that only hold on the tree path.

- [ ] **Step 2: Measure coverage and wall clock on a reference repo**

Scan a vendored snapshot of a real TS repository with `off` and with `auto`. Record: tree-served file count, refusals by reason, wall clock for both modes, peak RSS for both modes.

- [ ] **Step 3: Assert the acceptance thresholds**

Every file at or under 256 KiB is tree-served; zero refusals attributed to a scan-wide budget; `auto` wall clock under 2x `off`; peak retained tree bytes under the ceiling.

- [ ] **Step 4: Update the docs with the measured numbers**

Record the coverage and timing evidence in the spike doc as the Phase 3 readiness input, and note in `docs/features.md` that `auto` now covers whole repositories.

---

### Task 6: Full gate

- [ ] **Step 1: Format, lint, test, self-scan**

Run: `make fmt && make lint && make test && make codeguard-ci`

- [ ] **Step 2: Strict lint**

Run: `env -u GOROOT GOCACHE=/private/tmp/codeguard-go-cache GOLANGCI_LINT_CACHE=/private/tmp/golangci-lint-cache /Users/alex/.go/bin/golangci-lint run` (must be 0 issues)

- [ ] **Step 3: Race pass over the scan path**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./internal/codeguard/runner/... ./tests/... -count=1 -race`

- [ ] **Step 4: Capture knowledge**

Record the measured heap factors and the resident-ceiling rationale in `.claude/knowledge/architecture-boundaries.md` under the scanner hardening invariants.
