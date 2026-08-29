# Structural Origin Classification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace guessed structural-mutation ownership with declaration-, scope-, and alias-based evidence in Go and C++, while preserving baseline identity and measuring unresolved analysis coverage.

**Architecture:** Keep the existing flattened precision model for unrelated rules, but route structural mutation analysis through language-specific semantic resolvers. Both resolvers emit the existing mutation metadata plus non-finding unresolved evidence; the quality section aggregates unresolved counts into diagnostics. Finding identity is derived from stable rule/source identity rather than message or evidence prose.

**Tech Stack:** Go, `go/ast`, CodeGuard's C-like parser/tree-sitter integration, existing report diagnostics and baseline matcher.

**Spec:** `docs/superpowers/specs/2026-08-29-structural-origin-classification-design.md`

## Global Constraints

- Unknown symbols must never generate mutation findings by guessed ownership.
- Preserve unresolved operations internally and report aggregate unresolved-symbol counts by language.
- Do not add constructor/method allowlists, repository suppressions, waivers, new policy rules, or a strict/debug configuration surface.
- Scope lookup must honor shadowing and nested closures/lambdas.
- Alias traversal must support simple multi-hop chains with cycle detection and a fixed bound.
- Default findings require proven argument, receiver, global, shared, or escaped ownership.
- Diagnostic messages and evidence metadata must not affect exact, context, or content fingerprint identity.
- Analysis must remain bounded and function-local plus file/package declaration indexing.
- Final crumb-app acceptance is at most 19 unsuppressed findings with 887 verified stale entries retained.

---

### Task 1: Stable Finding Identity and Unresolved Evidence Contract

**Files:**
- Modify: `internal/codeguard/runner/support/findings_section.go`
- Modify: `internal/codeguard/runner/support/context_fingerprint.go`
- Modify: `internal/codeguard/checks/quality/quality_effect_evidence.go`
- Modify: `internal/codeguard/checks/quality/quality.go`
- Modify: `internal/codeguard/checks/quality/quality_scan_language.go`
- Test: `tests/support/context_fingerprint_test.go`
- Test: `tests/checks/fingerprint_baseline_test.go`
- Test: `internal/codeguard/checks/quality/quality_effect_evidence_test.go`
- Test: `internal/codeguard/report/diagnostics_test.go`

**Interfaces:**
- Produces: `unresolvedMutationEvidence{Language, Line, Operation, Symbol, Reason}` and aggregate quality diagnostics keyed by language.
- Produces: stable source-derived `Finding.Fingerprint` that excludes `Message` and `Metadata`.
- Consumes: existing `core.Finding`, `core.Diagnostic`, and deterministic exact/context/content baseline matching.

- [ ] **Step 1: Write failing fingerprint compatibility tests**

Add test helpers that build two findings for the same rule, path, line, and source while changing only `Message` and mutation metadata. Assert all three fingerprints are equal. Add a baseline regression that seeds the pre-change exact fingerprint plus current context/content fingerprints, changes prose/evidence, runs audit matching, and asserts the entry is active rather than stale.

```go
first := newFindingWithMessageAndMetadata(t, "mutates argument state", map[string]string{"origin": "caller_owned"})
second := newFindingWithMessageAndMetadata(t, "mutates state owned by argument", map[string]string{"origin": "shared"})
if first.Fingerprint != second.Fingerprint || first.ContextFingerprint != second.ContextFingerprint || first.ContentFingerprint != second.ContentFingerprint {
	t.Fatal("diagnostic prose or evidence metadata changed finding identity")
}
```

- [ ] **Step 2: Run the focused fingerprint tests and verify red**

Run: `GOCACHE=/private/tmp/codeguard-go-cache szr go test ./tests/support ./tests/checks -run 'Test.*Fingerprint.*(Message|Metadata|Baseline)' -count=1`

Expected: FAIL because `NewFinding` currently includes `Message` in the exact fingerprint.

- [ ] **Step 3: Make exact identity source-derived and retain fallback compatibility**

Change exact fingerprint construction to hash a versioned stable tuple containing rule ID, normalized path, line, and normalized source identity. Do not include message, confidence, or metadata. Keep context/content construction unchanged and ensure baseline matching still falls back exact → context → content for entries created by earlier versions.

```go
identity := strings.Join([]string{"v2", input.RuleID, normalizedPath, strconv.Itoa(input.Line), sourceIdentity}, "|")
finding.Fingerprint = fingerprint(identity)
```

- [ ] **Step 4: Add failing unresolved-evidence tests**

Define unit cases for unresolved Go and C++ roots. Assert they produce zero mutation findings, retain records with language/reason, and aggregate into two distinct diagnostics without becoming baseline entries.

```go
type unresolvedMutationEvidence struct {
	Language  string
	Line      int
	Operation string
	Symbol    string
	Reason    string
}
```

- [ ] **Step 5: Implement the shared unresolved contract and aggregation**

Change structural evidence collection to return a result carrying `[]mutationEvidence` and `[]unresolvedMutationEvidence`. Add a quality-section accumulator that emits one informational diagnostic per language with rule/code `quality.structural-unresolved-symbols` and metadata containing `language` and decimal `count`. Unknown roots must enter this collection instead of defaulting to `targetGlobal/originShared`.

```go
type mutationAnalysis struct {
	Mutations  []mutationEvidence
	Unresolved []unresolvedMutationEvidence
}
```

- [ ] **Step 6: Run focused tests, then commit**

Run: `GOCACHE=/private/tmp/codeguard-go-cache szr go test ./internal/codeguard/checks/quality ./internal/codeguard/report ./tests/support ./tests/checks -run 'Test.*(Fingerprint|Unresolved|Diagnostic)' -count=1`

Expected: PASS.

Commit: `fix: stabilize structural finding identity`

---

### Task 2: Go Declaration, Scope, and Alias Resolver

**Files:**
- Create: `internal/codeguard/checks/quality/quality_effect_evidence_go.go`
- Create: `internal/codeguard/checks/quality/quality_effect_evidence_go_test.go`
- Modify: `internal/codeguard/checks/quality/quality_precision.go`
- Modify: `internal/codeguard/checks/quality/quality_go.go`
- Modify: `internal/codeguard/checks/quality/quality_scan_language.go`
- Test: `tests/checks/function_effect_evidence_test.go`

**Interfaces:**
- Consumes: Task 1 `mutationAnalysis`, `mutationEvidence`, and `unresolvedMutationEvidence`.
- Produces: `goFunctionMutationEvidence(fn precisionFunction) mutationAnalysis` backed by declaration identities, nested scopes, reference shapes, and ordered operations.
- Produces: a per-directory/per-package index of imports, package variables, and local named struct field shapes.

- [ ] **Step 1: Add failing Go scope and import regressions**

Add table tests proving `fmt`, imported aliases, keywords, locals shadowing globals, locals in nested blocks, and closure-local shadows produce no findings. Assert an executed closure capturing and mutating a pointer parameter still reports argument/caller-owned evidence. Assert unresolved names increment the Go unresolved count without findings.

- [ ] **Step 2: Add failing Go value/reference-shape tests**

Use a value struct containing scalar, map, slice, pointer, and interface fields. Assert field reassignment is local, while map index, slice index, pointee field, and proven interface-contained reference mutation report through caller-owned content. Add `a := input; b := a` multi-hop cases for pointers/maps/slices and a value-copy negative.

```go
type Payload struct {
	Name string
	Tags []string
	Meta map[string]string
	Node *Node
	Any  any
}
```

- [ ] **Step 3: Run the Go-focused tests and verify red**

Run: `GOCACHE=/private/tmp/codeguard-go-cache szr go test ./internal/codeguard/checks/quality ./tests/checks -run 'Test.*Go.*(Origin|Scope|Alias|Reference|Closure)|TestFunctionEffects.*Go' -count=1`

Expected: FAIL because current evidence maps ownership by textual name and guesses unknown roots as globals.

- [ ] **Step 4: Build the Go semantic model from AST declarations**

Retain AST nodes or a compact semantic representation in `precisionFunction`. Create distinct symbol IDs for imports, receiver, parameters, `var`/`const`, newly introduced `:=` names, range variables, initializer scopes, and function-literal parameters/locals. Resolve nearest lexical declaration first. Index package globals/types by directory and package name.

```go
type goSymbol struct {
	ID       int
	Name     string
	Kind     goSymbolKind
	Scope    goScopeID
	Shape    goReferenceShape
	Origin   string
	AliasOf  int
	DeclLine int
}
```

- [ ] **Step 5: Implement ordered ownership and multi-hop alias evidence**

Walk mutations in source order. Follow alias IDs with cycle detection and a constant hop bound. Value copies remain local. Cross a caller-owned boundary only through a pointer, map, slice, or otherwise proven reference-backed field/content path. Mark a local escaped only after storage into a resolved global or caller-owned reachable location; only later mutation reports escaped/shared.

- [ ] **Step 6: Route Go structural rules through the resolver**

Use `goFunctionMutationEvidence` for Go hidden-mutation and command/query evidence. Keep unrelated precision rules on the flattened model. Imported packages and unresolved symbols must be non-targets; unresolved operations enter Task 1 diagnostics.

- [ ] **Step 7: Run focused and package tests, then commit**

Run: `GOCACHE=/private/tmp/codeguard-go-cache szr go test ./internal/codeguard/checks/quality ./tests/checks -run 'Test.*(Go|FunctionEffects|HiddenMutation)' -count=1`

Expected: PASS, including genuine receiver/argument/global/escaped negatives.

Commit: `fix: resolve Go structural mutation ownership`

---

### Task 3: C++ Declaration, Scope, Reference, and Lambda Resolver

**Files:**
- Create: `internal/codeguard/checks/quality/quality_effect_evidence_cpp.go`
- Create: `internal/codeguard/checks/quality/quality_effect_evidence_cpp_test.go`
- Modify: `internal/codeguard/checks/support/parser_types.go`
- Modify: `internal/codeguard/checks/support/parser_clike.go`
- Modify: `internal/codeguard/checks/support/parser_clike_functions.go`
- Modify: `internal/codeguard/checks/support/parser_clike_scope.go`
- Modify: `internal/codeguard/checks/quality/quality_precision.go`
- Test: `tests/checks/function_effect_evidence_test.go`

**Interfaces:**
- Consumes: Task 1 `mutationAnalysis`, `mutationEvidence`, and `unresolvedMutationEvidence`.
- Produces: declaration/scope metadata in `support.ParsedFunction` and `cppFunctionMutationEvidence(fn precisionFunction) mutationAnalysis`.
- Preserves existing C-like parser consumers by adding metadata rather than changing current assignment/call fields.

- [ ] **Step 1: Add failing C++ declaration and shadowing tests**

Cover `T x;`, `T x{}`, `auto x = T{}`, `constexpr`, constructors, member initializers, templates, qualified `DbRow::integer`, local builders, and locals shadowing globals or members. Assert keywords, types, qualified scopes, and locals never become mutation targets.

- [ ] **Step 2: Add failing C++ ownership and lambda tests**

Assert value parameters and moved locals remain local; `T&` and `T*` mutations report; simple multi-hop reference/pointer aliases report; lambda value captures remain local and reference captures retain resolved ownership. Add genuine receiver/member, proven global, and escaped-local positives.

- [ ] **Step 3: Run C++-focused tests and verify red**

Run: `GOCACHE=/private/tmp/codeguard-go-cache szr go test ./internal/codeguard/checks/support ./internal/codeguard/checks/quality ./tests/checks -run 'Test.*Cpp.*(Origin|Scope|Alias|Lambda|Constructor)|Test.*DbRow' -count=1`

Expected: FAIL because declaration syntax is incomplete and unresolved roots default to global/shared.

- [ ] **Step 4: Extend the C-like parser with declaration and scope metadata**

Record lexical span, declaration kind, type/reference shape, initializer alias source, qualified owner, lambda captures, and file-level global/member declarations. Ensure constructor member-initializer braces do not terminate function-head discovery. Treat keywords/type tokens as syntax, never declarations or mutation roots.

```go
type ParsedDeclaration struct {
	Name, Type, Kind, ReferenceShape string
	Line, ScopeStart, ScopeEnd        int
	AliasSource                      string
}
```

- [ ] **Step 5: Implement C++ lookup and bounded alias traversal**

Resolve innermost local, capture, parameter, member, then proven global. Propagate caller ownership only through `&`/`*` paths or reference captures. Keep value copies, `std::move(local)`, constructor-created values, template locals, and local builders local. Unknown calls/results remain unresolved evidence.

- [ ] **Step 6: Route C++ structural evidence through the resolver**

Use `cppFunctionMutationEvidence` for C++ hidden-mutation and command/query rules. Preserve existing parsing fields for unrelated rules. Emit unresolved C++ operations through Task 1 diagnostics rather than guessed findings.

- [ ] **Step 7: Run focused and parser tests, then commit**

Run: `GOCACHE=/private/tmp/codeguard-go-cache szr go test ./internal/codeguard/checks/support ./internal/codeguard/checks/quality ./tests/checks -run 'Test.*(Cpp|DbRow|FunctionEffects|HiddenMutation)' -count=1`

Expected: PASS, including genuine reference/pointer/receiver/global/escaped negatives.

Commit: `fix: resolve C++ structural mutation ownership`

---

### Task 4: Named Regressions, Invariants, and crumb-app Acceptance

**Files:**
- Modify: `tests/checks/function_effect_evidence_test.go`
- Create: `tests/checks/testdata/structural_origin/go/helpers.go`
- Create: `tests/checks/testdata/structural_origin/cpp/helpers.cpp`
- Modify: `tests/checks/fingerprint_baseline_test.go`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: Task 2 Go resolver, Task 3 C++ resolver, Task 1 diagnostics/fingerprints.
- Produces: end-to-end release regression coverage and recorded crumb-app acceptance evidence.

- [ ] **Step 1: Add named regression fixtures**

Create reduced, source-shaped cases from `badge_helpers.go`, `brand_helpers.go`, `place_helpers.go`, `OvertureDatasetPath`, `NewStaticOvertureDivisionResolver`, `duckDBDivisionHierarchy`, C++ local fluent builders, and `DbRow::integer`. Assert no structural finding for each and assert unresolved counts only where resolution is genuinely incomplete.

- [ ] **Step 2: Add negative-fixture invariants**

In the same fixtures add proven Go/C++ receiver, argument/reference, global, and escaped mutations. Assert their rule IDs and exact `mutation_target`, `effect_kind`, and `origin` metadata so false-positive removal cannot erase genuine coverage.

- [ ] **Step 3: Add baseline upgrade invariants**

Generate findings whose only differences are diagnostic prose and evidence metadata, seed entries using prior exact plus context/content identity, and assert audit/prune keeps active entries matched while resolved false positives become stale without waivers.

- [ ] **Step 4: Run all focused regression suites**

Run: `GOCACHE=/private/tmp/codeguard-go-cache szr go test ./internal/codeguard/checks/quality ./internal/codeguard/checks/support ./tests/checks ./tests/support -count=1`

Expected: PASS.

- [ ] **Step 5: Build the branch binary and run crumb-app acceptance**

Build: `GOCACHE=/private/tmp/codeguard-go-cache szr go build -o /private/tmp/codeguard-structural-origin ./cmd/codeguard`

From `/Users/agentcarl/Documents/Github/crumb-app`, run the branch binary with `.codeguard/codeguard.yaml`, profile `startup`, mode `full`, JSON output, cache disabled, and govulncheck disabled. Parse the JSON without modifying crumb-app. Assert no more than 19 unsuppressed findings, exactly 887 verified stale entries retained by audit/prune check semantics, genuine structural negatives still present in CodeGuard fixtures, and unresolved counts exist per analyzed language.

- [ ] **Step 6: Run full repository verification**

Run: `GOCACHE=/private/tmp/codeguard-go-cache szr go test ./...`

Run: `GOCACHE=/private/tmp/codeguard-go-cache szr go vet ./...`

Run the repository-pinned golangci-lint command from `.github/workflows/ci.yml`.

Expected: all commands PASS.

- [ ] **Step 7: Document the repair and commit**

Add a concise `CHANGELOG.md` entry describing declaration-based Go/C++ ownership, unresolved diagnostics, and wording-independent fingerprint identity. Do not advertise a strict/debug mode.

Commit: `test: lock structural origin classification acceptance`
