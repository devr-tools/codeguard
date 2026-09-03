# Cross-File Taint Analysis Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Detect Go source-to-sink flows that cross file and in-repo package boundaries, without guessing at unresolved hops and without stale cached verdicts.

**Architecture:** A target-level pre-pass builds `goFuncSummary` values for every non-test Go function, keyed by package-qualified identity, iterating to a fixed point in reverse topological package order. The per-file reporting pass keeps its current structure but resolves call sites against that index after exhausting same-file lookup. Taint gets its own cache section id whose entries carry a package-closure dependency hash.

**Tech Stack:** Go, `go/ast`, `support.BuildGoPackageImportGraph`, `dependency_graph_tarjan.go`, the shared per-scan corpus, the persisted findings cache.

**Spec:** `docs/superpowers/specs/2026-09-03-cross-file-taint-design.md`

**Branch:** `feat/cross-file-taint`

## Global Constraints

- Go only; Python and C++ analyzers are untouched, but the index seam must not be Go-shaped in its exported names.
- An unresolved hop never produces a finding. No allowlists, no assumed propagation.
- Findings anchor at the sink file and line; rule IDs, levels, and fix text are unchanged.
- Fingerprints of currently-detected intra-file flows must not change.
- Chain prose must not participate in fingerprint identity.
- Analysis stays bounded on iterations, hop depth, and indexed functions, and degrades to intra-file behavior with a diagnostic rather than emitting partial chains.
- Diff mode indexes the whole target and reports only changed lines.

**Local toolchain:** `export GOROOT=/opt/homebrew/opt/go/libexec && export PATH=$GOROOT/bin:$PATH`, or use `make` targets. Use `GOCACHE=/private/tmp/codeguard-go-cache`.

---

### Task 1: Cross-file fixtures and expectations (red first)

**Files:**
- Add: `tests/corpus/` fixtures for cross-file flows
- Modify: `tests/corpus/expectations.yaml`
- Test: `tests/checks/security_taint_cross_file_test.go`

**Interfaces:**
- Produces: the ground truth this branch is built against, expressed before any implementation.

- [ ] **Step 1: Write the positive fixtures**

Same-package two-file flow; cross-package flow through an in-repo import; flow through a package cycle; flow through a method on a receiver; flow whose hop is three functions deep.

```go
// handler.go
func Handle(r *http.Request) { run(r.URL.Query().Get("cmd")) }
// exec.go
func run(arg string) { exec.Command("sh", "-c", arg).Run() }
```

- [ ] **Step 2: Write the negative fixtures**

Sanitized flow (`shlex`-style quoting or `strconv` parse before the sink); interface-dispatch hop; function-value-parameter hop; call into a package outside the target. Each asserts zero findings.

- [ ] **Step 3: Run and verify red on positives, green on negatives**

Run: `env -u GOROOT GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/corpus ./tests/checks -run 'Taint.*CrossFile|TestCorpusExpectations' -count=1 -v`

Positives must fail now — that failure is the false negative this branch exists to remove. Negatives passing now must still pass at the end.

---

### Task 2: Package-qualified summary index

**Files:**
- Add: `internal/codeguard/checks/security/security_taint_index.go`
- Modify: `internal/codeguard/checks/security/security_taint_go_types.go`
- Test: `tests/checks/security_taint_index_test.go`

**Interfaces:**
- Produces: `taintSummaryIndex` keyed by `<pkg>.<Func>` and `<pkg>.(<recv>).<Method>`, holding `*goFuncSummary`.
- Consumes: `support.Context`, `core.TargetConfig`, `ParseGoFile` via the shared corpus, `support.BuildGoPackageImportGraph`.

- [ ] **Step 1: Write failing index tests**

Assert identity keying for plain functions, pointer-receiver methods, and value-receiver methods; that `_test.go` files are excluded; that generated and vendored paths follow the existing exclusion rules; and that the index is built once per target per scan.

- [ ] **Step 2: Run and verify red**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/checks -run 'TestTaintSummaryIndex' -count=1`

- [ ] **Step 3: Implement index construction**

Walk the target through the corpus, parse each non-test Go file, and record one summary slot per declaration. Reuse `goTaintAnalyzer.analyzeFunction` for summary computation so intra-function semantics stay identical; only the function map's scope changes.

- [ ] **Step 4: Verify green**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/checks -run 'TestTaintSummaryIndex' -count=1 -race`

---

### Task 3: Fixed point in package order

**Files:**
- Modify: `internal/codeguard/checks/security/security_taint_index.go`
- Modify: `internal/codeguard/checks/security/security_taint_go.go`
- Test: `tests/checks/security_taint_index_test.go`

**Interfaces:**
- Consumes: the package graph's SCCs from `dependency_graph_tarjan.go`.
- Produces: summaries that are final for callees before callers are analyzed, with a bounded iteration count per SCC.

- [ ] **Step 1: Write failing order and bound tests**

Assert a callee's summary is final before its caller's is computed for an acyclic pair; assert a two-package cycle converges; assert a synthetic pathological cycle stops at the iteration bound and records a degradation diagnostic instead of looping.

- [ ] **Step 2: Run and verify red**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/checks -run 'TestTaintSummaryFixedPoint' -count=1`

- [ ] **Step 3: Implement ordered iteration**

Compute SCCs, process in reverse topological order, iterate within an SCC until summaries stabilize or the bound is hit. Preserve the existing three-pass behavior inside a single package.

- [ ] **Step 4: Verify green**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/checks -count=1`

---

### Task 4: Cross-file call resolution and chain rendering

**Files:**
- Modify: `internal/codeguard/checks/security/security_taint_go_calls.go`
- Modify: `internal/codeguard/checks/security/security_taint_go_walk.go`
- Modify: `internal/codeguard/checks/security/security_taint.go` (file-qualified chain steps)
- Test: `tests/checks/security_taint_cross_file_test.go`
- Test: `tests/support/context_fingerprint_test.go`

**Interfaces:**
- Consumes: `taintSummaryIndex`, the file's import declarations and alias bindings.
- Produces: findings anchored at the sink with file-qualified chain steps and `confidence: high`.

- [ ] **Step 1: Write failing resolution and fingerprint tests**

Assert the precedence order same-file → same-package → imported in-repo package; assert an unresolved hop yields no finding and increments an unresolved counter; assert chain messages name the file once the chain leaves the reporting file; assert a currently-detected intra-file flow keeps its exact, context, and content fingerprints.

```go
if crossFile.Fingerprint != intraFileBaseline.Fingerprint {
	t.Fatal("cross-file chain rendering changed intra-file finding identity")
}
```

- [ ] **Step 2: Run and verify red**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/checks ./tests/support -run 'Taint.*CrossFile|Fingerprint' -count=1`

- [ ] **Step 3: Implement resolution and rendering**

Extend call-site handling to consult the index after same-file lookup, thread the callee's file into the chain step, and stop chains at unresolved hops with a counted reason. Keep the dedupe key as sink line, sink name, and source.

- [ ] **Step 4: Verify green including the corpus positives from Task 1**

Run: `env -u GOROOT GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/corpus ./tests/checks ./tests/support -count=1`

---

### Task 5: Cache dependency fingerprint

**Files:**
- Modify: `internal/codeguard/runner/support/cache_types.go`
- Modify: `internal/codeguard/runner/support/findings_scan.go`
- Modify: `internal/codeguard/runner/support/cache_helpers.go`
- Modify: `internal/codeguard/checks/security/security.go` (taint scans under its own section id)
- Test: `tests/support/cache_taint_test.go`

**Interfaces:**
- Adds: `cacheEntry.DepsHash`, populated for the taint section from the transitive package-closure file hashes.
- Bumps: `scanCacheVersion` from 8 to 9.

- [ ] **Step 1: Write failing cache invalidation tests**

Scan, mutate a helper in a dependency package, rescan: assert the caller's taint entry is recomputed and the new cross-file finding appears. Mutate an unrelated package: assert the caller's entry is a cache hit. Assert a v8 cache on disk is discarded rather than reused.

- [ ] **Step 2: Run and verify red**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/support -run 'TestTaintCache' -count=1`

- [ ] **Step 3: Implement the dependency hash**

Give taint its own section id, compute the closure hash from the package graph plus per-file content hashes, store it on the entry, and require it to match for a hit. Bump the cache version.

- [ ] **Step 4: Verify green**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/support ./tests/checks -count=1`

---

### Task 6: Bounds, degradation, diagnostics

**Files:**
- Modify: `internal/codeguard/checks/security/security_taint_index.go`
- Modify: `internal/codeguard/checks/security/security_taint_go.go`
- Test: `tests/checks/security_taint_bounds_test.go`
- Test: `internal/codeguard/report/diagnostics_test.go`

- [ ] **Step 1: Write failing bounds tests**

Assert that exceeding indexed-function, hop-depth, or iteration bounds degrades the target to intra-file analysis with a diagnostic and no partial cross-file finding; assert corpus AST budget exhaustion degrades the same way; assert unresolved-hop counts are reported by reason and are not findings.

- [ ] **Step 2: Run and verify red**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/checks ./internal/codeguard/report -run 'Taint.*(Bounds|Degrad)|Diagnostic' -count=1`

- [ ] **Step 3: Implement bounds and degradation**

- [ ] **Step 4: Verify green**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/... -count=1`

---

### Task 7: Docs, benchmarks, full gate

**Files:**
- Modify: `docs/checks.md` (taint rule descriptions now state cross-file scope and its limits)
- Modify: `docs/features.md`
- Modify: `docs/benchmarks.md` (refreshed detector-quality numbers)
- Modify: `.claude/knowledge/architecture-boundaries.md`

- [ ] **Step 1: Re-run the detector-quality lane and record the numbers**

Run: `env -u GOROOT GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/corpus -run TestCorpusExpectations -v`

Record TP / FN / FP against the 68 / 0 / 1 baseline. Any new false positive blocks the branch.

- [ ] **Step 2: Measure the pre-pass cost**

Full scan with and without the branch on the reference repositories; assert wall clock growth under 15%.

- [ ] **Step 3: Update docs to state the scope honestly**

Cross-file and cross-package within a target; interface dispatch and function values unresolved; out-of-target calls unresolved.

- [ ] **Step 4: Full gate**

Run: `make fmt && make lint && make test && make codeguard-ci`, then strict lint: `env -u GOROOT GOCACHE=/private/tmp/codeguard-go-cache GOLANGCI_LINT_CACHE=/private/tmp/golangci-lint-cache /Users/alex/.go/bin/golangci-lint run` (0 issues), then `go test ./... -race`.
