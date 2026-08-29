# Ownership-aware structural rules

## Scope

This repair improves four existing structural rules without adding exclusions,
waivers, strict profiles, or whole-program analysis.

## Effect evidence

A bounded intraprocedural pass classifies function parameters, receivers,
package/global names, local allocations, and simple aliases. It records direct
field/index assignments and recognized mutating calls, then propagates origin
through straightforward assignments and address/reference aliases. Local values
become escaped only when stored into caller-owned/shared state before later
mutation. Returning a freshly constructed value is still local construction.

Every reportable effect carries:

- `mutation_target`: `argument`, `receiver`, `global`, or `escaped`;
- `effect_kind`: `persistence`, `network`, `event`, or `shared_state`;
- `origin`: `caller_owned`, `shared`, or `unknown`.

Local construction evidence may use `local`, `construction`, and
`locally_allocated` internally but does not trigger default structural rules.

## Rule behavior

`function.hidden-mutation` reports receiver, argument/alias, global, and escaped
mutation. Locally allocated maps, slices, DTOs, protobufs, and builders returned
from the function remain construction.

`function.command-query-mix` reports a value-returning query only when effect
evidence is externally observable: persistence writes, network writes, event
publication, persistent cache mutation, or caller/shared mutation. Repository
reads, cache reads, row scanning/hydration, serialization, metrics observation,
protobuf setters, and response construction remain queries.

`smell.message-chain` requires traversal through several independently owned
collaborators. Repeated calls on one local builder and fluent/generated,
optional/result, JSON, or SQL-hydration chains are exempt.

`function.inconsistent-return-contract` compares equivalent outcomes while
recognizing deliberate pointer/null/collection optionality, `sql.Null*`,
option/result types, and `(value, found, error)` contracts.

## Bounds

Analysis is function-local, linear in parsed assignments/calls/statements, and
uses only direct alias propagation. Unknown calls do not become high-confidence
mutation evidence without an owned target or recognized observable effect.
