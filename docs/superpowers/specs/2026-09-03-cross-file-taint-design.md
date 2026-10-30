# Cross-File Taint Analysis Design

## Scope

This change extends Go source-to-sink taint analysis from one file to the target's package closure. It adds a target-level function summary index, resolves call sites across files and imported in-repo packages, and keeps the per-file findings cache correct under cross-file dependence.

Go only. The summary index seam is language-neutral so Python and C++ can follow, but neither is migrated here. No new rules: `security.taint.go`, `security.ssrf.go`, and the other Go taint rule IDs keep their identity, level, and fix guidance.

## Problem

`goTaintAnalyzer` (`internal/codeguard/checks/security/security_taint_go.go`) documents its own limit: intra-file analysis with call-site resolution for functions declared in the same file. It indexes `parsed.Decls` from one file, runs three fixed passes to a summary fixed point, and reports on the third.

The result is a false negative for the most common real shape, where the flow crosses a file boundary:

```go
// handler.go
func Handle(r *http.Request) { run(r.URL.Query().Get("cmd")) }

// exec.go
func run(arg string) { exec.Command("sh", "-c", arg).Run() }
```

Both halves are analyzed, neither sees a flow. `run` is not in `handler.go`'s function map, so the argument's taint stops at the call; `exec.go` has no source, so its parameter sink never fires. Splitting a handler and its helper across files is enough to hide the vulnerability entirely.

## Summary Index

A target-level pre-pass builds one summary per function declaration, keyed by resolved identity rather than bare name:

- package-level function: `<package dir>.<Func>`
- method: `<package dir>.(<recv type>).<Method>`

The summary value is the existing `goFuncSummary` — `returnTaint`, `paramsToReturn`, `paramsToSink` — unchanged in shape. What changes is its scope and lifetime: summaries are computed for every non-test Go file in the target and shared, instead of being rebuilt per file and discarded.

The pre-pass reads and parses through the shared per-scan corpus (`ParseGoFile`), so files already parsed by another section are not re-parsed, and the pass inherits the corpus AST budgets.

## Fixed Point Order

Summaries iterate to a fixed point in reverse topological order of the package import graph, so a callee's summary is final before its callers are analyzed. Both inputs already exist: `support.BuildGoPackageImportGraph` gives the target-local package graph and the file-to-package map, and `dependency_graph_tarjan.go` gives the strongly-connected components. Packages in a cycle iterate together until their summaries stop changing or the iteration bound is reached.

Within a package the existing three-pass structure is retained for intra-package recursion.

## Call Resolution

A call site resolves in a fixed precedence: same file, then same package, then an imported in-repo package via the file's import declarations and alias bindings. Standard library and third-party calls continue to resolve against the existing source/sink/sanitizer models, not the summary index.

Only fully resolved chains produce findings. When a hop cannot be resolved — dynamic dispatch through an interface value, a function value passed as a parameter, a call into a package outside the target — the chain stops and no finding is emitted for it. Unresolved hops are counted as diagnostics by reason, following the house rule that unknown symbols never become findings through a guessed relationship.

Interface method calls are explicitly unresolved in this change. Devirtualization is a follow-up.

## Finding Shape

Findings stay anchored at the sink: the reported path and line are the sink's file and line, so a finding's location is where the dangerous call is. The rendered chain becomes file-qualified once it leaves the reporting file, so the message shows the route:

```
tainted data from http request query (handler.go line 4) reaches exec.Command (exec.go line 3) via run(arg0) -> exec.go:3 exec.Command
```

Cross-file findings carry `confidence: high`, matching the existing intra-file taint findings, because emission still requires a fully resolved chain. Deduplication keeps its current key of sink line, sink name, and taint source, scoped per reporting file.

Fingerprints must not change for flows that are already detected intra-file, so an existing baseline entry for such a finding stays matched. Chain prose is excluded from fingerprint identity.

## Cache Correctness

`cachedFileFindings` keys entries on section, target, path, content hash, and section config hash. Cross-file taint breaks the assumption behind that key: a file's findings now depend on other files' contents, so an unchanged file can have a stale cached verdict after a helper changes.

Taint scanning moves to its own section id so its entries can carry an extra dependency fingerprint: the hash of the file hashes of every file in the transitive package closure of the reporting file's package. A change to a helper invalidates exactly the files whose closure contains it, which keeps diff-mode cache hit rates high because a PR usually touches few packages. `scanCacheVersion` is bumped so existing caches are discarded rather than reused with the old key semantics.

## Bounds and Degradation

Whole-program analysis is not in scope, and the analysis must stay bounded:

- maximum summary iterations per SCC
- maximum chain hop depth
- maximum indexed functions per target
- the existing corpus AST entry and byte budgets

Exceeding any bound degrades to the current intra-file behavior for the affected target and records an informational diagnostic. Degradation must never emit a partial cross-file finding.

Diff mode builds the summary index over the whole target, because a flow's source and sink can sit in unchanged files while the changed line is a hop in between; reporting stays scoped to changed lines as it is today.

## Acceptance

- The existing security corpus baseline holds: 68 true positives, 0 false negatives, at most 1 false positive (`docs/benchmarks.md`).
- New corpus fixtures cover source-to-helper-to-sink across files in the same package, across in-repo packages, through a package cycle, through a method, and through a sanitizer that neutralizes the flow (negative case).
- Unresolved-hop fixtures — interface dispatch, function-value parameter, call into an out-of-target package — produce no findings and increment the unresolved counters.
- Fingerprints for currently-detected intra-file flows are unchanged, proven by a baseline regression test.
- Cache test: changing a helper invalidates the caller's cached taint entry; changing an unrelated package does not.
- The self-scan (`make codeguard-ci`) gains no new findings.
- Full-scan wall clock growth from the pre-pass stays under 15% on the reference repositories.
