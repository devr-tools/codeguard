# Workspace govulncheck and credential fixture evidence

## Scope

This repair makes module-dependent vulnerability scanning workspace-aware and
replaces path-only credential fixture demotion with evidence-based
classification. It adds no repository exclusions, waivers, or unrelated rules.

## Govulncheck

For a target containing `go.work`, CodeGuard parses active `use` modules and
workspace `replace` directives, validates each module through its `go.mod`, and
runs govulncheck in each module directory. Without `go.work`, the nearest
applicable `go.mod` defines a single module; a repository root without either is
never used as a module working directory.

Module scans use bounded concurrency and independent timeouts. Their typed
results record success, failure, timeout, and skip status. Successful partial
results survive failures. Vulnerabilities retain advisory, module, package, and
call-stack evidence; advisory-level presentation is deduplicated across modules
without discarding those affected paths.

Only parsed vulnerabilities become `security.govulncheck` findings. Missing
packages/modules, timeouts, and invocation failures become operational
diagnostics. Required-mode operational failures make scan health nonzero, but
diagnostics are never eligible for a suppression baseline.

## Credential classification

Secret detection first extracts candidates, including adjacent or simply
concatenated string literals. Working-tree candidates are classified using
combined path, symbol, syntax, entropy, provider shape, host/account context,
and cross-file reuse evidence. Raw values remain transient and never enter
reports.

Provider-shaped credentials, private/signing material, meaningful JWTs,
realistic high-entropy values, production-host associations, and reuse outside
test/dev scope remain security findings regardless of their path. Clear fixture
paths plus synthetic symbols, reserved example domains, obvious dummy content,
or explicit fixture construction can establish a likely synthetic fixture only
when stronger credential evidence is absent.

Classifications are:

- confirmed: ordinary security finding;
- ambiguous fixture: low-confidence security finding requiring review;
- likely synthetic fixture: informational diagnostic, excluded from baselines.

Each result exposes non-sensitive evidence codes in JSON and SARIF. Git-history
scanning uses strict candidate detection directly and does not apply working-tree
fixture classification.

## Reporting invariants

Diagnostics are structurally separate from findings. They can affect section
and process health, but baseline generation and matching only consume findings.
SARIF results identify diagnostics and include evidence/status properties.
