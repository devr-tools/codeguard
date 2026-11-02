# Architecture & System Boundaries

Key architectural decisions, service boundaries, data flow, integration points, and why things are the way they are.

## Trust and untrusted input

- **Repository config is attacker-controlled** (codeguard runs on PRs). Command execution and non-allowlisted AI base URLs are gated by the process-global `internal/codeguard/trust` policy, off by default; the trust anchor is env/CLI flags (`--allow-config-commands`, `--allow-config-ai-endpoints`), never config.
- **Every config-derived command must call `trust.GuardConfigCommand` before exec** (e.g. `runner/support/commands.go`, `ai/runtime`, `ai/nlrule`, `ai/semantic`, `runner/govulncheck` for a `govulncheck_command` override). codeguard's own fixed invocations (git, built-in `govulncheck` from PATH, the `node -e` TS runner) stay ungated. `execlimit` does no gating itself.
- **Outbound AI HTTP goes through `ai/safehttp`** (`ValidateProviderURL` allowlist, a `Client` that blocks private/loopback at dial time, `MaxResponseBytes`). Only an env-sourced base URL is trusted (`BaseURLFromEnv`), never one from config.
- **Git subprocesses:** validate `base_ref` with `runnersupport.ValidateBaseRef` (rejects leading `-`) and pass `--end-of-options`. Use `exec.CommandContext` with a timeout, and read output through a `LimitReader`. The `//nolint:contextcheck` markers are deliberate: don't remove them without actually threading ctx.
- Config artifact paths (`baseline.path`, `cache.path`, `ai.cache.path`) resolve relative to the config dir and must stay inside it (`config.containConfigArtifactPaths`).
- MCP caller-supplied `config_path` is confined to the server config dir, the cwd and client roots (`confinePath`). This fixed a remote file read.
- The no-arg/`help` "What's New" banner makes the one outbound call. It is off when `CODEGUARD_NO_UPDATE_CHECK` or `$CI` is set. `CHANGELOG.md` is embedded from the root `changelog.go` because `go:embed` can't reach `../`.

## Package boundaries

- **Checks must not import `internal/codeguard/runner/...`**, with no allowlist (enforced by `tests/architecture`). Reach runner services through `checks/support.Context` hooks. Put shared helpers in `checks/support` or a leaf package (`execlimit`, `cachefile`, `benchregression`, `buildregression`). There is no fallback for a nil `env.ListTargetFiles` hook, so unit contexts must supply it.
- **Score history is recorded by the runner, not checks**: checks publish `slop_score`/`performance_score`/`repo_legibility` artifacts, and `runnersupport.RecordScoreHistories` writes trends after `Build`. Write failures appear as `scan.history-write` diagnostics in the final report only, not in `OnSectionComplete` streams.
- `internal/codeguard/history` (git-history secret scan) must stay its own package: `checks/security` imports `runner/support`, so putting it there would create a cycle. Its parser uses `bufio.Reader`, not `bufio.Scanner`, because Scanner silently stops on long lines.
- The MCP server is hand-rolled. A new method must be wired into both `dispatchSyncMethod` (`mcp_dispatch.go`, used by HTTP) and the stdio switch in `mcp_run.go`. MCP fix-verification test runs are trust-gated, so they need `CODEGUARD_ALLOW_CONFIG_COMMANDS=1`.

## Scanner, corpus and cache

- **File-level scans run in parallel.** `ScanTargetFiles` runs up to 4 workers inside a NumCPU-bounded section pool. Any shared mutable state reached from an evaluator needs a lock; an unlocked go.mod resolver cache crashed with concurrent map writes, but only on the third run. Evaluators that rely on serial order must use `ScanTargetFilesSequential`.
- **The `ScanTargetFiles` per-file cache requires pure evaluators.** On a cache hit the evaluator is skipped, so closure-built state silently vanishes. Use `VisitTargetFiles` for cross-file or stateful passes (as clone detection does). Return diagnostics through `ScanTargetFilesWithDiagnostics`, never as a side effect.
- **The cache key is `(sectionID, target, rel)`**: a second `ScanTargetFiles` pass in the same section needs a distinct id (e.g. `"security-secrets"`), or it gets the first pass's findings.
- **The cache fingerprint is per config family** (`SectionConfigHashes`/`sectionConfigFamily` in `cache_helpers.go`). A new section must be added there. Finalize-time-only settings (`min_confidence`, `confidence_demotion`) must be stripped in `findingRelevantChecks` so that changing them re-renders without invalidating the cache.
- **Finding fingerprints use only rule, path, line and source identity**, never message, metadata, confidence or reported level. That is what lets demoted findings still match baselines.
- **The shared corpus** (`runner/support/corpus.go`) reads each file once, caps reads at 32 MiB, and memoizes Go ASTs (`ParseGoSource`) and C-like parses (`ParseCLikeFile`). Cached ASTs and `*ParsedFile` slices are read-only: never sort or append to them. Fixed-name reads (`go.mod`, root `package.json`) stay on `os.ReadFile` on purpose. Read walk- or diff-derived content through the corpus (`env.ReadTargetFile`). A few legacy direct reads, such as `contracts.go` `readHead` and the Python/C++ toolchain dead-code reads, have no size cap at all, so don't copy them.
- **Corpus budgets are counted in retained bytes and only stop caching**: over-budget work is served uncached, never skipped. Keep it that way, because skipping files on a budget miss caused silent false negatives. Analysis-specific budgets, such as clone detection's `quality.duplicate-code-budget`, do truncate, so check `*-budget` diagnostics before calling a missing finding a detector gap.
- **Never `break` or `return` early inside a `range` over a map** in a capped or first-match scan. Go randomizes map order, so the findings become nondeterministic; iterate a sorted key slice instead. This bit clone candidates and data clumps.
- Hot per-line helpers should return `nil` when they find nothing, not `make([]T, 0, N)` with N>0: that allocates every call and cost ~1 GB per scan pass.
- `LooksBinary` is applied centrally in `scanTargetFiles`, but not in `VisitTargetFiles` or the corpus read path. A new raw-read line scanner must guard itself. The secrets scanner also caps files at 5 MiB and lines at 64 KiB.
- **Exclusions** live in `runner/support/utils_match.go` in three tiers: segment-matched unambiguous dirs, generated file suffixes, and ambiguous names (`build`, `out`, `target`, `coverage`…) that are pruned only when a sibling manifest corroborates them (`isCorroboratedBuildOutputDir`). That corroboration happens at walk time only, so path-only callers (`ShouldExclude`, diff mode) still see those dirs. A leading `!` in `exclude` re-includes, and a wildcard-led re-include disables all directory pruning. Don't memoize `dirHasEntryWithSuffix`: the long-lived MCP server would serve stale results.

## Sections and config

- Sections are data-driven through `sectionRegistry` (`runner/checks/registry.go`), and its order is the output order. `SafeRun` turns a panic into a fail-level `scan.section.panic` diagnostic that no exit policy can relax.
- Adding a section also requires: a `CheckConfig` bool and a rules struct, catalog and fix-template maps, defaults, docs, and the cache family above. Top-level `CheckConfig` bools default to **off** when omitted.
- Registry id `supply-chain` vs report id `supply_chain`: user-facing ids come from `core.SectionKeys()`.
- **`ApplyDefaults` uses `baseDefaultConfig()`** (`config/defaults_base*.go`). `baseExampleConfig()` adds `applyStarterPolicy()` and is what `init` writes. A config-less scan uses repository-aware `initconfig.DefaultScanConfig`. Anything added to the baseline applies to every user, so repository-shape policy (filenames, test layout, layering) belongs in the starter or a profile. A `def` value only takes effect if some `apply*Defaults` actually reads it.
- `config.WriteFile` stamps `codeguard_version`. `ApplyDefaults` does not, the stamp is never validated, and it is excluded from cache hashes.
- `design_rules.forbid_internal_import_cmd` is defaulted and documented, but no check reads it.
- **The SDK facade (`design_rules.sdk_facade_paths`, `SDKFacadeForTarget`) exempts only `design.service-import-internal`.** `design.service-import-cmd` deliberately has no exemption. The default heuristic treats every `pkg/` dir as a facade, so sharing the exemption would silently retire both fail rules.
- Inline `codeguard:ignore <rule>` matches an exact id (or alias) or a bare `*` only. A prefix glob like `security.*` parses but silently suppresses nothing.
- **1.x compatibility defaults (keep them):** an MCP `scan`/`validate_patch` call with no new inputs returns the legacy full report. `quality_rules.cyclomatic_counting` defaults to `legacy`; the self-scan opts into `gocyclo`. The SDK risk weights stay `int`, with explicit zero tracked by key presence (`SetExplicit`).

## Detector invariants

- **The secret-scanner literal gate (`builtinGatePasses`/`gateLiterals`) must be a superset of every built-in pattern**: a new credential pattern needs its literal in the gate (`TestBuiltinGateCoversPatterns`). The test generates concrete strings from each regex, and `TestSecurityGatedPatternsAreSupersets` does the same for the per-line gates. Patterns match the raw line. Placeholder filtering happens in Go after the match, because RE2 has no lookahead. Unlike the per-line gates, `builtinGatePasses` has no non-ASCII bail-out, so `(?i)` Unicode folds (`ſ`, `K`) are a known false negative. Fixing it only adds findings, so make that change deliberately, with tests.
- Secrets run as a separate pass for both TS and non-TS targets. Other line rules (`security_owasp_extra.go`, `findingsForFile`) do **not** run on TS/JS, which goes through `typeScriptTargetFindings`.
- **Fixture handling:** path evidence can only demote (`classifyPathScope`), because paths are attacker-chosen. Suppression requires value evidence, so keep value signals exact. `private_key`/`high_entropy` are never path-demoted. `security_rules.demote_fixture_findings: false` restores strict levels (it is live, despite the struct comment). The history scan ignores fixture classification.
- Provider patterns with no capture group get no "example" suppression from `isPlaceholderSecret`. Vendor placeholders (`AKIAIOSFODNN7EXAMPLE`) use an exact denylist. Re-check this when adding a provider.
- CodeQL `go/regex/missing-regexp-anchor` fires on host literals in regexes. Bound them on both sides with `(?:^|[^A-Za-z0-9_])(...)(?:$|[^A-Za-z0-9_])`.
- OWASP/CWE mappings are applied in the `catalog` var initializer (`withRuleCWE(withSecurityOWASP(...))`), not in `init()`, which would run too late.
- Taint SSRF routing: to add a sink, extend the language's HTTP-sink map (e.g. `goHTTPSinkArgIndex`, wired through `taintRules.isSSRFSink`). The `security.ssrf.*` rule id follows automatically.
- `parseLockfileState`'s bool means "format recognised", not "packages found". Returning true on garbage turns every dependency into a fail-level `lockfile-drift`. Name comparison is ecosystem-gated: PEP 503 normalisation for Python only, while npm and Cargo names stay exact.
- `goFuncEligibleForUnusedCheck` must not consult `fn.Doc`: it is populated now (ParseComments corpus), and using it would change findings.
- Clone hashing is non-semantic (candidates are value-verified). Never regenerate `tests/checks/quality_clone_differential_test.go` expectations from current code.