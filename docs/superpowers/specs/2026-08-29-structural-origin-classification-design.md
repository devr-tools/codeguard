# Structural Origin Classification Repair Design

## Scope

This repair removes the v1.8.2 structural false-positive regression by replacing name- and token-based ownership guesses with declaration-, scope-, and alias-based evidence. It applies to Go and C++ structural mutation rules without adding allowlists, repository suppressions, new policy rules, or whole-program analysis.

The default rules report only proven mutation of caller-owned, receiver-owned, global, shared, or escaped state. Unresolved symbols remain internal evidence, are counted by language for coverage monitoring, and do not become mutation findings. A future strict or debug mode may expose unresolved cases, but that mode is outside this repair.

## Shared Evidence Model

Each analyzed function receives a bounded lexical scope model built from its parsed declarations. A symbol record contains its language, declaration location, lexical scope, declaration kind, type/reference shape, ownership origin, mutability capabilities, and any resolved alias source. Symbol lookup starts in the innermost scope and walks outward, so local declarations, nested closures or lambdas, and shadowing resolve deterministically.

Ownership values remain `locally_allocated`, `caller_owned`, `shared`, and `unknown`. Mutation targets remain `local`, `argument`, `receiver`, `global`, and `escaped`. `unknown` is not promoted to `global`: unresolved operations are recorded as unresolved evidence with a language and reason, excluded from default findings, and included in aggregate unresolved-symbol counts.

Aliases are edges between resolved symbols rather than textual name substitutions. The analysis follows simple multi-hop chains within the function and nested closures, with cycle detection and a bounded traversal depth. Mutation through an argument requires an actual reference-capable path to caller-owned storage. Value copies do not inherit caller ownership merely because their source was a parameter.

## Go Resolution

Go declarations are resolved from function receivers, parameters, local declarations, short declarations, range variables, nested function literals, imported package names, and package declarations. Imported packages are symbol kind `package` and cannot be mutation targets. Nearest lexical declaration wins under shadowing.

Go ownership distinguishes binding reassignment from referenced-content mutation:

- A value parameter and a local value copy remain local when fields are reassigned.
- A pointer parameter can expose caller-owned pointee mutation.
- Maps, slices, pointers, interfaces holding reference-backed values, and fields of value structs that contain those shapes are reference-capable only for operations that mutate their referenced contents.
- Reassigning a map, slice, pointer, or interface field on a copied value struct mutates only the local copy.
- Index writes, dereferences, append/copy effects assigned through the original reference, and method calls with proven mutation semantics may mutate referenced caller-owned contents.
- Newly declared maps, slices, builders, DTOs, constructor results, and result values are `locally_allocated` unless a proven escape occurs.

Nested closures capture the resolved outer symbol. A mutation finding is produced only when the captured symbol's ownership and reference shape prove an externally observable mutation.

## C++ Resolution

C++ declarations are resolved from parameters, references, pointers, local declarations, members, constructors, member initializers, templates, lambdas, and surrounding globals. Language keywords and declaration syntax, including `auto`, `constexpr`, `const`, move syntax, type names, and constructor names, cannot become mutation targets.

References and pointers preserve the ownership of their resolved source through simple multi-hop aliases. Value parameters, moved local values, constructor-created objects, template-local variables, and local builders remain local unless they escape or mutate through a proven caller-owned reference or pointer. Member access resolves against `this` or an explicit object; unqualified local declarations shadow members and globals. Lambdas use their capture form and resolved declaration to distinguish value captures from reference captures.

## Finding and Diagnostic Behavior

Structural findings retain the existing rule IDs. Every reported mutation continues to include `mutation_target`, `effect_kind`, and `origin` metadata and explains the resolved mutation path. Evidence wording can evolve without changing finding identity.

Unresolved evidence records at least language, source location, operation kind, textual symbol, and resolution failure reason. Reports expose aggregate unresolved-symbol counts by language through the existing diagnostics/reporting mechanism; unresolved records are not security or quality findings, are not baselinable, and do not affect the default finding count.

## Fingerprint and Baseline Compatibility

Exact and context fingerprints derive from stable rule, path, source location/context, and source identity. Diagnostic message text and evidence metadata are excluded. Tests must prove that changing a finding's message or mutation metadata leaves its fingerprints stable.

Existing entries continue matching through the deterministic exact-first, context-second, content-last baseline matcher. Wording-only changes therefore do not make entries stale, and the upgrade does not rewrite baseline identity.

## Regression and Acceptance Coverage

Tests are written before implementation and include focused regressions derived from:

- `badge_helpers.go`, `brand_helpers.go`, and `place_helpers.go`;
- `OvertureDatasetPath` and `NewStaticOvertureDivisionResolver`;
- `duckDBDivisionHierarchy`;
- C++ local builder patterns; and
- `DbRow::integer`.

Go coverage includes imported packages, nested scopes and closures, shadowing, local allocations, multi-hop aliases, and value structs containing maps, slices, pointers, and interfaces. It distinguishes field reassignment from referenced-content mutation. Negative fixtures retain real receiver, argument, global, escaped, and reference-backed mutations.

C++ coverage includes references, pointers, member access, move operations, templates, constructors, member initializers, nested lambdas, local builders, and locals shadowing globals. Negative fixtures retain proven receiver/member, argument, pointer/reference, global, and escaped mutations.

Fingerprint tests cover message changes, metadata changes, existing-baseline fallback matching, and wording-independent context/source identity. Diagnostic tests assert unresolved counts independently for Go and C++ so a reduction in findings cannot silently discard analysis coverage.

The local crumb-app checkout and its baseline are available. The final acceptance scan must:

- produce no more than the existing 19 unsuppressed findings;
- retain the verified 887 stale baseline entries;
- preserve genuine negative-fixture findings; and
- report unresolved-symbol counts by language.

## Performance and Non-Goals

Analysis is function-local plus package/file declaration indexing. Scope lookup and alias traversal are bounded; no whole-program points-to analysis is introduced. Parsed syntax and existing parser artifacts are reused where possible.

This repair does not add constructor or method allowlists, path exclusions, repository suppressions, permanent waivers, new structural rules, or a strict/debug configuration surface. It does not classify unknown symbols as local, safe, or global; it preserves them as unresolved evidence.
