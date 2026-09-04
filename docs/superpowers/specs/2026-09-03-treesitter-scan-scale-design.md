# Tree-Sitter Scan Scale Design

## Scope

Phase 2 of the tree-sitter migration (`docs/treesitter-spike.md` §9) shipped the parsing seam, four migrated TypeScript rules, and one migrated Python rule behind `parsers.treesitter: auto`. This change makes that path reachable on real repositories. It replaces the scan-lifetime tree retention model with a bounded resident cache plus heap-aware parse admission, and it makes every refusal of the tree path observable.

It does not migrate additional rules, does not change any rule's semantics, does not flip the `parsers.treesitter` default, and does not alter behavior when the mode is `off`.

## Problem

`fileCorpus.parseScript` (`internal/codeguard/runner/support/corpus.go`) retains every parsed tree in `c.scripts` for the lifetime of the scan so that N rules across N sections pay for one parse. Retention makes peak memory a function of total scanned bytes, which forces the two scan-wide budgets that bound it:

```go
const maxTreeSitterScanBytes = 256 * 1024
const maxTreeSitterScanFiles = 64
```

256 KiB is a whole-scan allowance, not a per-file one, so a repository is covered by the tree path only until the first few files exhaust it. Past that point `parseScript` returns a budget error, `support.ScriptSyntaxTree` returns nil, and each migrated rule silently takes its regex fallback — the path the spike measured at 60% / 54.5% precision against 100% / 100% for the tree path. Parsing is additionally serialized through a capacity-1 channel, so the tree path cannot use more than one core even within its budget.

The consequence is that the precision win Phase 2 paid for is unrealized on any repository larger than a handful of script files, and nothing in the output says so.

## Resident Cache Model

Trees become a bounded resident cache rather than a scan-lifetime map. A tree is retained after parse so concurrent consumers in different sections share it; it is evicted when the resident budget is exceeded, least-recently-used first. A consumer that requests an evicted tree re-parses it. There is no per-scan file count or total-bytes ceiling: coverage is bounded by per-file size only.

The resident budget is expressed in retained tree bytes, not source bytes, and its default is derived from a measurement task rather than assumed. Eviction accounting uses the measured retained cost of each tree so the ceiling means what it says.

Correctness properties the cache must preserve from the current implementation:

- Keying stays `path + language + content hash`, so diff-mode patched content parses separately from on-disk content.
- Concurrent callers racing a cold entry parse exactly once and observe the same tree.
- Returned trees stay immutable and safe for concurrent queries.
- A tree still in use by one section is never freed underneath it; eviction removes the cache's reference only, and the holder's reference keeps it alive.

## Parse Admission

Parse concurrency is governed by transient heap rather than a fixed count. Each admitted parse reserves `len(source) * treeSitterHeapFactor` bytes against a global in-flight ceiling, where the factor comes from the spike's measured ~0.5–0.6 MB of transient heap per KB of TypeScript. A parse waits until its reservation fits. Small files therefore parse concurrently up to a worker cap, while a single large file parses alone.

The per-file refusal at `MaxTreeSitterFileBytes` (256 KiB, `internal/codeguard/checks/support/treeprovider_parse.go`) is unchanged and remains justified by the same heap factor. Raising it is out of scope.

## Observability

Every refusal of the tree path is counted per scan and attributed by reason: file oversize, parse failure, error-heavy tree, grammar not embedded in this build, and resident-cache re-parse. Counts are reported through the existing scan-diagnostic mechanism used by `fileCorpus.recordBudget`, aggregated by language, and are informational: they are not findings, are not baselinable, and do not affect exit status.

This telemetry is the Phase 3 gate. Flipping the `parsers.treesitter` default requires evidence that tree coverage is near-total on reference repositories and that fallback is confined to files the tree path legitimately refuses.

## Non-Goals

Per-file batching of tree-consuming rules — one pass per file that runs every tree rule against one parse and then drops the tree — has a strictly better memory profile than a resident cache, but it requires restructuring how sections wire rules to files. It stays a documented follow-up, taken only if profiling shows re-parse thrash the LRU cannot absorb. The `ParserProvider` seam is unchanged either way.

## Acceptance

- With `parsers.treesitter: off`, findings are byte-identical to the current implementation on the corpus suite and the self-scan.
- With `parsers.treesitter: auto` on a reference TypeScript corpus of at least 200 script files, every file at or under 256 KiB is served by the tree path; the fallback counters attribute every remaining file to a per-file reason, with zero attributed to a scan-wide budget.
- Peak retained tree bytes stay under the configured ceiling under a synthetic corpus larger than that ceiling, asserted in the style of the existing `corpus_memory_test.go` budget tests.
- Full-scan wall clock with `auto` grows less than 2x against `off` on the reference repositories, the Phase 3 exit criterion from the spike.
- `go test ./...` passes with `-race`, since both section and per-file scanning are parallel.
