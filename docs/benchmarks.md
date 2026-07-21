# Frozen PR benchmarks

CodeGuard benchmarks PR-time scans using a small, versioned corpus of frozen
public pull-request checkouts. The repository does not fetch or vendor those
projects: the benchmark operator provisions each checkout at the exact commit
listed in the manifest, then runs the harness locally or in a dedicated CI
job.

The manifest schema is versioned and machine-readable. Each entry records a
repository, PR number, immutable base/head revisions, language, a worktree
name relative to `-work-root`, and a relative CodeGuard configuration path.
Use [manifest.example.json](../benchmarks/manifest.example.json) as the
onboarding template. Do not replace immutable revisions with branch names.

Export the corpus identity for an auditable result bundle:

```sh
go run ./cmd/codeguard-benchmark export \
  -manifest benchmarks/manifest.json -out corpus.json
```

After provisioning the listed worktrees beneath a single directory, measure
each diff scan:

```sh
go run ./cmd/codeguard-benchmark run \
  -manifest benchmarks/manifest.json \
  -work-root /private/tmp/codeguard-benchmark-worktrees \
  -binary ./dist/codeguard -warm-repeats 3 -out results.json
```

Results contain a first-process `cold` run and the requested `warm` repeats
for each entry. “Cold” means a fresh CodeGuard process; it does not claim to
clear the host filesystem cache or alter the repository's configured cache.
Record p50/p95 separately for cold and warm runs, and include the exported
corpus metadata next to any published figures.

Runtime is only one benchmark lane. Use the existing ground-truth detector
corpus for precision/noise, and verified-fix fixtures for proposal coverage,
verifier acceptance, and independently validated acceptance. Do not collapse
the three lanes into a single competitive score.
