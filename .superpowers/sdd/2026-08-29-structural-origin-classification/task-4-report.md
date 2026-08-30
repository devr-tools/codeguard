# Task 4 Report — Named Regressions, Invariants, and crumb-app Acceptance

- status: IMPLEMENTATION_VERIFIED_EXTERNAL_FULL_CONFIG_GATE_DEFECT — implementation and repository verification pass; the supplied full configuration has an independently measured 193-finding non-structural floor, so its total cannot satisfy the requested maximum of 19
- commits: `04398d3` (`test: lock structural origin classification acceptance`), `202c57f` (`fix: close structural acceptance parser gaps`), `7c759d2` (`fix: refine structural effect grammar`), `cbaa5e9` (`fix: preserve Go conversion ownership`), plus this indexed/string conversion fix commit

## Coverage delivered

- Added reduced, source-shaped Go regressions for badge normalization, brand patching, place updates, `OvertureDatasetPath`, `NewStaticOvertureDivisionResolver`, and `duckDBDivisionHierarchy`.
- Added reduced C++ regressions for a local fluent builder and `DbRow::integer`.
- Asserted that every named local/value case emits no structural finding and that genuinely unresolved operations remain diagnostic-only (Go: 1; C++: 1 in the fixtures).
- Added proven Go and C++ receiver, argument/reference, global, and escaped mutations. Each invariant checks `function.hidden-mutation`, the exact mutation target, `effect_kind=shared_state`, and the expected `origin` (`caller_owned` or `shared`).
- Added baseline upgrade coverage proving that diagnostic prose, confidence, and evidence metadata do not change finding identity. A prior exact fingerprint plus current context/content identities remains active through real audit semantics, while a resolved false positive becomes the sole stale/prunable entry.
- Migrated three stale tests that expected guessed ownership for unknown React Native/Go collaborators. They now enforce the Task 1–3 contract: no structural finding without ownership proof and a language-specific unresolved diagnostic instead.
- Removed unused compatibility helpers and parameters and clarified two local names so the repository-pinned linter remains clean after the Task 2/3 integration.
- Added the requested changelog entry for declaration-based Go/C++ ownership, unresolved diagnostics, and wording-independent fingerprints.

## RED evidence

Before the new fixture files existed:

`GOCACHE=/private/tmp/codeguard-go-cache szr proxy go test ./tests/checks -run 'TestStructuralOrigin|TestBaselineAuditAndPruneKeep' -count=1 -v`

- Both structural-origin fixture tests failed: no expected unresolved diagnostic was present and none of the eight proven mutation cases could be found.
- The baseline upgrade invariant passed immediately because it locks the Task 1 identity/audit behavior already consumed by this task; it required no production change.

The first fixture run then exposed three useful expectation errors before becoming green:

- unresolved fixture counts were Go 3 and C++ 1, not the provisional values;
- a generic lookup selected a command/query finding before the required hidden-mutation finding;
- the C++ escaped-local fixture's first mutation was the global escape store, so the fixture was reduced to make the escaped local mutation deterministic.

## GREEN evidence

- `GOCACHE=/private/tmp/codeguard-go-cache szr go test ./tests/checks -run 'TestStructuralOrigin|TestBaselineAuditAndPruneKeep' -count=1`
  - PASS: 1 package.
- `GOCACHE=/private/tmp/codeguard-go-cache szr go test ./internal/codeguard/checks/quality ./internal/codeguard/checks/support ./tests/checks ./tests/support -count=1`
  - PASS: 4 packages.
- `GOCACHE=/private/tmp/codeguard-go-cache szr go test ./...`
  - PASS: 27 packages.
- `GOCACHE=/private/tmp/codeguard-go-cache szr go vet ./...`
  - PASS.
- `GOMODCACHE=/private/tmp/codeguard-gomodcache GOCACHE=/private/tmp/codeguard-lint-go-cache-final GOLANGCI_LINT_CACHE=/private/tmp/codeguard-lint-cache-final go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run`
  - PASS: `0 issues.`
- `szr git diff --check`
  - PASS.
- `GOCACHE=/private/tmp/codeguard-go-cache szr go build -o /private/tmp/codeguard-structural-origin ./cmd/codeguard`
  - PASS. Go emitted a non-fatal module stat-cache permission warning; the requested binary was produced and executed successfully.

## crumb-app acceptance

The command shape was confirmed read-only from `scan --help`, `baseline audit --help`, `baseline prune --help`, and the checked-in crumb-app configuration before execution. From `/Users/agentcarl/Documents/Github/crumb-app`:

`/private/tmp/codeguard-structural-origin scan -config .codeguard/codeguard.yaml -profile startup -mode full -format json -set cache.enabled=false -set checks.security_rules.govulncheck_mode=off`

`/private/tmp/codeguard-structural-origin baseline audit -config .codeguard/codeguard.yaml -profile startup -mode full -format json -set cache.enabled=false -set checks.security_rules.govulncheck_mode=off`

`/private/tmp/codeguard-structural-origin baseline prune -check -config .codeguard/codeguard.yaml -profile startup -mode full -format json -set cache.enabled=false -set checks.security_rules.govulncheck_mode=off`

The scan and audit exited 0. The prune check exited 1 because stale entries exist, as documented by its check-only semantics, and emitted the same count report without changing the baseline. crumb-app remained clean on `main` at `8ffc82b295b1bd6358a778d6dabacd1fdccc712b`.

Observed scan counts:

| Measure | Go | C++ | Total |
|---|---:|---:|---:|
| Unsuppressed findings | 130 | 203 (88 `.cpp`, 115 `.hpp`) | 333 |
| Unresolved mutation symbols | 1,965 | 3,367 | 5,332 |

- suppressed findings: 5,450
- scan sections: 5 pass, 2 warn, 0 fail
- baseline before: 6,560
- active: 5,316 (`active_context`: 5,316; exact/content: 0)
- stale: 1,244
- prune-check simulated removal: 1,244; simulated final: 5,316; invalid: 0
- genuine structural negatives in the CodeGuard fixtures: PASS (eight exact rule/metadata assertions)

## Concerns

- The required crumb-app gates are **not satisfied**: 333 unsuppressed findings exceeds the maximum of 19, and 1,244 stale entries differs from the required 887. These are the exact results from the requested branch binary, flags, clean external checkout, and mutation-free audit/prune-check workflow.
- Unresolved evidence exists for both analyzed language groups as required, but at substantial volume: Go 1,965 and C++ 3,367. Unsupported declarations/macros and intentionally conservative unknown ownership remain diagnostic-only rather than becoming guessed structural findings.
- The acceptance mismatch should be reconciled against the frozen crumb-app revision/baseline or investigated as a separate behavior change; this task does not alter crumb-app or rewrite its baseline to manufacture the expected totals.

## Resumed acceptance diagnosis and fix round

The initial count failure was not accepted as a revision assumption. A detached control worktree at the exact pre-repair base, `c077933`, was built and run against the same clean crumb-app checkout and flags. All artifacts are retained outside both repositories:

- control: `/private/tmp/crumb-structural-control-scan.json`, `/private/tmp/crumb-structural-control-baseline-audit.json`, `/private/tmp/crumb-structural-control-baseline-prune-check.json`
- initial branch: `/private/tmp/crumb-structural-origin-scan.json`, `/private/tmp/crumb-structural-origin-baseline-audit.json`, `/private/tmp/crumb-structural-origin-baseline-prune-check.json`
- final fix round: `/private/tmp/crumb-structural-origin-scan-final.json`, `/private/tmp/crumb-structural-origin-baseline-audit-final.json`, `/private/tmp/crumb-structural-origin-baseline-prune-check-final.json`

### Initial 333-finding classification

This table exhaustively partitions the initial unsuppressed result by rule and analyzed language:

| Rule | Go | C++ | Total | Recurring source pattern |
|---|---:|---:|---:|---|
| `design.persistence-model-leak` | 7 | 82 | 89 | ORM/storage records, tasks, and DB concepts exposed at boundaries |
| `function.hidden-mutation` | 25 | 60 | 85 | proven receiver/reference/global writes and calls; false lexical families investigated below |
| `function.command-query-mix` | 16 | 25 | 41 | value-returning functions with the same proven mutation evidence |
| `design.temporal-coupling` | 25 | 4 | 29 | call/state ordering dependencies |
| `design.pass-through-abstraction` | 26 | 0 | 26 | forwarding wrappers without added policy |
| `smell.message-chain` | 1 | 18 | 19 | long member/call chains |
| `design.configuration-leak` | 1 | 10 | 11 | configuration concepts crossing boundaries |
| `naming.behavior-mismatch` | 7 | 0 | 7 | query-shaped Go names with structural effects |
| `quality.hidden-side-effect` | 7 | 0 | 7 | same Go structural evidence surfaced by the quality rule |
| `design.infrastructure-type-leak` | 0 | 4 | 4 | infrastructure types exposed by APIs |
| `function.orchestration-domain-mix` | 3 | 0 | 3 | domain decisions mixed with orchestration |
| `defensive.unvalidated-boundary-input` | 2 | 0 | 2 | boundary inputs used without validation |
| `error.generic-message` | 2 | 0 | 2 | generic error prose |
| `naming.cardinality-mismatch` | 2 | 0 | 2 | scalar/collection naming mismatch |
| `defensive.integer-overflow` | 1 | 0 | 1 | unchecked integer operation |
| `error.cleanup-error-ignored` | 1 | 0 | 1 | ignored cleanup failure |
| `naming.role-suffix-overuse` | 1 | 0 | 1 | generic role suffix |
| `quality.duplicated-knowledge` | 1 | 0 | 1 | repeated encoded knowledge |
| `smell.data-clump` | 1 | 0 | 1 | repeated parameter group |
| `smell.god-object` | 1 | 0 | 1 | oversized responsibility surface |
| **Total** | **130** | **203** | **333** | |

Structural metadata was complete and non-guessed:

| Rule/language | Argument / caller-owned | Receiver / caller-owned | Global / shared | Total |
|---|---:|---:|---:|---:|
| C++ hidden mutation | 26 | 32 | 2 | 60 |
| C++ command/query | 18 | 6 | 1 | 25 |
| Go hidden mutation | 3 | 22 | 0 | 25 |
| Go command/query | 8 | 8 | 0 | 16 |

The recurring hidden-mutation evidence consisted of Go `r.pool.Exec` (14 receiver cases plus one argument `pool.Exec`), stateful service/loader receivers, and explicit map/pointer arguments. C++ was dominated by repository `context_` receiver binding (9), coroutine/member `state` writes (5), `error` and `out` references (4 each), event dispatchers and shutdown flags (3 each), and explicit reference operations such as `clear`, `assign`, and `execPrepared`. Those proven operations remain covered rather than being discarded to meet a count.

The named acceptance sources (`badge_helpers.go`, `brand_helpers.go`, `place_helpers.go`, `OvertureDatasetPath`, `NewStaticOvertureDivisionResolver`, `duckDBDivisionHierarchy`, the local fluent builders, and `DbRow::integer`) have zero findings in the initial and final branch JSON. Their reduced fixtures remain green alongside the genuine receiver/reference/global/escaped negatives.

### Concrete gaps and RED evidence

Four source-independent defects accounted for the remaining false structural family:

1. `(?i)` applied to the entire mutation/read regex, including its `[A-Z...]` camel-case boundary. Consequently `addedCellIds` matched `add` and `savedPlateIds` matched `save`.
2. Observable-effect classification inspected every token in a callee. Receiver/member words such as `savedPlateIds`, `upload`, and `prefetch` could therefore manufacture persistence/network effects even when the terminal method was read-only.
3. The C++ function-head parser interpreted `if constexpr (requires(...))` as a nested function named `constexpr`.
4. Constructor syntax and qualified method names were not honored by hidden-mutation naming: constructors appeared hidden, and `TraceSpan::setStatus` did not apply command semantics to leaf method `setStatus`.

The RED command was:

`GOCACHE=/private/tmp/codeguard-go-cache-red2 szr proxy go test ./internal/codeguard/checks/quality ./internal/codeguard/checks/support -run 'TestCppOriginReadOnlyMemberCalls|TestCppParserDoesNotTreatIfConstexpr' -count=1 -v`

- all four source-shaped read calls (`savedPlateIds.size`, `addedCellIds.size`, `savedPlateIds.contains`, and `savedPlateIds.empty`) produced caller-owned mutation evidence;
- the parser produced a nested `ParsedFunction{Name:"constexpr"}`;
- separate RED runs proved constructor initialization and `TraceSpan::setStatus` still produced hidden-mutation evidence.

The repair scopes case-insensitivity to verb tokens while keeping the camel boundary case-sensitive, classifies observable effects from terminal method identifier words, rejects the C++ keyword as a function head, recognizes constructor/destructor syntax structurally, and applies naming semantics to a qualified method's leaf name. It adds no crumb path/function/method allowlist, suppression, waiver, or baseline change.

### Final GREEN and acceptance results

- focused read/parser/constructor/qualified-name regressions: PASS (2 packages)
- required focused suite: PASS (4 packages)
- full `go test ./...`: PASS (27 packages)
- `go vet ./...`: PASS
- pinned golangci-lint v2.12.2: PASS (`0 issues.`)
- branch binary build and `git diff --check`: PASS

The final scan removed 12 initial unsuppressed false findings and added none. The removed family comprises four read-only C++ calls, two phantom `constexpr` functions, two constructors, one qualified explicit command, and three whole-callee substring effects in Go. Final exact results:

| Measure | Go | C++ | Total |
|---|---:|---:|---:|
| Unsuppressed findings | 127 | 194 | 321 |
| Unresolved mutation symbols | 1,921 | 3,113 | 5,034 |

- suppressed findings: 5,409
- baseline before: 6,560
- active: 5,275 (`active_context`: 5,275; exact/content: 0)
- stale: 1,285
- prune-check simulated removal: 1,285; simulated final: 5,275; invalid: 0
- unresolved diagnostics remain present independently for Go and C++
- crumb-app remained clean at `8ffc82b295b1bd6358a778d6dabacd1fdccc712b`

### Mathematical acceptance contradiction

The pre-repair control on the same source/configuration produced 645 unsuppressed findings, 5,807 suppressed findings, and exactly 887 stale entries. Its unsuppressed findings consist of 452 structural findings plus an unchanged **193-finding non-structural floor**:

- 159 Design Patterns findings (89 persistence-model, 29 temporal-coupling, 26 pass-through, 11 configuration-leak, 4 infrastructure-type-leak);
- 34 other quality findings, including the 19 message-chain findings.

Therefore a structural-origin repair cannot produce a total of at most 19 under the required full configuration: even eliminating every structural and structurally-derived finding leaves 193. The observed `19` is the count of `smell.message-chain` alone, not the control's total unsuppressed count.

The audit arithmetic is deterministic but does not by itself prove why an entry stopped matching: with 6,560 entries, `stale = 6,560 - active`. The control has 5,673 active entries and 887 stale; this repair round's current audit has 5,261 active and 1,299 stale. Reaching 887 with the same baseline would require 412 additional distinct current context matches. The audit artifacts prove the match counts only; they do **not** prove that every newly stale entry is obsolete, and this report does not make that claim.

Accordingly the code work is verified, while the `<=19` full-configuration gate is impossible for the supplied checkout because the independently measured non-structural floor is 193. The stale result is reported separately and conservatively from the audit artifact.

## Review fix round 1 — grammatical effects, command names, and Go fresh copies

Review artifacts:

- scan: `/private/tmp/crumb-structural-origin-scan-review1.json`
- audit: `/private/tmp/crumb-structural-origin-baseline-audit-review1.json`
- prune check: `/private/tmp/crumb-structural-origin-baseline-prune-check-review1.json`
- remaining structural listing: `/private/tmp/crumb-structural-origin-remaining-review1.tsv`
- previous-to-current location delta: `/private/tmp/crumb-structural-origin-delta-review1.tsv`

### RED and repair evidence

The focused RED run covered all three review families before production changes:

`GOCACHE=/private/tmp/codeguard-go-cache-review-red szr proxy go test ./internal/codeguard/checks/quality -run 'TestObservableCallEffectPreservesReceiverGrammar|TestExplicitCommandNamesUseLeadingVerbGrammar|TestQueryLikeNameWithProvenMutationStillReports|TestGoStructuralOriginFixtureUnresolvedEvidenceIsIntentional' -count=1 -v`

- `http.Post`, qualified HTTP `Put`/`Patch`, cache `Set`/`Put`, and `fetchQueue.Enqueue` had no observable effect; existing `dispatcher.tryEnqueue` and the read-only receiver negatives remained correct.
- all requested command-leading names (`MarkRead`, `RevokeCreatorInvite`, `DeactivateUser`, `ConfigureSafety`, `shutdown`, qualified `bind`, `release`, `discard`, `Subscribe`, and `IssueAdminAccessToken`) were unrecognized; the query-name negative controls and a query with proven receiver mutation already behaved correctly.
- `NewStaticOvertureDivisionResolver` emitted two false unresolved assignments for `copied`, in addition to the intentional unresolved `mystery` call.

The repair now classifies the terminal method and receiver as separate token grammar. Event verbs are terminal-only; HTTP `Post`/`Put`/`Patch` require an `http` receiver token; cache `Set`/`Put` require a `cache` receiver token. It therefore restores `network`, `persistence`, and `event` effects without returning to whole-callee substring matching. Explicit commands use an exact leading-verb vocabulary on the leaf of qualified names rather than a raw prefix or per-repository function allowlist.

For Go, `append` now derives its result shape and ownership from its first slice. Fresh `[]T(nil)`, `[]T{}`, and `make([]T, 0, ...)` destinations remain local, while `append(input[:0], ...)` retains caller backing-storage ownership. The named fixture now has exactly one Go unresolved record: language `go`, operation `call`, symbol `mystery`, reason `call target ownership could not be resolved`, at its source line. The prior two `copied` assignment records are absent because their fresh local ownership is now proven; the integration expectation changes from Go 3 to Go 1 for that reason. C++ remains at one genuinely incomplete fixture record.

### GREEN and repository verification

- focused review regressions, including fresh-copy negatives and caller-backing positive: PASS
- structural-origin integration and function-effect suites: PASS
- required focused suite: PASS (4 packages; rerun outside the filesystem sandbox because existing `httptest` cases bind localhost)
- full `go test ./...`: PASS (27 packages; same localhost permission)
- `go vet ./...`: PASS
- pinned golangci-lint v2.12.2: PASS (`0 issues.`)
- branch binary build and `git diff --check`: PASS

### Current crumb-app result and remaining structural classification

The new binary ran the same mutation-free scan/audit/prune-check commands and flags documented above. Scan and audit exited 0; prune check exited 1 because check-only mode found stale entries. crumb-app remained clean at `8ffc82b295b1bd6358a778d6dabacd1fdccc712b`.

| Measure | Go | C++ | Total |
|---|---:|---:|---:|
| Unsuppressed findings | 118 | 181 | 299 |
| Unresolved mutation symbols | 1,760 | 3,125 | 4,885 |

- suppressed findings: 5,395
- baseline before: 6,560
- active: 5,261 (`active_context`: 5,261; exact/content: 0)
- stale: 1,299
- prune-check simulated removal: 1,299; simulated final: 5,261; invalid: 0

The 106 remaining structural/structurally-derived unsuppressed findings partition exactly as follows; the other 193 findings are the independently measured non-structural floor:

| Rule | Go | C++ | Total | Dominant proved source patterns |
|---|---:|---:|---:|---|
| `function.hidden-mutation` | 15 | 37 | 52 | Go repository `r.pool.Exec` receivers and explicit receiver/argument writes; C++ output/error references, coroutine/member state, event dispatch, global state, and proven persistence calls |
| `function.command-query-mix` | 14 | 26 | 40 | value-returning helpers with the same proved receiver/argument/global mutations |
| `naming.behavior-mismatch` | 7 | 0 | 7 | Go query-shaped names whose proved mutation remains intentionally visible |
| `quality.hidden-side-effect` | 7 | 0 | 7 | the corresponding Go structural evidence surfaced by the quality rule |
| **Total** | **43** | **63** | **106** | |

For the two direct structural rules, all metadata remains ownership-proven. Go has 15 hidden mutations (9 receiver persistence, 5 receiver shared-state, 1 argument shared-state) and 14 command/query findings (8 argument and 6 receiver shared-state). C++ has 37 hidden mutations (22 argument, 13 receiver, 2 global) and 26 command/query findings (18 argument, 7 receiver, 1 global); effect metadata includes the restored event and persistence cases.

Relative to the prior final scan, this round removes 24 unsuppressed structural locations and adds the two genuine cache-`put` findings at `request_planner_profile_generation_store.cpp:131`, for a net reduction of 22 (321 to 299). The location delta and complete 92-row direct structural listing are retained in the artifacts above. No crumb source, baseline, suppression, waiver, or cache file was modified.

## Review fix round 2 — Go reference-backed conversions

The focused RED command was:

`GOCACHE=/private/tmp/codeguard-go-cache-review2-red szr proxy go test ./internal/codeguard/checks/quality -run 'TestGoReferenceBackedConversionsPreserveCallerOwnership|TestGoFreshReferenceBackedConversionsRemainLocal' -count=1 -v`

- unnamed `[]Item(input)` and `map[string]*Item(input)` conversions were incorrectly classified as local and emitted no caller mutation;
- declared slice/map conversions became unresolved because the call-shaped syntax was not connected to the package type declaration;
- the existing unnamed nil conversions remained correctly local, while declared fresh conversions exposed the same declaration gap.

The resolver now identifies conversion syntax from an AST type expression or an indexed declared type, respecting lexical value shadowing. A reference-backed conversion inherits its operand's symbol and ownership. Nil, basic/composite literals, address-of fresh literals, builtin allocation, and recursively fresh conversion operands remain local. A non-fresh operand whose ownership cannot be resolved stays diagnostic-only rather than being guessed local. Conversion calls whose declared type has a mutator-shaped name such as `Set` are recognized by declaration, not by their spelling, and do not create spurious call diagnostics.

Focused coverage proves:

- caller-owned unnamed and declared slice conversions report `mutation_target=argument`, `origin=caller_owned`;
- caller-owned unnamed and declared map conversions report the same exact ownership;
- unnamed/declared nil slice conversions and unnamed nil/declared literal map conversions remain local with no mutation or unresolved record;
- a declared reference conversion over an unresolved call result produces exactly one assignment diagnostic for symbol `copied` with reason `symbol ownership or reference shape could not be resolved`;
- the earlier fresh-append, reused-backing-array, and named-fixture unresolved invariants remain green.

Verification:

- focused conversion and prior Go ownership regressions: PASS
- complete `internal/codeguard/checks/quality` package: PASS
- structural-origin/function-effect integration selection in `tests/checks`: PASS
- `go vet ./...`: PASS
- branch binary build and `git diff --check`: PASS

Because the change can reveal a genuine caller mutation, a scan-only acceptance comparison was run and retained at `/private/tmp/crumb-structural-origin-scan-review2.json`. Its finding set is identical to round 1: 299 unsuppressed (Go 118, C++ 181), 5,395 suppressed, with no added or removed rule/path/line location. The Go unresolved diagnostic count increases conservatively from 1,760 to 1,761; C++ remains 3,125. Since the baseline-relevant finding set did not change, audit/prune were not rerun as permitted by this review round. crumb-app was not modified.

## Review fix round 3 — indexed types and string-backed slice allocation

The focused RED command was:

`GOCACHE=/private/tmp/codeguard-go-cache-review3-red szr proxy go test ./internal/codeguard/checks/quality -run 'TestGoReferenceBackedConversionsPreserveCallerOwnership|TestGoStringToByteAndRuneSliceConversionsAllocateFreshStorage' -count=1 -v`

- `Set[Item](input)` and `ItemsByKey[string, *Item](input)` produced unresolved assignment records instead of preserving caller ownership;
- later element writes through `[]byte(inputString)`, `[]rune(inputString)`, and `[]byte(namedString)` were incorrectly reported as caller-owned mutation;
- the pre-existing non-generic slice/map alias cases remained green during RED, localizing the gaps to indexed type syntax and string conversion allocation.

The resolver now handles `ast.IndexExpr` and `ast.IndexListExpr` as type expressions only when their base resolves to a declared type, and derives the shape from that base declaration. This preserves the declared slice/map reference kind without a type-name allowlist. Generic single- and multi-parameter conversions therefore carry the non-fresh operand's caller ownership into later element writes.

String-to-slice allocation is handled separately from ordinary reference conversion. The resolver compares the source's declaration-resolved underlying scalar type with the target slice element's declaration-resolved underlying scalar type. String to byte/`uint8` or rune/`int32` produces fresh local backing storage, including a declared string source type; ordinary slice/map conversion continues to alias its operand. No variable/function spelling participates in that decision.

GREEN verification:

- focused indexed, string-to-byte/rune, ordinary alias, fresh, and unknown conversion regressions: PASS
- complete `internal/codeguard/checks/quality` package: PASS
- structural-origin/function-effect integration selection in `tests/checks`: PASS
- `go vet ./...`: PASS
- branch binary build: PASS
- `git diff --check`: PASS

No crumb scan was required for this review round; the acceptance artifacts and external full-configuration gate analysis remain as recorded above.
