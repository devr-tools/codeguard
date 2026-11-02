# Rule index

Every rule in the built-in catalog, grouped by the report section that emits it,
with its default level and a one-line summary taken from the rule metadata.

- Narrative behavior, configuration, and examples for each section live in
  [checks.md](checks.md); this page is the complete list.
- `codeguard explain <rule-id>` prints the full metadata for one rule:
  description, how to fix, fix template, language coverage, and OWASP mapping.
- A rule's *default level* is the level it reports at unless a policy profile
  or a section-specific threshold changes it. A few rules
  escalate from `warn` to `fail` when a stricter condition holds; the summary
  says so when that applies.
- Each row carries a stable anchor, `#rule-<id with dots replaced by hyphens>`
  (`security.taint.go` → `#rule-security-taint-go`), which SARIF `helpUri`
  values and `codeguard explain` link to.
- `tests/rules/docs_coverage_test.go` fails when a catalog rule is missing from
  this page or `checks.md`, so add a row (with its anchor) when adding a rule.

Catalog size: 375 rules.

## Code Quality

72 rules. Section reference: [checks.md](checks.md#quality).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-function-hidden-mutation"></a>`function.hidden-mutation` | warn | Warns when a function mutates inputs, collaborators, or state without a command-style name. |
| <a id="rule-function-inconsistent-return-contract"></a>`function.inconsistent-return-contract` | warn | Warns when one function mixes empty and value return shapes without a clear contract. |
| <a id="rule-function-multiple-responsibilities"></a>`function.multiple-responsibilities` | warn | Warns when one function combines several semantic responsibilities such as validation, loading, writing, sending, caching, transforming, or observing. |
| <a id="rule-function-orchestration-domain-mix"></a>`function.orchestration-domain-mix` | warn | Warns when handlers, controllers, jobs, or workers mix request/job orchestration with domain decisions. |
| <a id="rule-function-partial-result"></a>`function.partial-result` | warn | Warns when a function can return a value together with an error without an explicit partial-result contract. |
| <a id="rule-naming-behavior-mismatch"></a>`naming.behavior-mismatch` | warn | Warns when query/build/format names perform side effects or command-style names only read. |
| <a id="rule-naming-boolean-not-predicate"></a>`naming.boolean-not-predicate` | warn | Warns when boolean variables, parameters, or boolean-returning functions do not read like predicates. |
| <a id="rule-naming-cardinality-mismatch"></a>`naming.cardinality-mismatch` | warn | Warns when a plural name holds a single non-numeric, non-boolean scalar, or when a singular name holds a collection of the type it is named after (user []User). Counts (maxLines), flags (lineNumbers), byte strings, argv vectors, keyed maps, and mass nouns are not reported. |
| <a id="rule-naming-cross-layer-inconsistency"></a>`naming.cross-layer-inconsistency` | warn | Warns when API/domain/persistence layer names use different terms for the same starter domain concept. |
| <a id="rule-naming-domain-vocabulary-drift"></a>`naming.domain-vocabulary-drift` | warn | Warns when configured glossary concepts appear under multiple terms in the same source. |
| <a id="rule-naming-implementation-leak"></a>`naming.implementation-leak` | warn | Warns when domain-facing names encode infrastructure details such as SQL, HTTP, Redis, Kafka, or ORM. |
| <a id="rule-naming-missing-unit"></a>`naming.missing-unit` | warn | Warns when numeric duration, size, or money names omit a unit suffix. |
| <a id="rule-naming-role-suffix-overuse"></a>`naming.role-suffix-overuse` | warn | Warns when a file repeatedly relies on vague suffixes such as Manager, Helper, Util, Service, or Processor. |
| <a id="rule-naming-unknown-abbreviation"></a>`naming.unknown-abbreviation` | warn | Warns when identifiers contain abbreviations that are not common or configured for the repository. |
| <a id="rule-quality-ai-change-risk"></a>`quality.ai.change-risk` | warn | Warns when an AI-assisted or AI-shaped change accumulates enough risky signals that it should receive stricter review before merge. |
| <a id="rule-quality-ai-contract-drift"></a>`quality.ai.contract-drift` | warn | Warns when optional LLM-assisted semantic review finds that a changed function appears to drift from its prior behavior or surrounding contract without a clear caller-facing update. |
| <a id="rule-quality-ai-dead-code"></a>`quality.ai.dead-code` | warn | Warns when code contains unreachable constant-condition branches, statements after unconditional returns or raises, or private functions that are never referenced, all common leftovers in AI-generated patches. |
| <a id="rule-quality-ai-error-style-drift"></a>`quality.ai.error-style-drift` | warn | Warns when a file's error-handling style diverges from the repository's dominant convention, such as unwrapped errors in a %w-wrapping Go codebase, bare except clauses in a typed-except Python codebase, or raw Error throws in a custom-error-class TypeScript codebase. |
| <a id="rule-quality-ai-hallucinated-import"></a>`quality.ai.hallucinated-import` | warn | Warns when an import cannot be resolved against local module manifests, declared dependencies, standard libraries, or local files, which is a common AI-generated code failure mode. |
| <a id="rule-quality-ai-local-idiom-drift"></a>`quality.ai.local-idiom-drift` | warn | Warns when new tests diverge from the repository's dominant local testing idiom, which is a common smell in AI-generated contributions that are technically valid but inconsistent. |
| <a id="rule-quality-ai-naming-drift"></a>`quality.ai.naming-drift` | warn | Warns when a file's identifier naming convention (snake_case vs camelCase) diverges from the repository's dominant convention, a common smell in AI-generated contributions. |
| <a id="rule-quality-ai-narrative-comment"></a>`quality.ai.narrative-comment` | warn | Warns when comments merely narrate nearby code instead of explaining intent, constraints, or non-obvious tradeoffs, a common LLM failure mode. |
| <a id="rule-quality-ai-over-mocked-test"></a>`quality.ai.over-mocked-test` | warn | Warns when a test leans heavily on mocks and stubs with little direct assertion signal, a common sign that AI-generated tests are only verifying the mock setup. |
| <a id="rule-quality-ai-provenance-policy"></a>`quality.ai.provenance-policy` | warn | Applies stricter review thresholds when the current change is tagged as AI-assisted through configured environment hints or commit trailers. |
| <a id="rule-quality-ai-semantic-doc-mismatch"></a>`quality.ai.semantic-doc-mismatch` | warn | Warns when optional LLM-assisted semantic review finds that a changed function name or adjacent documentation no longer matches the implementation. |
| <a id="rule-quality-ai-semantic-error-message"></a>`quality.ai.semantic-error-message` | warn | Warns when optional LLM-assisted semantic review finds that a changed error message misstates the actual failing condition, resource, or recovery path. |
| <a id="rule-quality-ai-semantic-runtime"></a>`quality.ai.semantic-runtime` | fail | Fails when semantic review was enabled but the configured command was missing, crashed, or returned invalid output, so the scan would otherwise lose semantic coverage silently. |
| <a id="rule-quality-ai-semantic-test-adequacy"></a>`quality.ai.semantic-test-adequacy` | warn | Warns when optional LLM-assisted semantic review finds that nearby tests for a changed behavior look weak, overly happy-path, or otherwise inadequate for the risk introduced by the change. |
| <a id="rule-quality-ai-semantic-test-coverage"></a>`quality.ai.semantic-test-coverage` | warn | Warns when optional LLM-assisted semantic review finds that changed behavior does not appear to be exercised by nearby repository tests. |
| <a id="rule-quality-ai-swallowed-error"></a>`quality.ai.swallowed-error` | warn | Warns when code drops errors through blank assignments, empty catch blocks, or pass-only exception handlers that are common in AI-generated patches. |
| <a id="rule-quality-ambiguous-name"></a>`quality.ambiguous-name` | warn | Warns when a local identifier uses vague names such as data, manager, helper, process, or thing. |
| <a id="rule-quality-boolean-argument"></a>`quality.boolean-argument` | warn | Warns when function parameters hide alternate behavior behind boolean flags. |
| <a id="rule-quality-command-check"></a>`quality.command-check` | fail | Fails when a configured language-specific quality command exits non-zero. |
| <a id="rule-quality-coverage-delta"></a>`quality.coverage-delta` | warn | Warns when the test coverage of changed lines in a diff scan falls below the configured threshold. Opt-in (quality_rules.coverage_delta.enabled) because it runs the target's tests during the scan; only active in diff mode. Go targets run go test -coverprofile natively, other languages run a configured coverage command and parse its lcov report. |
| <a id="rule-quality-cpp-clang-format"></a>`quality.cpp.clang-format` | fail | Fails when an explicitly configured C++ target is not clang-format clean or the required formatter cannot run. |
| <a id="rule-quality-cpp-compiler-parse"></a>`quality.cpp.compiler-parse` | fail | Validates target-local compilation database entries with a sanitized clang++ syntax-only invocation. |
| <a id="rule-quality-cyclomatic-complexity"></a>`quality.cyclomatic-complexity` | warn | Warns when a function exceeds the configured cyclomatic complexity threshold. |
| <a id="rule-quality-dead-code-toolchain"></a>`quality.dead-code.toolchain` | warn | Warns when opt-in toolchain or language-graph evidence indicates code is unreachable from configured entrypoints. Go uses linker dependency evidence; Rust uses cargo/rustc dead_code and unreachable_code diagnostics plus configured artifact symbol reports; C++ ingests trusted linker map, nm, and dumpbin-style artifacts when configured and otherwise falls back to target-local include and C++20 named-module graph evidence; Python uses static import reachability plus trusted reports; TypeScript and JavaScript use compiler semantic diagnostics plus trusted bundler reports. |
| <a id="rule-quality-dependency-direction"></a>`quality.dependency-direction` | warn | Warns when a Go package imports an internal tree it is outside of, which Go's internal-visibility rule does not permit. Same-module internal imports (cmd, pkg facades, external test packages) are legal and are not reported. |
| <a id="rule-quality-duplicate-code"></a>`quality.duplicate-code` | warn | Warns when files contain long duplicated normalized token sequences above the configured threshold. |
| <a id="rule-quality-duplicated-knowledge"></a>`quality.duplicated-knowledge` | warn | Warns when domain literals or rules appear duplicated instead of living behind one named concept. |
| <a id="rule-quality-environment-branching"></a>`quality.environment-branching` | warn | Warns when domain or application source code branches on production/staging/development environment instead of isolating that policy in configuration or bootstrap boundaries. |
| <a id="rule-quality-gofmt"></a>`quality.gofmt` | fail | Fails when a Go source file is not gofmt-clean. |
| <a id="rule-quality-hidden-side-effect"></a>`quality.hidden-side-effect` | warn | Warns when a function name implies a query or builder but the body mutates state, publishes, writes, or sends. |
| <a id="rule-quality-javascript-debugger-statement"></a>`quality.javascript.debugger-statement` | warn | Warns when JavaScript code contains committed debugger statements. |
| <a id="rule-quality-javascript-double-assertion"></a>`quality.javascript.double-assertion` | warn | Warns when JavaScript code uses chained assertions such as as unknown as T to bypass type safety. |
| <a id="rule-quality-javascript-explicit-any"></a>`quality.javascript.explicit-any` | warn | Warns when JavaScript code opts out of type safety with explicit any in type-checked syntax or annotations. |
| <a id="rule-quality-javascript-non-null-assertion"></a>`quality.javascript.non-null-assertion` | warn | Warns when JavaScript code forces nullability away with the non-null assertion operator. |
| <a id="rule-quality-javascript-ts-expect-error"></a>`quality.javascript.ts-expect-error` | warn | Warns when JavaScript code uses @ts-expect-error to suppress type checking. |
| <a id="rule-quality-javascript-ts-ignore"></a>`quality.javascript.ts-ignore` | warn | Warns when JavaScript code uses @ts-ignore to suppress type checking. |
| <a id="rule-quality-javascript-ts-nocheck"></a>`quality.javascript.ts-nocheck` | warn | Warns when JavaScript code disables type checking with @ts-nocheck. |
| <a id="rule-quality-max-file-lines"></a>`quality.max-file-lines` | warn | Warns when a file exceeds the configured maximum line count and escalates to a failure when the same file also exceeds cyclomatic complexity limits. |
| <a id="rule-quality-max-function-lines"></a>`quality.max-function-lines` | warn | Warns when a function exceeds the configured maximum line count. |
| <a id="rule-quality-max-parameters"></a>`quality.max-parameters` | warn | Warns when a function exceeds the configured parameter count. |
| <a id="rule-quality-mutable-global-state"></a>`quality.mutable-global-state` | warn | Warns when production code declares mutable module or package state. |
| <a id="rule-quality-parse-error"></a>`quality.parse-error` | fail | Fails when a Go source file cannot be parsed. |
| <a id="rule-quality-primitive-obsession"></a>`quality.primitive-obsession` | warn | Warns when related domain values are passed around as raw strings, numbers, or booleans. |
| <a id="rule-quality-redundant-comment"></a>`quality.redundant-comment` | warn | Warns when a comment repeats nearby code instead of explaining intent, constraints, or tradeoffs. |
| <a id="rule-quality-typescript-debugger-statement"></a>`quality.typescript.debugger-statement` | warn | Warns when TypeScript code contains committed debugger statements. |
| <a id="rule-quality-typescript-double-assertion"></a>`quality.typescript.double-assertion` | warn | Warns when TypeScript code uses chained assertions such as as unknown as T to bypass type safety. |
| <a id="rule-quality-typescript-explicit-any"></a>`quality.typescript.explicit-any` | warn | Warns when TypeScript code opts out of type safety with explicit any. |
| <a id="rule-quality-typescript-non-null-assertion"></a>`quality.typescript.non-null-assertion` | warn | Warns when TypeScript code forces nullability away with the non-null assertion operator. |
| <a id="rule-quality-typescript-ts-expect-error"></a>`quality.typescript.ts-expect-error` | warn | Warns when TypeScript code uses @ts-expect-error to suppress type checking. |
| <a id="rule-quality-typescript-ts-ignore"></a>`quality.typescript.ts-ignore` | warn | Warns when TypeScript code uses @ts-ignore to suppress type checking. |
| <a id="rule-quality-typescript-ts-nocheck"></a>`quality.typescript.ts-nocheck` | warn | Warns when a TypeScript file disables type checking with @ts-nocheck. |
| <a id="rule-smell-data-clump"></a>`smell.data-clump` | warn | Warns when the same group of primitive/domain parameters appears repeatedly across functions. |
| <a id="rule-smell-feature-envy"></a>`smell.feature-envy` | warn | Warns when a function or method mostly interrogates one external collaborator instead of its own receiver/context. |
| <a id="rule-smell-god-object"></a>`smell.god-object` | warn | Warns when a local type/class accumulates many methods, fields, and responsibility clusters. |
| <a id="rule-smell-message-chain"></a>`smell.message-chain` | warn | Warns when code reaches through a long chain of collaborators, increasing coupling to object structure. |
| <a id="rule-smell-middle-man"></a>`smell.middle-man` | warn | Warns when a type/class mostly forwards calls to one collaborator without policy, translation, or ownership. |
| <a id="rule-smell-refused-bequest"></a>`smell.refused-bequest` | warn | Warns when a derived/embedded type inherits a contract but explicitly refuses multiple inherited-style methods with no-op or unsupported implementations. |
| <a id="rule-smell-switch-on-type"></a>`smell.switch-on-type` | warn | Warns when behavior repeatedly branches on type/kind/discriminator checks that should move behind polymorphism or dispatch. |

## Code Quality / Local Precision

32 rules. Section reference: [checks.md](checks.md#quality).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-defensive-bounds-assumption"></a>`defensive.bounds-assumption` | warn | Warns when indexed access assumes collection bounds without a nearby length check. |
| <a id="rule-defensive-fail-open-authorization"></a>`defensive.fail-open-authorization` | warn | Warns when authorization failure paths default to allow or success. |
| <a id="rule-defensive-integer-overflow"></a>`defensive.integer-overflow` | warn | Warns when arithmetic on count, size, or length input lacks an overflow bound check. |
| <a id="rule-defensive-invalid-state-representable"></a>`defensive.invalid-state-representable` | warn | Warns when booleans or raw status strings can represent impossible state combinations. |
| <a id="rule-defensive-invalid-state-transition"></a>`defensive.invalid-state-transition` | warn | Warns when state transitions write terminal states without checking allowed prior state. |
| <a id="rule-defensive-missing-resource-limit"></a>`defensive.missing-resource-limit` | warn | Warns when boundary reads or uploads lack explicit size, count, or time limits. |
| <a id="rule-defensive-missing-schema-validation"></a>`defensive.missing-schema-validation` | warn | Warns when decoded JSON, events, or request payloads are used without schema or invariant validation. |
| <a id="rule-defensive-non-exhaustive-branch"></a>`defensive.non-exhaustive-branch` | warn | Warns when enum-like state/kind/type branching lacks default or exhaustive handling. |
| <a id="rule-defensive-null-assumption"></a>`defensive.null-assumption` | warn | Warns when a value whose declared type says it may be absent (?, \| null, \| undefined, \| None, Optional, Maybe, std::optional) is dereferenced without a guard. A bare pointer is the pass-by-reference idiom, not a nullability declaration, and is not reported. |
| <a id="rule-defensive-sequence-collision-risk"></a>`defensive.sequence-collision-risk` | warn | Warns when external ID allocation derives the next value from current count, including retry-mitigated cases that remain architectural debt. |
| <a id="rule-defensive-unchecked-external-response"></a>`defensive.unchecked-external-response` | warn | Warns when external responses are consumed without checking status, ok, or transport errors. |
| <a id="rule-defensive-unchecked-type-assertion"></a>`defensive.unchecked-type-assertion` | warn | Warns when a type assertion or cast bypasses runtime validation or omits the safe comma-ok form. |
| <a id="rule-defensive-unsafe-default"></a>`defensive.unsafe-default` | warn | Warns when a config/env fallback can fail open or disable a safety control. |
| <a id="rule-defensive-unsafe-numeric-conversion"></a>`defensive.unsafe-numeric-conversion` | warn | Warns when a narrowing or sign-changing numeric conversion can truncate, wrap, or lose precision. |
| <a id="rule-defensive-unvalidated-boundary-input"></a>`defensive.unvalidated-boundary-input` | warn | Warns when handler, API, event, or filesystem input is consumed without validation evidence. |
| <a id="rule-error-cleanup-error-ignored"></a>`error.cleanup-error-ignored` | warn | Warns when close, rollback, delete, or cleanup failures are discarded. |
| <a id="rule-error-context-lost"></a>`error.context-lost` | warn | Warns when an error is rethrown or returned bare from a lower-level call without contextual wrapping. |
| <a id="rule-error-exception-used-for-control-flow"></a>`error.exception-used-for-control-flow` | warn | Warns when exception/panic/throw is used for ordinary branch control. |
| <a id="rule-error-fallback-hides-corruption"></a>`error.fallback-hides-corruption` | warn | Warns when fallback success after parse, corruption, or validation failure can hide bad data. |
| <a id="rule-error-generic-message"></a>`error.generic-message` | warn | Warns when an error message lacks operation, resource, or decision context. |
| <a id="rule-error-inconsistent-wrapping"></a>`error.inconsistent-wrapping` | warn | Warns when a function mixes wrapped errors with bare error returns. |
| <a id="rule-error-logged-and-ignored"></a>`error.logged-and-ignored` | warn | Warns when an error is logged and then ignored, converted to a success value, or allowed to continue without propagation. |
| <a id="rule-error-logged-and-returned"></a>`error.logged-and-returned` | warn | Warns when the same error is logged and returned, risking duplicate logs at multiple layers. |
| <a id="rule-error-panic-on-recoverable-path"></a>`error.panic-on-recoverable-path` | warn | Warns when recoverable request, validation, or I/O failures are handled with panic/throw. |
| <a id="rule-error-partial-failure-hidden"></a>`error.partial-failure-hidden` | warn | Warns when a partial failure path continues or reports success without surfacing failed work. |
| <a id="rule-error-retryable-not-distinguished"></a>`error.retryable-not-distinguished` | warn | Warns when retry paths cannot distinguish transient from permanent failures. |
| <a id="rule-error-user-message-leaks-internals"></a>`error.user-message-leaks-internals` | warn | Warns when user-facing errors expose database, transport, stack, or infrastructure internals. |
| <a id="rule-error-wrong-abstraction-level"></a>`error.wrong-abstraction-level` | warn | Warns when higher-level error contracts expose lower-level infrastructure details. |
| <a id="rule-function-command-query-mix"></a>`function.command-query-mix` | warn | Warns when a function returns a value while also invoking mutating side-effect operations. |
| <a id="rule-function-excessive-parameters"></a>`function.excessive-parameters` | warn | Warns when a function exceeds the configured parameter threshold and should likely group related inputs or split responsibilities. |
| <a id="rule-function-mixed-abstraction-level"></a>`function.mixed-abstraction-level` | warn | Warns when one function combines orchestration-level calls with low-level infrastructure operations such as SQL, HTTP, filesystem, or environment access. |
| <a id="rule-naming-generic-identifier"></a>`naming.generic-identifier` | warn | Warns when a function, parameter, or local variable uses placeholder names such as foo, tmp, thing, or obj instead of domain vocabulary. |

## Code Smells / History

2 rules. Section reference: [checks.md](checks.md#quality).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-smell-divergent-change-history"></a>`smell.divergent-change-history` | warn | Warns when a changed file has recent commit subjects spanning several concern families. |
| <a id="rule-smell-shotgun-surgery-history"></a>`smell.shotgun-surgery-history` | warn | Warns when a changed file repeatedly co-changes with several partners, indicating one concept may be spread across files. |

## Maintainability History

5 rules. Section reference: [checks.md](checks.md#quality).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-maintainability-change-amplification"></a>`maintainability.change-amplification` | warn | Warns when a changed file historically fans out into many co-changed partners. |
| <a id="rule-maintainability-high-churn-hotspot"></a>`maintainability.high-churn-hotspot` | warn | Warns when a changed file combines repeated churn with current complexity hints, making safe review and future changes harder. |
| <a id="rule-maintainability-hotspot"></a>`maintainability.hotspot` | warn | Warns when a changed file has high recent churn, defect history, or both. |
| <a id="rule-maintainability-repeat-defect-area"></a>`maintainability.repeat-defect-area` | warn | Warns when a changed file has multiple recent fix, regression, incident, or defect-linked commits in git history. |
| <a id="rule-maintainability-unstable-interface"></a>`maintainability.unstable-interface` | warn | Warns when a changed public-surface file has repeated churn or defect history, suggesting compatibility risk. |

## Maintainability Delta

2 rules. Section reference: [checks.md](checks.md#quality).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-maintainability-dependency-growth"></a>`maintainability.dependency-growth` | warn | Warns in diff scans when a changed file imports or includes more direct dependencies than it did at the base ref. |
| <a id="rule-maintainability-public-surface-growth"></a>`maintainability.public-surface-growth` | warn | Warns in diff scans when a changed file exports more public symbols than it did at the base ref. |

## Performance

46 rules. Section reference: [checks.md](checks.md#performance).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-performance-ai-semantic-perf"></a>`performance.ai.semantic-perf` | warn | Warns when optional LLM-assisted semantic review finds a performance concern in changed functions that static rules cannot judge: repeated expensive calls that want caching or memoization, algorithmic complexity out of line with plausible input sizes, or obviously redundant work across the change. |
| <a id="rule-performance-ai-semantic-runtime"></a>`performance.ai.semantic-runtime` | fail | Fails when the AI-assisted performance lens was enabled but the configured semantic command was missing, crashed, or returned invalid output, so the performance section would otherwise lose semantic coverage silently. |
| <a id="rule-performance-benchmark-regression"></a>`performance.benchmark-regression` | warn | Runs go test -run=^$ -bench=. -benchmem over the configured packages and warns when a benchmark's ns/op regresses beyond performance_rules.benchmarks.max_regression_percent relative to the stored baseline. The first run writes the baseline and reports nothing. Off by default because it executes the repository's own test code (performance_rules.benchmarks.enabled). |
| <a id="rule-performance-budget"></a>`performance.budget` | warn | Compares measured artifacts against the budgets configured in performance_rules.budgets: on-disk file or glob-total sizes (kind file-size), bundler stats totals or per-asset sizes from an esbuild metafile or webpack stats JSON (kind bundle-stats), Clang -ftime-trace durations for whole traces or named events (kind clang-time-trace), and Cargo --timings HTML reports for whole-build or per-crate Rust compile time (kind cargo-timings). A missing artifact reports as a warn finding, never a hard error; a budget entry may set level fail to gate the scan. |
| <a id="rule-performance-build-regression"></a>`performance.build-regression` | warn | Runs the configured performance_rules.build_regression.commands, measures each command's wall-clock duration, and warns when a command regresses beyond performance_rules.build_regression.max_regression_percent relative to the stored baseline. The first run writes the baseline and reports nothing. Off by default because it executes repository-configured commands (and therefore requires trusting the repository config). |
| <a id="rule-performance-complexity-regression"></a>`performance.complexity-regression` | warn | Warns in diff scans when a changed function's maximum loop-nesting depth increased relative to the diff base ref (performance_rules.detect_complexity_regression, on by default). Functions that do not exist at the base ref are skipped, and full scans are unaffected: the rule only activates in diff mode. |
| <a id="rule-performance-cpp-alloc-in-loop"></a>`performance.cpp.alloc-in-loop` | warn | Warns when a C++ loop repeatedly grows a std::string via +=, append, or push_back without visible reserve() on that accumulator (performance_rules.detect_alloc_in_loop). |
| <a id="rule-performance-cpp-flush-in-loop"></a>`performance.cpp.flush-in-loop` | warn | Warns when std::endl or std::flush is written inside a C++ loop, defeating stream buffering and often dominating hot-path latency (performance_rules.detect_hot_path_patterns). Ordinary newline writes such as a plain newline character are not flagged. |
| <a id="rule-performance-cpp-hot-header"></a>`performance.cpp.hot-header` | warn | Warns when a target-local C++ file has too many direct include/module importers, making edits fan out recompilation broadly (performance_rules.detect_rebuild_cascade, hot_package_importer_threshold). |
| <a id="rule-performance-cpp-range-for-copy"></a>`performance.cpp.range-for-copy` | warn | Warns when a C++ range-for binds each element by value, copying it every iteration (performance_rules.detect_hot_path_patterns). Covers structured bindings by value and explicit by-value iteration over heavy standard-library types such as std::string, std::vector, and map/set pair types; plain auto value : items is intentionally out of scope. |
| <a id="rule-performance-cpp-rebuild-amplifier"></a>`performance.cpp.rebuild-amplifier` | warn | Warns when a target-local C++ file has too many transitive include/module dependents, amplifying rebuild cascades (performance_rules.detect_rebuild_cascade, rebuild_amplifier_threshold). |
| <a id="rule-performance-cpp-sleep-in-loop"></a>`performance.cpp.sleep-in-loop` | warn | Warns when std::this_thread::sleep_for or sleep_until runs inside a C++ loop body, which usually marks a poll that wants a condition variable, timer primitive, or bounded backoff helper (performance_rules.detect_sleep_in_loop). |
| <a id="rule-performance-cpp-unbounded-concurrency"></a>`performance.cpp.unbounded-concurrency` | warn | Warns when threads or async tasks are accumulated or detached inside a loop without a visible concurrency bound (performance_rules.detect_unbounded_concurrency). |
| <a id="rule-performance-go-alloc-in-loop"></a>`performance.go.alloc-in-loop` | warn | Warns when a loop grows a string by concatenation or accumulates fmt.Sprintf output (performance_rules.detect_alloc_in_loop, on by default). When performance_rules.detect_prealloc_in_loop is enabled (off by default), also warns when a loop appends to a slice without preallocated capacity despite a knowable bound. |
| <a id="rule-performance-go-defer-in-loop"></a>`performance.go.defer-in-loop` | warn | Warns when a defer statement sits inside a loop body: deferred calls run only at function exit, so resources accumulate for every iteration (performance_rules.detect_defer_in_loop). |
| <a id="rule-performance-go-hot-package"></a>`performance.go.hot-package` | warn | Warns when a Go package has too many direct importers in the in-repo package graph, making routine edits fan out rebuilds broadly (performance_rules.detect_rebuild_cascade, hot_package_importer_threshold). |
| <a id="rule-performance-go-lock-held-across-blocking-call"></a>`performance.go.lock-held-across-blocking-call` | warn | Warns when a sync.Mutex or sync.RWMutex is held across an obvious blocking call - synchronous file I/O, time.Sleep, package-level HTTP calls, SQL-style Query/Exec methods, or client.Do - which serializes callers and amplifies tail latency (performance_rules.detect_hot_path_patterns). The check is lexical rather than flow-sensitive, so treat it as a review cue when a branch unlocks before the call. |
| <a id="rule-performance-go-nested-loop-scan"></a>`performance.go.nested-loop-scan` | warn | Warns when nested range loops compare an outer loop variable directly against an inner loop variable, which scans the inner collection linearly on every outer iteration (performance_rules.detect_hot_path_patterns). Intentionally narrow: it does not infer indexed lookups, helper wrappers, or non-equality membership checks. |
| <a id="rule-performance-go-rebuild-amplifier"></a>`performance.go.rebuild-amplifier` | warn | Warns when a Go package has too many transitive dependents in the in-repo package graph, so changes in that package amplify rebuild cascades across the target (performance_rules.detect_rebuild_cascade, rebuild_amplifier_threshold). |
| <a id="rule-performance-go-sleep-in-loop"></a>`performance.go.sleep-in-loop` | warn | Warns when time.Sleep runs inside a loop body, which usually marks a poll that burns latency (performance_rules.detect_sleep_in_loop). |
| <a id="rule-performance-go-slice-membership-in-loop"></a>`performance.go.slice-membership-in-loop` | warn | Warns when slices.Contains, ContainsFunc, Index, or IndexFunc is called inside a loop, so the slice is scanned linearly each iteration (performance_rules.detect_hot_path_patterns). It does not prove the slice is large enough to matter, so treat it as a hot-path review cue rather than a universal ban. |
| <a id="rule-performance-go-timer-leak-in-loop"></a>`performance.go.timer-leak-in-loop` | warn | Warns when time.After is called inside a loop: each call allocates a timer that is not garbage-collected until it fires, so tight loops leak timers (performance_rules.detect_timer_leaks). |
| <a id="rule-performance-javascript-await-in-loop"></a>`performance.javascript.await-in-loop` | warn | Warns when await runs inside a loop body, serializing work that could run concurrently; for await streams are exempt (performance_rules.detect_await_in_loop). |
| <a id="rule-performance-javascript-express-sync-middleware"></a>`performance.javascript.express-sync-middleware` | warn | Warns when a known CPU-heavy synchronous API (bcrypt.hashSync/compareSync, crypto.pbkdf2Sync/scryptSync, zlib *Sync, child_process.execSync) runs inside an app.use/router.use middleware region in a file with express evidence, blocking the event loop on every request. Takes precedence over performance.javascript.sync-io-in-handler on the same line (performance_rules.detect_framework_patterns). |
| <a id="rule-performance-javascript-react-expensive-render"></a>`performance.javascript.react-expensive-render` | warn | Warns when a React component body (file imports react; function named with a capital letter or use* prefix) chains two or more array methods (.sort/.filter/.map) or constructs via new Array/JSON.parse outside a useMemo/useCallback/useEffect wrapper, redoing the work on every render (performance_rules.detect_framework_patterns). |
| <a id="rule-performance-javascript-sync-io-in-handler"></a>`performance.javascript.sync-io-in-handler` | warn | Warns when a synchronous *Sync call runs inside an HTTP request handler, blocking the event loop. |
| <a id="rule-performance-javascript-timer-listener-leak"></a>`performance.javascript.timer-listener-leak` | warn | Warns when setInterval has no matching clearInterval anywhere in the file, or addEventListener runs inside a loop with no removeEventListener/AbortSignal in the file — both keep callbacks and their captures alive indefinitely (performance_rules.detect_timer_leaks). |
| <a id="rule-performance-javascript-unbounded-concurrency"></a>`performance.javascript.unbounded-concurrency` | warn | Warns when promises are created inside a loop without batching or a concurrency limiter. |
| <a id="rule-performance-n-plus-one-query"></a>`performance.n-plus-one-query` | warn | Warns when a database query or remote fetch call runs inside a loop body, suggesting an N+1 access pattern. |
| <a id="rule-performance-python-django-nplusone-relation"></a>`performance.python.django-nplusone-relation` | warn | Warns when a loop over a Django queryset accesses a related object (item.relation.attr or item.relation_set.*) in a file with Django evidence and no select_related/prefetch_related anywhere, issuing one extra query per row (performance_rules.detect_framework_patterns). |
| <a id="rule-performance-python-orm-query-in-loop"></a>`performance.python.orm-query-in-loop` | warn | Warns when a Django .objects.get/.objects.filter call or a SQLAlchemy session.get call runs inside a loop body, issuing one query per iteration. Covers only the ORM call shapes the generic performance.n-plus-one-query pattern misses, so a line never reports under both rules (performance_rules.detect_framework_patterns). |
| <a id="rule-performance-python-sync-io-in-async"></a>`performance.python.sync-io-in-async` | warn | Warns when requests, urllib, or time.sleep calls run inside an async def body, blocking the event loop. |
| <a id="rule-performance-python-unbounded-concurrency"></a>`performance.python.unbounded-concurrency` | warn | Warns when asyncio.create_task or asyncio.ensure_future runs inside a loop with no visible concurrency limit (Semaphore, TaskGroup, or a limiter library) in the file (performance_rules.detect_unbounded_concurrency). |
| <a id="rule-performance-regex-compile-in-loop"></a>`performance.regex-compile-in-loop` | warn | Warns when a regular expression is compiled inside a loop body (regexp.Compile/MustCompile, re.compile, new RegExp, or std::regex construction), repeating the compilation cost every iteration (performance_rules.detect_regex_compile_in_loop). |
| <a id="rule-performance-rust-alloc-in-loop"></a>`performance.rust.alloc-in-loop` | warn | Warns when a Rust loop repeatedly grows a String initialized without visible preallocation via +=, x = x + ..., or push_str (performance_rules.detect_alloc_in_loop). |
| <a id="rule-performance-rust-sleep-in-loop"></a>`performance.rust.sleep-in-loop` | warn | Warns when std::thread::sleep or thread::sleep runs inside a Rust loop body, which usually marks a poll that wants a signal, timer primitive, or bounded backoff helper (performance_rules.detect_sleep_in_loop). |
| <a id="rule-performance-string-concat-in-loop"></a>`performance.string-concat-in-loop` | warn | Warns when a string grows by += inside a loop in Python or TypeScript/JavaScript, copying the accumulated value every iteration (performance_rules.detect_alloc_in_loop; the Go equivalent is performance.go.alloc-in-loop). |
| <a id="rule-performance-sync-io-in-request-path"></a>`performance.sync-io-in-request-path` | warn | Warns when a likely Go HTTP handler performs synchronous file I/O directly in the request path. |
| <a id="rule-performance-typescript-await-in-loop"></a>`performance.typescript.await-in-loop` | warn | Warns when await runs inside a loop body, serializing work that could run concurrently; for await streams are exempt (performance_rules.detect_await_in_loop). |
| <a id="rule-performance-typescript-express-sync-middleware"></a>`performance.typescript.express-sync-middleware` | warn | Warns when a known CPU-heavy synchronous API (bcrypt.hashSync/compareSync, crypto.pbkdf2Sync/scryptSync, zlib *Sync, child_process.execSync) runs inside an app.use/router.use middleware region in a file with express evidence, blocking the event loop on every request. Takes precedence over performance.typescript.sync-io-in-handler on the same line (performance_rules.detect_framework_patterns). |
| <a id="rule-performance-typescript-react-expensive-render"></a>`performance.typescript.react-expensive-render` | warn | Warns when a React component body (file imports react; function named with a capital letter or use* prefix) chains two or more array methods (.sort/.filter/.map) or constructs via new Array/JSON.parse outside a useMemo/useCallback/useEffect wrapper, redoing the work on every render (performance_rules.detect_framework_patterns). |
| <a id="rule-performance-typescript-sync-io-in-handler"></a>`performance.typescript.sync-io-in-handler` | warn | Warns when a synchronous *Sync call runs inside an HTTP request handler, blocking the event loop. |
| <a id="rule-performance-typescript-timer-listener-leak"></a>`performance.typescript.timer-listener-leak` | warn | Warns when setInterval has no matching clearInterval anywhere in the file, or addEventListener runs inside a loop with no removeEventListener/AbortSignal in the file — both keep callbacks and their captures alive indefinitely (performance_rules.detect_timer_leaks). |
| <a id="rule-performance-typescript-unbounded-concurrency"></a>`performance.typescript.unbounded-concurrency` | warn | Warns when promises are created inside a loop without batching or a concurrency limiter. |
| <a id="rule-performance-unbounded-goroutines-in-loop"></a>`performance.unbounded-goroutines-in-loop` | warn | Warns when Go code launches goroutines from inside loops without any visible bounding mechanism. |
| <a id="rule-performance-unbounded-read"></a>`performance.unbounded-read` | warn | Warns when an entire input is read into memory without a size bound: io.ReadAll in Go HTTP handlers or loops, and .read()/.readlines() with no size argument in Python loops. One oversized input then costs the whole payload in memory (performance_rules.detect_unbounded_reads). |

## Reliability

13 rules. Section reference: [checks.md](checks.md#reliability).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-reliability-lost-error-context"></a>`reliability.lost-error-context` | warn | Warns when an error is replaced without operation context or wrapping. |
| <a id="rule-reliability-missing-cancellation"></a>`reliability.missing-cancellation` | warn | Warns when request or job code drops caller cancellation by using a background context for downstream work. |
| <a id="rule-reliability-missing-concurrency-limit"></a>`reliability.missing-concurrency-limit` | warn | Warns when concurrent work is launched without an obvious limit. |
| <a id="rule-reliability-missing-graceful-shutdown"></a>`reliability.missing-graceful-shutdown` | warn | Warns when servers or long-running workers start without shutdown/drain handling. |
| <a id="rule-reliability-missing-timeout"></a>`reliability.missing-timeout` | fail | Fails when outbound production calls are made without a bounded timeout or deadline. |
| <a id="rule-reliability-non-idempotent-retry"></a>`reliability.non-idempotent-retry` | fail | Fails when retry logic wraps a non-idempotent operation without idempotency evidence. |
| <a id="rule-reliability-partial-failure-hidden"></a>`reliability.partial-failure-hidden` | fail | Fails when batch or multi-step work can fail partially but still report success. |
| <a id="rule-reliability-recoverable-panic"></a>`reliability.recoverable-panic` | fail | Fails when a recoverable runtime condition is handled with panic in production code. |
| <a id="rule-reliability-resource-leak"></a>`reliability.resource-leak` | fail | Fails when opened resources such as response bodies, rows, files, tickers, or timers are not closed or stopped. |
| <a id="rule-reliability-retry-without-backoff"></a>`reliability.retry-without-backoff` | warn | Warns when retry logic repeats immediately without backoff or jitter. |
| <a id="rule-reliability-swallowed-error"></a>`reliability.swallowed-error` | fail | Fails when an error is discarded or ignored in production code. |
| <a id="rule-reliability-unbounded-retry"></a>`reliability.unbounded-retry` | fail | Fails when retry logic can continue without a clear attempt limit or cancellation boundary. |
| <a id="rule-reliability-unbounded-work"></a>`reliability.unbounded-work` | warn | Warns when goroutines, workers, queues, or buffers can grow without an explicit bound. |

## Data Correctness

11 rules. Section reference: [checks.md](checks.md#data-correctness).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-data-cache-without-policy"></a>`data.cache-without-policy` | warn | Warns when production caches lack TTL, invalidation, or ownership policy evidence. |
| <a id="rule-data-exactly-once-assumption"></a>`data.exactly-once-assumption` | warn | Warns when code assumes exactly-once message delivery without idempotency or deduplication evidence. |
| <a id="rule-data-missing-deduplication"></a>`data.missing-deduplication` | warn | Warns when message or event handling lacks deduplication evidence. |
| <a id="rule-data-missing-outbox-strategy"></a>`data.missing-outbox-strategy` | fail | Fails when code persists state and publishes an event without an outbox or equivalent strategy. |
| <a id="rule-data-missing-transaction-boundary"></a>`data.missing-transaction-boundary` | fail | Fails when related persistence writes can commit independently without a transaction boundary. |
| <a id="rule-data-non-idempotent-consumer"></a>`data.non-idempotent-consumer` | fail | Fails when a message/event consumer performs side effects without idempotency evidence. |
| <a id="rule-data-read-modify-write-race"></a>`data.read-modify-write-race` | fail | Fails when code reads state and writes derived state without a transaction, lock, or compare-and-swap boundary. |
| <a id="rule-data-side-effect-in-transaction"></a>`data.side-effect-in-transaction` | fail | Fails when external side effects are performed inside retried or rollback-capable transactions. |
| <a id="rule-data-unbounded-read"></a>`data.unbounded-read` | warn | Warns when database reads lack a limit, cursor, stream, or bounded filter. |
| <a id="rule-data-unsafe-dual-write"></a>`data.unsafe-dual-write` | fail | Fails when code writes to two systems without a strategy for partial failure. |
| <a id="rule-data-unstable-pagination"></a>`data.unstable-pagination` | warn | Warns when paginated reads lack deterministic ordering or cursor stability. |

## Observability

7 rules. Section reference: [checks.md](checks.md#observability).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-observability-critical-path-uninstrumented"></a>`observability.critical-path-uninstrumented` | warn | Warns when handlers, consumers, jobs, or other critical paths lack visible logging, metrics, or tracing evidence. |
| <a id="rule-observability-error-without-context"></a>`observability.error-without-context` | warn | Warns when an error is logged without operation, request, or safe business context. |
| <a id="rule-observability-high-cardinality-label"></a>`observability.high-cardinality-label` | warn | Warns when metric labels use identifiers or raw paths that can explode cardinality. |
| <a id="rule-observability-log-and-ignore"></a>`observability.log-and-ignore` | warn | Warns when code logs a failure and then continues or returns success without surfacing the error. |
| <a id="rule-observability-sensitive-log-data"></a>`observability.sensitive-log-data` | fail | Fails when log calls include sensitive names such as tokens, passwords, authorization headers, cookies, or PII-like fields. |
| <a id="rule-observability-shallow-health-check"></a>`observability.shallow-health-check` | warn | Warns when a health/readiness endpoint returns a static OK response while dependency evidence exists nearby. |
| <a id="rule-observability-unstructured-log"></a>`observability.unstructured-log` | warn | Warns when production code writes raw logs without structured context fields. |

## Operations

2 rules. Section reference: [checks.md](checks.md#operations).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-operations-missing-owner"></a>`operations.missing-owner` | warn | Warns when critical production code has no CODEOWNERS, service catalog, or configured ownership metadata. |
| <a id="rule-operations-missing-runbook"></a>`operations.missing-runbook` | warn | Warns when critical systems lack runbook or operations documentation evidence. |

## Delivery

4 rules. Section reference: [checks.md](checks.md#delivery).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-delivery-high-risk-change-without-kill-switch"></a>`delivery.high-risk-change-without-kill-switch` | warn | Warns when critical payment, auth, checkout, billing, or migration behavior changes without feature flag or kill-switch evidence. |
| <a id="rule-delivery-missing-post-deploy-verification"></a>`delivery.missing-post-deploy-verification` | warn | Warns when a deployment workflow has no smoke, health, synthetic, SLO, or equivalent verification evidence after rollout. |
| <a id="rule-delivery-missing-rollback-strategy"></a>`delivery.missing-rollback-strategy` | warn | Warns when deployment or destructive migration evidence appears without rollback, revert, or restore instructions. |
| <a id="rule-delivery-unsafe-migration-order"></a>`delivery.unsafe-migration-order` | warn | Warns when a destructive migration lacks expand/backfill/contract, concurrent, or rollback sequencing evidence. |

## Change Safety

6 rules. Section reference: [checks.md](checks.md#change-safety).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-change-mixed-concerns"></a>`change.mixed-concerns` | warn | Warns when a PR combines unrelated subsystems, architectural layers, or rule families in one review unit. |
| <a id="rule-change-mixed-refactor-and-behavior"></a>`change.mixed-refactor-and-behavior` | warn | Warns when a diff combines moves, renames, or extraction with observable behavior changes. |
| <a id="rule-change-move-without-verification"></a>`change.move-without-verification` | warn | Warns when files or symbols move without test, build, or behavior-preservation evidence. |
| <a id="rule-change-oversized-diff"></a>`change.oversized-diff` | warn | Warns when the changed-file, changed-directory, changed-line, public-interface, or test-ratio budget makes the PR hard to review safely. |
| <a id="rule-change-too-many-concerns"></a>`change.too-many-concerns` | warn | Warns when change concentration evidence shows too many unrelated concepts being modified at once. |
| <a id="rule-change-unnecessary-surface-area"></a>`change.unnecessary-surface-area` | warn | Warns when a narrow change touches more files, directories, or public interfaces than the behavior requires. |

## Change Safety / Refactors

9 rules. Section reference: [checks.md](checks.md#change-safety).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-refactor-behavior-change-detected"></a>`refactor.behavior-change-detected` | fail | Fails when a refactor-labeled diff changes return paths, side effects, authorization checks, writes, emitted events, or external calls. |
| <a id="rule-refactor-dead-path-left-behind"></a>`refactor.dead-path-left-behind` | warn | Warns when refactoring leaves obsolete branches, feature flags, wrappers, or compatibility paths without consumers. |
| <a id="rule-refactor-dependency-direction-worsened"></a>`refactor.dependency-direction-worsened` | warn | Warns when refactoring introduces an inward dependency on infrastructure, UI, persistence, or framework code. |
| <a id="rule-refactor-duplicate-implementation-left-behind"></a>`refactor.duplicate-implementation-left-behind` | warn | Warns when an extraction or move leaves the previous implementation active in another path. |
| <a id="rule-refactor-error-path-changed"></a>`refactor.error-path-changed` | fail | Fails when a refactor changes wrapping, returned errors, ignored errors, panic behavior, or partial-failure handling. |
| <a id="rule-refactor-public-contract-changed"></a>`refactor.public-contract-changed` | fail | Fails when exported signatures, API schemas, events, or persistence contracts change in a refactor-only PR. |
| <a id="rule-refactor-side-effect-order-changed"></a>`refactor.side-effect-order-changed` | fail | Fails when database writes, event publishing, network calls, cleanup, or authorization side effects are reordered. |
| <a id="rule-refactor-test-coverage-reduced"></a>`refactor.test-coverage-reduced` | warn | Warns when a refactor removes or weakens tests over the moved or reshaped behavior. |
| <a id="rule-refactor-visibility-expanded"></a>`refactor.visibility-expanded` | warn | Warns when private symbols become public or cross-package visible during a refactor. |

## Change Safety / Testability

9 rules. Section reference: [checks.md](checks.md#change-safety).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-change-cleanup-regression"></a>`change.cleanup-regression` | warn | Warns when a cleanup-labeled change increases complexity, duplication, public surface, or dependency count. |
| <a id="rule-change-complexity-increased"></a>`change.complexity-increased` | warn | Warns when touched functions, files, or hotspots become materially more complex in the diff. |
| <a id="rule-change-duplicate-helper"></a>`change.duplicate-helper` | warn | Warns when a change introduces helper logic that overlaps existing project vocabulary or utilities. |
| <a id="rule-change-one-use-abstraction"></a>`change.one-use-abstraction` | warn | Warns when a new abstraction is introduced but has only one consumer or delegates without simplifying the caller. |
| <a id="rule-testing-behavior-change-without-test"></a>`testing.behavior-change-without-test` | fail | Fails when production behavior changes without nearby test evidence in the same diff. |
| <a id="rule-testing-failure-path-missing"></a>`testing.failure-path-missing` | warn | Warns when high-risk branches add error, retry, fallback, authorization, or external dependency paths without failure-path tests. |
| <a id="rule-testing-hardwired-dependency"></a>`testing.hardwired-dependency` | warn | Warns when business logic constructs clocks, random sources, network clients, filesystem access, or infrastructure dependencies directly. |
| <a id="rule-testing-legacy-hotspot-uncovered"></a>`testing.legacy-hotspot-uncovered` | warn | Warns when a high-churn or complex legacy hotspot is touched without characterization or regression-test evidence. |
| <a id="rule-testing-nondeterministic-domain-logic"></a>`testing.nondeterministic-domain-logic` | warn | Warns when domain code reads time, randomness, filesystem, network, or environment state directly. |

## Supply Chain

8 rules. Section reference: [checks.md](checks.md#supply-chain).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-supply_chain-cargo-missing-package-license"></a>`supply_chain.cargo.missing-package-license` | warn | Warns when a Cargo.toml package omits the package.license field, making license policy and artifact metadata harder to audit. |
| <a id="rule-supply_chain-cargo-non-hermetic-source"></a>`supply_chain.cargo.non-hermetic-source` | warn | Warns when a Cargo dependency uses a local path source, a floating git branch, or a git source without a pinned rev. |
| <a id="rule-supply_chain-denied-license"></a>`supply_chain.denied-license` | fail | Fails when a dependency resolves to a license that violates configured policy. |
| <a id="rule-supply_chain-lockfile-drift"></a>`supply_chain.lockfile-drift` | fail | Fails when a manifest change is not reflected in the associated lockfile. |
| <a id="rule-supply_chain-missing-lockfile"></a>`supply_chain.missing-lockfile` | fail | Fails when a supported package manifest is committed without its expected lockfile. |
| <a id="rule-supply_chain-missing-provenance"></a>`supply_chain.missing-provenance` | fail | Fails when artifact build or publish automation exists without provenance, attestation, SBOM, or signing evidence. |
| <a id="rule-supply_chain-unpinned-dependency"></a>`supply_chain.unpinned-dependency` | warn | Warns when a dependency declaration does not pin to a concrete version or digest. |
| <a id="rule-supply_chain-vulnerable-dependency"></a>`supply_chain.vulnerable-dependency` | fail | Fails when a normalized dependency matches an affected version range in the configured local advisory cache. |

## API Contracts

6 rules. Section reference: [checks.md](checks.md#api-contracts).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-contracts-cpp-public-breaking"></a>`contracts.cpp-public-breaking` | fail | Fails in diff mode when a declaration in an include, public, or api header is removed, renamed, or changes signature against the base ref. |
| <a id="rule-contracts-go-exported-breaking"></a>`contracts.go-exported-breaking` | fail | Fails in diff mode when exported Go functions, methods, types, or consts are removed or renamed, or when an exported function signature changes against the base ref. |
| <a id="rule-contracts-migration-destructive"></a>`contracts.migration-destructive` | warn | Warns when migration files contain destructive operations such as DROP TABLE, DROP COLUMN, TRUNCATE, or ALTER ... NOT NULL without a DEFAULT. |
| <a id="rule-contracts-non-expand-contract-migration"></a>`contracts.non-expand-contract-migration` | fail | Fails when a rolling database schema migration contracts a schema before compatible code and backfill steps are in place. |
| <a id="rule-contracts-openapi-breaking"></a>`contracts.openapi-breaking` | fail | Fails in diff mode when an OpenAPI document removes paths, operations, or response codes, or makes request parameters or body fields newly required against the base ref. |
| <a id="rule-contracts-proto-breaking"></a>`contracts.proto-breaking` | fail | Fails in diff mode when a .proto file removes messages, services, or rpcs, or removes, renumbers, or retypes message fields against the base ref. |

## Design Patterns

50 rules. Section reference: [checks.md](checks.md#design).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-design-capability-boundary"></a>`design.capability-boundary` | fail | Fails when a source module imports a sensitive technical capability outside its approved paths. |
| <a id="rule-design-cmd-through-internal-cli"></a>`design.cmd-through-internal-cli` | fail | Fails when cmd entrypoints import reusable service packages directly instead of going through internal/cli. |
| <a id="rule-design-command-check"></a>`design.command-check` | fail | Fails when a configured language-specific design command exits non-zero. |
| <a id="rule-design-configuration-leak"></a>`design.configuration-leak` | warn | Warns when config, feature-flag, or environment concerns leak into domain code. |
| <a id="rule-design-cpp-generic-module-name"></a>`design.cpp.generic-module-name` | warn | Warns when a C++ source, header, or module file name is too generic to communicate responsibility. |
| <a id="rule-design-cpp-import-cycle"></a>`design.cpp.import-cycle` | fail | Fails when target-local C++ headers, source files, or named modules form a dependency cycle. |
| <a id="rule-design-cpp-max-decls-per-file"></a>`design.cpp.max-decls-per-file` | warn | Warns when a C++ file accumulates too many top-level declarations. |
| <a id="rule-design-cpp-max-interface-methods"></a>`design.cpp.max-interface-methods` | warn | Warns when a C++ contract file exposes too many public methods on one type. |
| <a id="rule-design-cpp-max-methods-per-type"></a>`design.cpp.max-methods-per-type` | warn | Warns when one C++ file defines too many qualified methods for the same type. |
| <a id="rule-design-data-ownership"></a>`design.data-ownership` | fail | Fails when one domain directly imports another domain's data or persistence modules. |
| <a id="rule-design-diff-command-check"></a>`design.diff-command-check` | fail | Fails in diff mode when a configured language-specific contract check reports breaking drift between the base ref and the current target. |
| <a id="rule-design-domain-boundary"></a>`design.domain-boundary` | fail | Fails when a cross-domain import is not permitted or bypasses the target domain's public paths. |
| <a id="rule-design-domain-logic-in-handler"></a>`design.domain-logic-in-handler` | warn | Warns when a handler/controller owns business-rule branching instead of orchestrating a domain service. |
| <a id="rule-design-excessive-public-surface"></a>`design.excessive-public-surface` | warn | Warns when one file exposes more public API than the configured local-design threshold. |
| <a id="rule-design-generic-package-name"></a>`design.generic-package-name` | warn | Warns when a package name is too generic to communicate ownership or responsibility. |
| <a id="rule-design-god-module"></a>`design.god-module` | warn | Warns when a module's combined fan-in and fan-out exceeds the configured threshold, indicating it concentrates too much of the dependency graph. |
| <a id="rule-design-high-impact-change"></a>`design.high-impact-change` | warn | Warns in diff mode when a changed module has more transitive dependents than the configured threshold. |
| <a id="rule-design-infrastructure-type-leak"></a>`design.infrastructure-type-leak` | warn | Warns when framework, database, cloud, or transport types cross into domain/public APIs. |
| <a id="rule-design-java-import-cycle"></a>`design.java.import-cycle` | fail | Fails when Java classes form an internal import cycle. |
| <a id="rule-design-javascript-import-cycle"></a>`design.javascript.import-cycle` | fail | Fails when JavaScript modules form an internal import cycle. |
| <a id="rule-design-layer-boundary"></a>`design.layer-boundary` | fail | Fails when a module imports a denied architectural layer or an external dependency denied to its layer. |
| <a id="rule-design-max-decls-per-file"></a>`design.max-decls-per-file` | warn | Warns when a file accumulates too many top-level declarations. |
| <a id="rule-design-max-interface-methods"></a>`design.max-interface-methods` | warn | Warns when an interface is too large. |
| <a id="rule-design-max-methods-per-type"></a>`design.max-methods-per-type` | warn | Warns when a type accumulates too many methods. |
| <a id="rule-design-pass-through-abstraction"></a>`design.pass-through-abstraction` | warn | Warns when a wrapper delegates directly without policy, validation, translation, or isolation. |
| <a id="rule-design-persistence-model-leak"></a>`design.persistence-model-leak` | warn | Warns when ORM/database records appear in public API or handler contracts. |
| <a id="rule-design-private-module-import"></a>`design.private-module-import` | fail | Fails when code outside a configured public surface imports one of its implementation modules instead of an approved entrypoint. |
| <a id="rule-design-production-imports-test"></a>`design.production-imports-test` | fail | Fails when a configured production module imports a test, mock, fixture, or other test-only module. |
| <a id="rule-design-python-generic-module-name"></a>`design.python.generic-module-name` | warn | Warns when a Python module name is too generic to communicate ownership or responsibility. |
| <a id="rule-design-python-import-cycle"></a>`design.python.import-cycle` | fail | Fails when Python modules form an internal import cycle. |
| <a id="rule-design-python-max-methods-per-type"></a>`design.python.max-methods-per-type` | warn | Warns when a Python class accumulates too many direct methods. |
| <a id="rule-design-python-max-protocol-members"></a>`design.python.max-protocol-members` | warn | Warns when a Python Protocol accumulates too many direct members. |
| <a id="rule-design-python-public-depends-on-cli"></a>`design.python.public-depends-on-cli` | fail | Fails when a public Python module reaches a CLI or entrypoint module through the project import graph. |
| <a id="rule-design-python-public-imports-cli"></a>`design.python.public-imports-cli` | fail | Fails when a public Python module imports a CLI or entrypoint module. |
| <a id="rule-design-python-public-imports-private"></a>`design.python.public-imports-private` | fail | Fails when a public Python module imports a private module or private symbol. |
| <a id="rule-design-rust-generic-module-name"></a>`design.rust.generic-module-name` | warn | Warns when a Rust module name is too generic to communicate ownership or responsibility. |
| <a id="rule-design-rust-import-cycle"></a>`design.rust.import-cycle` | fail | Fails when Rust modules form an internal use/mod dependency cycle. |
| <a id="rule-design-rust-max-methods-per-type"></a>`design.rust.max-methods-per-type` | warn | Warns when a Rust type accumulates too many impl methods within one file. |
| <a id="rule-design-rust-max-trait-members"></a>`design.rust.max-trait-members` | warn | Warns when a Rust trait accumulates too many members. |
| <a id="rule-design-service-import-cmd"></a>`design.service-import-cmd` | fail | Fails when reusable service code imports cmd packages. |
| <a id="rule-design-service-import-internal"></a>`design.service-import-internal` | fail | Fails when reusable public packages, other than the SDK facade, import internal packages. |
| <a id="rule-design-shallow-module"></a>`design.shallow-module` | warn | Warns when a module exposes many public symbols but provides little behavior or abstraction depth. |
| <a id="rule-design-stability-direction"></a>`design.stability-direction` | warn | Warns when a widely depended-on stable module imports a substantially less stable module, reversing the desired dependency direction. |
| <a id="rule-design-temporal-coupling"></a>`design.temporal-coupling` | warn | Warns when a function relies on an implicit setup-before-action call order. |
| <a id="rule-design-typescript-generic-module-name"></a>`design.typescript.generic-module-name` | warn | Warns when a TypeScript or JavaScript module name is too generic to communicate ownership or responsibility. |
| <a id="rule-design-typescript-import-cycle"></a>`design.typescript.import-cycle` | fail | Fails when TypeScript modules form an internal import cycle. |
| <a id="rule-design-typescript-max-interface-members"></a>`design.typescript.max-interface-members` | warn | Warns when a TypeScript interface or object type accumulates too many members. |
| <a id="rule-design-typescript-max-methods-per-type"></a>`design.typescript.max-methods-per-type` | warn | Warns when a TypeScript or JavaScript class accumulates too many methods. |
| <a id="rule-design-unassigned-module"></a>`design.unassigned-module` | fail | Fails when boundary assignment is required and a module is outside every configured layer or domain. |
| <a id="rule-design-unreachable-module"></a>`design.unreachable-module` | warn | Warns when a production module cannot be reached from any configured application or package entrypoint. |

## Security

57 rules. Section reference: [checks.md](checks.md#security).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-security-bind-all-interfaces"></a>`security.bind-all-interfaces` | warn | Warns when a service binds to 0.0.0.0, exposing it on every network interface. |
| <a id="rule-security-command-check"></a>`security.command-check` | fail | Fails when a configured language-specific security command exits non-zero. |
| <a id="rule-security-cors-wildcard"></a>`security.cors-wildcard` | warn | Warns when Access-Control-Allow-Origin is set to the wildcard '*', which lets any site read cross-origin responses. |
| <a id="rule-security-cpp-insecure-tls"></a>`security.cpp.insecure-tls` | fail | Fails when C++ code disables certificate or hostname verification in common libcurl, OpenSSL, Boost.Asio, or cpprestsdk APIs. |
| <a id="rule-security-cpp-shell-execution"></a>`security.cpp.shell-execution` | warn | Warns when C++ code invokes system, popen, or _popen. |
| <a id="rule-security-cpp-unsafe-c-api"></a>`security.cpp.unsafe-c-api` | warn | Warns when C++ code calls gets, strcpy, strcat, sprintf, or vsprintf, whose destination bounds are not part of the API. |
| <a id="rule-security-csharp-insecure-tls"></a>`security.csharp.insecure-tls` | fail | Fails when C# code disables TLS certificate validation. |
| <a id="rule-security-csharp-shell-execution"></a>`security.csharp.shell-execution` | warn | Warns when C# code invokes OS process execution primitives. |
| <a id="rule-security-debug-enabled"></a>`security.debug-enabled` | warn | Warns when a framework debug flag is enabled (e.g. debug=True), which can expose stack traces, consoles, or secrets in production. |
| <a id="rule-security-dockerfile-root"></a>`security.dockerfile-root` | warn | Warns when a Dockerfile explicitly sets USER root, so the container process runs with root privileges. |
| <a id="rule-security-govulncheck"></a>`security.govulncheck` | fail | Surfaces vulnerabilities or integration failures from govulncheck. |
| <a id="rule-security-hardcoded-credential"></a>`security.hardcoded-credential` | fail | Fails when a value matching a known credential format (AWS, GitHub, GitLab, Slack, Stripe, Google, npm, SendGrid, or a configured custom pattern) is found in source text. |
| <a id="rule-security-hardcoded-secret"></a>`security.hardcoded-secret` | warn | Warns on the lower-confidence name-based heuristic: a secret/token/api_key/password identifier assigned a quoted literal. |
| <a id="rule-security-high-entropy-string"></a>`security.high-entropy-string` | warn | Warns on a high-entropy string literal that may be an unknown or random secret. Opt-in via security_rules.secrets.entropy. |
| <a id="rule-security-insecure-deserialization"></a>`security.insecure-deserialization` | warn | Warns when untrusted data may be deserialized through a dangerous API (pickle, yaml.load, Java readObject, Marshal.load, unserialize), which can lead to remote code execution. |
| <a id="rule-security-insecure-tls"></a>`security.insecure-tls` | fail | Fails when insecure TLS verification is explicitly disabled. |
| <a id="rule-security-java-insecure-tls"></a>`security.java.insecure-tls` | fail | Fails when Java code disables hostname or certificate verification. |
| <a id="rule-security-java-shell-execution"></a>`security.java.shell-execution` | warn | Warns when Java code invokes OS process execution primitives. |
| <a id="rule-security-javascript-dynamic-code"></a>`security.javascript.dynamic-code` | warn | Warns when JavaScript code evaluates dynamic source text. |
| <a id="rule-security-javascript-insecure-tls"></a>`security.javascript.insecure-tls` | fail | Fails when JavaScript code disables TLS verification. |
| <a id="rule-security-javascript-postmessage-wildcard"></a>`security.javascript.postmessage-wildcard` | warn | Warns when JavaScript code sends postMessage calls to the wildcard origin. |
| <a id="rule-security-javascript-shell-execution"></a>`security.javascript.shell-execution` | warn | Warns when JavaScript code invokes shell execution primitives. |
| <a id="rule-security-javascript-string-timer-code"></a>`security.javascript.string-timer-code` | warn | Warns when JavaScript code uses string-based setTimeout or setInterval execution. |
| <a id="rule-security-javascript-taint-flow"></a>`security.javascript.taint-flow` | warn | Warns when untrusted input flows into a sensitive sink, including across module boundaries. |
| <a id="rule-security-javascript-unsafe-html-sink"></a>`security.javascript.unsafe-html-sink` | warn | Warns when JavaScript code writes untrusted content into unsafe HTML injection sinks. |
| <a id="rule-security-javascript-untrusted-input-flow"></a>`security.javascript.untrusted-input-flow` | warn | Warns when JavaScript semantic analysis finds untrusted input flowing into a risky sink. |
| <a id="rule-security-javascript-vm-dynamic-code"></a>`security.javascript.vm-dynamic-code` | warn | Warns when JavaScript code uses Node vm APIs to compile or execute dynamic code. |
| <a id="rule-security-log-secret-exposure"></a>`security.log-secret-exposure` | warn | Warns when a secret-bearing value appears inside the argument list of a logging call: Go log./logger./slog./zap./logrus. Print/Info/Error/Debug/Warn/Fatal/Panic variants; Python logging./logger./log. level methods and print; TS/JS console./logger./log. methods. Matching runs on masked source (comments and string contents blanked), so a secret word merely appearing in a comment or a plain format string never fires. On the call line it fires only when: (1) an argument identifier has a secret-named snake_case/camelCase component - password, passwd, secret, token, api_key/apikey, private_key, credential, authorization (whole components, so 'tokenizer' does not match; f-string and template-literal interpolations such as f"{token}" and `${token}` count); (2) a short whitespace-free string literal naming a secret is used as a structured-logging key (e.g. "password", value); (3) a string literal containing a secret keyword is concatenated with '+' to an expression (e.g. "Authorization: Bearer " + tok); or (4) a literal embeds '<keyword>=' or '<keyword>:' immediately followed by a string format directive (%s/%v/%q or '{'). "token count: %d" with a non-secret argument matches none of these. |
| <a id="rule-security-private-key"></a>`security.private-key` | fail | Fails when PEM private key material is detected. |
| <a id="rule-security-python-dynamic-code"></a>`security.python.dynamic-code` | warn | Warns when Python code evaluates or executes dynamic source text. |
| <a id="rule-security-python-insecure-tls"></a>`security.python.insecure-tls` | fail | Fails when Python code disables TLS verification. |
| <a id="rule-security-python-shell-execution"></a>`security.python.shell-execution` | warn | Warns when Python code invokes shell execution primitives. |
| <a id="rule-security-ruby-dynamic-code"></a>`security.ruby.dynamic-code` | warn | Warns when Ruby code evaluates dynamic source text. |
| <a id="rule-security-ruby-insecure-tls"></a>`security.ruby.insecure-tls` | fail | Fails when Ruby code disables TLS verification. |
| <a id="rule-security-ruby-shell-execution"></a>`security.ruby.shell-execution` | warn | Warns when Ruby code invokes shell or process execution primitives. |
| <a id="rule-security-rust-insecure-tls"></a>`security.rust.insecure-tls` | fail | Fails when Rust code disables TLS certificate or hostname verification. |
| <a id="rule-security-rust-shell-execution"></a>`security.rust.shell-execution` | warn | Warns when Rust code launches shell interpreters through process execution APIs. |
| <a id="rule-security-secrets-config"></a>`security.secrets-config` | fail | Fails when a security_rules.secrets allow_pattern or custom_pattern cannot be compiled and had to be skipped, so the scan is running with less coverage than configured. |
| <a id="rule-security-shell-execution"></a>`security.shell-execution` | warn | Warns when shell execution primitives are used and should receive manual review. |
| <a id="rule-security-ssrf-cpp"></a>`security.ssrf.cpp` | fail | Fails when untrusted input flows into an outbound URL or host in recognized libcurl, cpr, Boost.Asio, cpprestsdk, or Poco APIs. |
| <a id="rule-security-ssrf-go"></a>`security.ssrf.go` | fail | Fails when untrusted input flows into the URL of an outbound HTTP request (http.Get/Post/Head/PostForm/NewRequest), letting an attacker make the server reach arbitrary or internal hosts. The finding message includes the source-to-sink chain. |
| <a id="rule-security-ssrf-python"></a>`security.ssrf.python` | fail | Fails when untrusted input flows into the URL of an outbound HTTP request (requests.get/post/etc., urllib urlopen), letting an attacker make the server reach arbitrary or internal hosts. The finding message includes the source-to-sink chain. |
| <a id="rule-security-taint-cpp"></a>`security.taint.cpp` | fail | Fails when untrusted input from the environment, process arguments, standard input, or a recognized web request flows into a shell or process execution API. The finding message includes the source-to-sink chain. |
| <a id="rule-security-taint-go"></a>`security.taint.go` | fail | Fails when untrusted input (HTTP request data, environment, arguments, stdin) flows into a dangerous sink such as exec.Command, SQL query text, file paths, or template parsing. The finding message includes the source-to-sink chain. |
| <a id="rule-security-taint-python"></a>`security.taint.python` | fail | Fails when untrusted input (input(), os.environ, sys.argv, web request attributes) flows into a dangerous sink such as os.system, subprocess with a shell or string command, eval/exec, or SQL execute with interpolated query text. The finding message includes the source-to-sink chain. |
| <a id="rule-security-typescript-dynamic-code"></a>`security.typescript.dynamic-code` | warn | Warns when TypeScript or JavaScript code evaluates dynamic source text. |
| <a id="rule-security-typescript-insecure-tls"></a>`security.typescript.insecure-tls` | fail | Fails when TypeScript or JavaScript code disables TLS verification. |
| <a id="rule-security-typescript-postmessage-wildcard"></a>`security.typescript.postmessage-wildcard` | warn | Warns when TypeScript code sends postMessage calls to the wildcard origin. |
| <a id="rule-security-typescript-shell-execution"></a>`security.typescript.shell-execution` | warn | Warns when TypeScript or JavaScript code invokes shell execution primitives. |
| <a id="rule-security-typescript-string-timer-code"></a>`security.typescript.string-timer-code` | warn | Warns when TypeScript code uses string-based setTimeout or setInterval execution. |
| <a id="rule-security-typescript-taint-flow"></a>`security.typescript.taint-flow` | warn | Warns when untrusted input flows into a sensitive sink, including across module boundaries. |
| <a id="rule-security-typescript-unsafe-html-sink"></a>`security.typescript.unsafe-html-sink` | warn | Warns when TypeScript or JavaScript code writes untrusted content into unsafe HTML injection sinks. |
| <a id="rule-security-typescript-untrusted-input-flow"></a>`security.typescript.untrusted-input-flow` | warn | Warns when TypeScript semantic analysis finds untrusted input flowing into a risky sink. |
| <a id="rule-security-typescript-vm-dynamic-code"></a>`security.typescript.vm-dynamic-code` | warn | Warns when TypeScript or JavaScript code uses Node vm APIs to compile or execute dynamic code. |
| <a id="rule-security-unsanitized-error-response"></a>`security.unsanitized-error-response` | warn | Warns when a raw error value is written directly into an HTTP response, leaking internal details to clients and starving server-side logs of the diagnostic detail incident response needs. Conservative single-line patterns, matched on masked source: Go http.Error(w, err.Error(), ...) and fmt.Fprint/Fprintf/Fprintln with a response-writer-named first argument (w, wr, rw, res, resp, rsp, writer) and a raw err argument; TS/JS res/resp/response .send/.json/.end called with an error-named identifier (err, error, e, ex), including res.status(...).send(err.stack \|\| err.message) chains; Python return str(<exc>) or HttpResponse(str(<exc>)) on a line inside an except block, where <exc> is the block's 'as' alias. Errors passed to a logger but not written to the response do not fire. |
| <a id="rule-security-weak-cipher"></a>`security.weak-cipher` | warn | Warns when a weak cipher or mode is used (DES, 3DES, RC4, or ECB block mode). |
| <a id="rule-security-weak-hash"></a>`security.weak-hash` | warn | Warns when a broken hash algorithm (MD5 or SHA-1) is used, which is unsafe for signatures, integrity, or password storage. |

## AI Prompts

10 rules. Section reference: [checks.md](checks.md#prompts).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-prompts-agent-config-literal-secret"></a>`prompts.agent-config-literal-secret` | fail | Fails when an MCP server's env or headers, or a .claude/settings*.json env block, holds a literal credential instead of an environment-variable reference. |
| <a id="rule-prompts-agent-dangerous-instructions"></a>`prompts.agent-dangerous-instructions` | fail | Fails when agent instruction files (CLAUDE.md, AGENTS.md, GEMINI.md, .cursorrules, .windsurfrules, .github/copilot-instructions.md, .claude/agents, .claude/commands, .claude/skills/**/SKILL.md, .cursor/rules) instruct the agent to bypass approvals, permissions, or its sandbox - including bypass flags such as --dangerously-skip-permissions or --yolo and "never ask for confirmation" phrasing. A flag named only to prohibit it is not reported. |
| <a id="rule-prompts-agent-standing-permissions"></a>`prompts.agent-standing-permissions` | fail | Fails when agent configuration grants wildcard or effectively unrestricted standing tool permissions: .claude/settings*.json allow rules such as Bash(*), bare Bash, or an interpreter prefix like Bash(python3:*), defaultMode bypassPermissions, or wildcard allowlists in instruction files. Warns on enableAllProjectMcpServers. |
| <a id="rule-prompts-hidden-unicode"></a>`prompts.hidden-unicode` | fail | Fails when a governed prompt, agent instruction, or agent/MCP config file contains bidi control characters (U+202A-U+202E, U+2066-U+2069) or Unicode tag characters (U+E0000-U+E007F), and warns on zero-width characters - invisible text a model reads but a reviewer cannot see. |
| <a id="rule-prompts-mcp-config-risk"></a>`prompts.mcp-config-risk` | fail | Fails when MCP server configuration allows broad tool access (wildcard alwaysAllow/autoApprove, trust: true) or launches a server through a shell wrapper or a curl-pipe-to-shell installer. |
| <a id="rule-prompts-mcp-insecure-transport"></a>`prompts.mcp-insecure-transport` | fail | Fails when a remote MCP server url uses http:// or ws:// to a non-loopback host, exposing tool calls, results, and credentials in transit. |
| <a id="rule-prompts-mcp-remote-unauthenticated"></a>`prompts.mcp-remote-unauthenticated` | warn | Warns (low confidence) when a remote MCP server is declared with no Authorization, token, or API-key header. Servers that authenticate through OAuth discovery leave no trace in the config; disable with prompt_rules.require_mcp_remote_auth: false. |
| <a id="rule-prompts-mcp-unpinned-server"></a>`prompts.mcp-unpinned-server` | warn | Warns when an MCP server is launched from a registry (npx, bunx, pnpm dlx, npm exec, uvx, uv tool run, pipx run, docker/podman run) without an exact version, at a floating tag such as @latest, or as an untagged image, so a newly published release runs with the agent's privileges. |
| <a id="rule-prompts-secret-interpolation"></a>`prompts.secret-interpolation` | fail | Fails when prompt assets interpolate likely secret values. |
| <a id="rule-prompts-unsafe-instructions"></a>`prompts.unsafe-instructions` | warn | Warns when prompt assets contain instruction-injection or system prompt exfiltration patterns. |

## CI/CD

15 rules. Section reference: [checks.md](checks.md#ci).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-ci-always-true-test-assertion"></a>`ci.always-true-test-assertion` | warn | Warns when every assertion in a test only checks constants (for example EXPECT_TRUE(true), expect(true).toBe(true), or assert 1 == 1), so the test can never fail. |
| <a id="rule-ci-conditional-assertion"></a>`ci.conditional-assertion` | warn | Warns when every assertion in a test sits inside a conditional without an else branch, so the assertions may silently never run. Explicit failure calls such as Go t.Fatal and C++ ADD_FAILURE/FAIL are not flagged. |
| <a id="rule-ci-missing-required-gate"></a>`ci.missing-required-gate` | fail | Fails when configured required gates such as test or security checks are absent from CI workflows. |
| <a id="rule-ci-mutable-deployment-reference"></a>`ci.mutable-deployment-reference` | fail | Fails when deployment automation uses a GitHub Action ref other than a full commit SHA or a container image tagged latest. |
| <a id="rule-ci-required-file"></a>`ci.required-file` | fail | Fails when required workflow, release, or automation files are missing. |
| <a id="rule-ci-required-workflow-dir"></a>`ci.required-workflow-dir` | fail | Fails when the configured workflow directory is required but missing. |
| <a id="rule-ci-test-file-location"></a>`ci.test-file-location` | fail | Fails when language-specific test files live outside the configured test directories. |
| <a id="rule-ci-test-without-assertion"></a>`ci.test-without-assertion` | warn | Warns when a test function contains no recognizable assertion. Custom assertion helper names can be registered via ci_rules.test_quality.assertion_helpers. |
| <a id="rule-ci-workflow-content"></a>`ci.workflow-content` | fail | Fails when required workflow file markers are absent. |
| <a id="rule-ci-workflow-dangerous-trigger"></a>`ci.workflow-dangerous-trigger` | fail | Fails when a pull_request_target or workflow_run workflow checks out the pull request's own head (github.event.pull_request.head.*, github.head_ref, workflow_run head refs, refs/pull/*, or gh pr checkout), running untrusted code with the base repository's secrets and write token. |
| <a id="rule-ci-workflow-fork-trigger-secrets"></a>`ci.workflow-fork-trigger-secrets` | warn | Warns when a job in a pull_request_target workflow reads secrets.* (other than GITHUB_TOKEN) or passes secrets: inherit, because any fork can trigger the run. |
| <a id="rule-ci-workflow-missing-permissions"></a>`ci.workflow-missing-permissions` | warn | Warns when a workflow has no top-level permissions block and at least one job sets none, so GITHUB_TOKEN falls back to the repository or organization default (read-write on all scopes for older or permissive repositories). Workflows triggered only by workflow_call are skipped because the caller caps their token. |
| <a id="rule-ci-workflow-remote-script-pipe"></a>`ci.workflow-remote-script-pipe` | warn | Warns when a workflow or composite action step pipes curl or wget output straight into sh/bash (including bash <(curl ...) and sh -c "$(curl ...)"), executing whatever the remote host serves. |
| <a id="rule-ci-workflow-script-injection"></a>`ci.workflow-script-injection` | fail | Fails when an attacker-controlled ${{ }} expression (issue, pull request, comment or review text, commit messages, github.head_ref, or a composite action's inputs.*) is interpolated directly into a run: script or an actions/github-script script. |
| <a id="rule-ci-workflow-write-all-permissions"></a>`ci.workflow-write-all-permissions` | fail | Fails when a workflow or job sets permissions: write-all, giving GITHUB_TOKEN write access to every scope. |

## Agent Context

9 rules. Section reference: [checks.md](checks.md#agent-context).

| Rule | Default level | What it catches |
| --- | --- | --- |
| <a id="rule-context-agent-docs-drift"></a>`context.agent-docs-drift` | warn | Warns when an agent instruction file references a path, make target, or npm script that provably no longer exists, so agents inherit stale instructions. |
| <a id="rule-context-agent-docs-missing"></a>`context.agent-docs-missing` | warn | Warns when the repository root has no agent instruction file (CLAUDE.md, AGENTS.md, .cursorrules, or .github/copilot-instructions.md), leaving AI agents without documented conventions. |
| <a id="rule-context-ambiguous-symbol"></a>`context.ambiguous-symbol` | warn | Warns when the same source-file basename appears in many directories (context_rules.ambiguous_symbol_threshold, default 4), defeating filename search and grep-based navigation for AI agents. |
| <a id="rule-context-doc-link-rot"></a>`context.doc-link-rot` | warn | Warns when a markdown link in an agent instruction file or the root README points at a repository file or directory that does not exist. External URLs are never checked (no network I/O) and anchors are ignored. |
| <a id="rule-context-legibility-threshold"></a>`context.legibility-threshold` | warn | Fires when the repo_legibility score (0-100, higher is better) falls below a configured floor: warns below context_rules.legibility_warn_threshold and fails below context_rules.legibility_fail_threshold (0 disables each). The message carries the per-component breakdown so the weakest signal is visible. |
| <a id="rule-context-oversized-agent-doc"></a>`context.oversized-agent-doc` | warn | Warns when an agent instruction file exceeds context_rules.max_agent_doc_lines (default 600): agent docs are loaded into every session verbatim, so an oversized one consumes the context window it exists to save. |
| <a id="rule-context-oversized-context-unit"></a>`context.oversized-context-unit` | warn | Warns when a source file exceeds the agent context budget (context_rules.max_file_lines, default 1500): a file that large crowds out the rest of an AI agent's working context. |
| <a id="rule-context-readme-drift"></a>`context.readme-drift` | warn | Warns when a fenced shell block in the root README invokes a script, make target, or repo-relative path that provably does not exist. |
| <a id="rule-context-undocumented-commands"></a>`context.undocumented-commands` | warn | Warns when a high-signal Makefile target or package.json script (build, check, dev, fmt, lint, run, start, test) is mentioned by no agent instruction file and not even the root README, so agents cannot discover the repo's canonical entrypoints. Silent when the repo has no agent docs at all. |
