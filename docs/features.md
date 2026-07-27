# Features

This page lists the current `codeguard` feature surface and the main config entrypoints for operators.

## Core scan families

- `quality`
  - maintainability thresholds
  - clone detection
  - language-native quality heuristics for Go, Python, TypeScript, JavaScript, Rust, Java, C++, C#, and Ruby
  - local-quality precision heuristics for naming, function shape, error handling, defensive programming, and maintainability deltas where the active build includes them
  - AI-quality heuristics such as swallowed errors, narrative comments, hallucinated imports, dead code, over-mocked tests, idiom drift, semantic review, provenance policy, and change-risk rollups
  - changed-line coverage gating in diff mode
  - opt-in `clang-format` and sanitized `clang++ -fsyntax-only` validation backed by safe `compile_commands.json` metadata
- `design`
  - layering and boundary rules
  - import cycle and god-module detection
  - reachability and stability policy warnings over language graphs
  - high-impact-change analysis and dependency graph artifacts
  - Go package-boundary checks plus declarations-per-file, methods-per-type, and interface-size heuristics
  - Python public/private and entrypoint coupling checks, import-cycle detection, generic module names, class-size heuristics, and protocol-size heuristics
  - TypeScript generic-module, class-size, and interface-size heuristics plus graph resolution through relative imports, `tsconfig` paths, package `imports`, and workspace package exports
  - Rust module graphs plus generic-module, methods-per-type, and trait-size heuristics
  - C++ target-local include and named-module graphs for cycles, reachability, stability, change impact, and boundary policy enforcement, plus generic filename, declarations-per-file, method-count, and contract-surface heuristics
- `security`
  - hardcoded secrets and private keys
  - Go, Python, TypeScript, and JavaScript taint-style flow checks
  - insecure API heuristics
  - C++ insecure TLS, shell execution, and unsafe C string API checks
  - C++ same-file taint-flow and SSRF analysis for common process and networking APIs
  - optional `govulncheck`
- `prompts`
  - prompt-asset governance
  - agent config and MCP config checks
  - dangerous instruction and standing-permission detection
- `ci`
  - workflow/release policy
  - test-quality heuristics, including GoogleTest, Catch2/doctest, and Boost.Test
- `supply_chain`
  - manifest normalization, including `vcpkg.json`, `conanfile.txt`, statically analyzed `conanfile.py`, and CMake dependency declarations
  - deterministic CycloneDX 1.6 JSON SBOM output from normalized dependency artifacts (`output.format: cyclonedx`)
  - lockfile presence and drift validation
  - unpinned dependency detection
  - dependency and manifest license policy
  - opt-in offline advisory-cache matching for concrete pinned dependency versions, with cache provenance and age in findings
  - Cargo manifest hygiene for missing package licenses and non-hermetic dependency sources
- `performance`
  - N+1 query patterns, allocation-heavy loops, blocking I/O in request paths, and unbounded concurrency
  - Go package and C++ include/module rebuild-cascade analysis for rebuild hot spots and amplifiers
  - Rust and C++ loop-smell coverage for regex construction, non-preallocated string growth, and polling sleeps
  - C++ loop-driven unbounded thread/task launch detection
  - build regression, benchmark regression, artifact-size budgets, and clang `-ftime-trace` budgets
- `reliability`
  - production-readiness checks for Go, Python, TypeScript, JavaScript, and C++
  - missing outbound timeouts, retry policy gaps, non-idempotent retries, cancellation gaps, unbounded work, resource cleanup, swallowed/lost errors, recoverable panics, and graceful-shutdown evidence
- `data`
  - distributed-system and data-correctness checks for Go, Python, TypeScript, JavaScript, and C++
  - read-modify-write race patterns, missing transaction boundaries, side effects in transactions, consumer idempotency/deduplication gaps, unsafe dual writes, missing outbox strategy, unstable pagination, unbounded reads, exactly-once assumptions, and cache policy gaps
- `observability`
  - production operability checks for Go, Python, TypeScript, JavaScript, and C++
  - unstructured logs, errors without operation/request context, sensitive log data, high-cardinality metric labels, missing critical-path instrumentation, log-and-ignore failures, and shallow health checks
- `operations`
  - ownership and runbook-readiness checks for critical production paths
  - enterprise profile coverage for service ownership and operational handoff metadata
- `delivery`
  - rollout-safety checks for workflows, deployment files, migrations, and high-risk source changes
  - missing rollback strategies, unsafe migration ordering, high-risk behavior without feature flags or kill switches, and missing post-deploy verification
- `change`
  - diff-mode change-safety, testability, and refactor-confidence checks for PR review
  - implemented signals for oversized and mixed-concern diffs, too many concerns, mixed refactor/behavior diffs, broad public-surface edits, one-use abstractions, duplicate helpers, cleanup regressions, complexity increases, moves without verification, behavior changes without tests, failure-path coverage gaps, legacy hotspots without characterization coverage, and hardwired or nondeterministic domain dependencies
  - implemented direct `refactor.*` IDs for behavior preservation checks, public-contract checks, error-path checks, side-effect ordering, visibility expansion, dependency direction, duplicate implementations left behind, and dead paths left behind
  - PR-summary signals for `change_safety`, `refactor_confidence`, and `maintainability_delta` when the change-summary postprocessor is available
- `contracts`
  - exported Go and public C++ API compatibility against a diff base
  - OpenAPI, protobuf, destructive migration checks, and non-expand/contract migration risk

## Agent-native features

- `codeguard serve --mcp`
- `codeguard validate-patch`
- `codeguard explain -format agent`
- verified auto-fix through SDK and CLI
- hook-pack examples for Claude Code and Cursor

## External report ingestion

CodeGuard can import findings from scanners that have already run. It does not
execute external tools. SARIF 2.x reports (including CodeQL), native Gitleaks
JSON, and native Trivy JSON are added as a normal `External Reports` section
with namespaced rule IDs such as `external.codeql.go-sql-injection`.

```yaml
external_reports:
  - path: .codeguard/reports/codeql.sarif
    format: sarif
    source: codeql
  - path: .codeguard/reports/gitleaks.json
    format: gitleaks
  - path: .codeguard/reports/trivy.json
    format: trivy
```

Report paths are constrained to the config directory when loaded from a config
file, must be regular non-symlink files, and are capped at 16 MiB. Artifact
paths embedded in SARIF are retained only when they are safe repository-relative
paths. The same path policy applies to native reports. Gitleaks and Trivy
secret payload fields are deliberately not decoded, stored, or emitted.
Imported reports are never passed to AI triage.

## AI-specific features

- `slop_score` artifact
- AI provenance policy
- hybrid AI triage
- semantic review:
  - `quality.ai.semantic-doc-mismatch`
  - `quality.ai.contract-drift`
  - `quality.ai.semantic-error-message`
  - `quality.ai.semantic-test-coverage`
  - `quality.ai.semantic-test-adequacy`
  - framework-aware request enrichment for changed Express handlers, middleware chains, React components, and Next.js route/component files so the semantic runtime sees higher-level contract context, not just raw file text
  - built-in reference semantic runner scaffold at `examples/semantic/reference_runner.py` for `CODEGUARD_SEMANTIC_COMMAND`
  - reference runner supports scaffold, local-command, and OpenAI-compatible backend modes without changing prompt assembly
- `quality.ai.change-risk`
  - aggregates AI-quality and review-risk signals into a target-level artifact plus a `Code Quality` finding when thresholds are crossed
- Diff-mode file risk and PR hotspots
  - emits `file_risk` and `pr_hotspots` artifacts that rank every changed file without changing finding severity
  - explains each score with stable, configurable contributions from findings, security and supply-chain signals, changed-line coverage, AI provenance, and slop-score artifacts where available
- Diff-mode production risk
  - emits `pr_summary.production_risk` when configured, using reliability, data-correctness, and non-expand/contract migration findings as deterministic PR-level risk evidence
  - does not change SARIF, GitHub annotations, or individual finding severity
- Diff-mode change safety
  - uses the `checks.change` family to report implemented change-safety, cleanup, testability, and safe-refactor findings
  - emits PR-summary fields such as `change_safety`, `refactor_confidence`, and `maintainability_delta` only as artifact evidence; they do not create extra annotations or change per-rule severities
  - local-quality precision and history-aware families such as `naming.*`, `function.*`, `error.*`, `defensive.*`, `maintainability.*`, and `smell.*` support the same review goal; use `codeguard rules` on the active build to see the exact rollout subset

## Parsers

- `parsers.treesitter: "off" | "auto"` (default `"off"`) selects the parsing
  substrate for TypeScript/TSX/JavaScript plus the migrated Python/C++ paths
  (`docs/treesitter-spike.md`).
  - `"off"`: the regex-based scanners run exactly as before.
  - `"auto"`: script files parse through embedded tree-sitter grammars; the
    migrated rules (`quality.typescript.explicit-any`,
    `quality.typescript.non-null-assertion`,
    `quality.typescript.double-assertion`,
    `security.typescript.unsafe-html-sink` and their `*.javascript.*`
    mirrors) evaluate grammar queries instead of regexes and report
    `confidence: high`. Files that exceed the 256 KiB parse cap, fail to
    parse, or produce error-heavy trees fall back to the regex path per file,
    so enabling the flag can never lose coverage.

## JSON/YAML config examples

### Tree-sitter parsing for TypeScript/JavaScript

YAML:

```yaml
parsers:
  treesitter: auto
```

JSON:

```json
{
  "parsers": {
    "treesitter": "auto"
  }
}
```

### Enable AI change risk

YAML:

```yaml
checks:
  quality: true
  quality_rules:
    ai_change_risk:
      enabled: true
      warn_threshold: 30
      fail_threshold: 60
```

JSON:

```json
{
  "checks": {
    "quality": true,
    "quality_rules": {
      "ai_change_risk": {
        "enabled": true,
        "warn_threshold": 30,
        "fail_threshold": 60
      }
    }
  }
}
```

### AI provenance policy

YAML:

```yaml
checks:
  quality: true
  quality_rules:
    ai_provenance:
      enabled: true
      env_vars:
        - CODEGUARD_AI_ASSISTED
      commit_trailers:
        - AI-Assisted
        - AI-Generated
      slop_score_warn_threshold: 20
      slop_score_fail_threshold: 40
```

JSON:

```json
{
  "checks": {
    "quality": true,
    "quality_rules": {
      "ai_provenance": {
        "enabled": true,
        "env_vars": ["CODEGUARD_AI_ASSISTED"],
        "commit_trailers": ["AI-Assisted", "AI-Generated"],
        "slop_score_warn_threshold": 20,
        "slop_score_fail_threshold": 40
      }
    }
  }
}
```

### Supply-chain license commands

YAML:

```yaml
checks:
  supply_chain: true
  supply_chain_rules:
    denied_licenses:
      - GPL-3.0
    license_commands:
      npm:
        name: npm-license-resolver
        command: ./scripts/resolve-npm-licenses.sh
```

JSON:

```json
{
  "checks": {
    "supply_chain": true,
    "supply_chain_rules": {
      "denied_licenses": ["GPL-3.0"],
      "license_commands": {
        "npm": {
          "name": "npm-license-resolver",
          "command": "./scripts/resolve-npm-licenses.sh"
        }
      }
    }
  }
}
```

### Enable change safety in diff scans

YAML:

```yaml
checks:
  change: true
  change_rules:
    max_changed_files: 25
    max_changed_directories: 8
    max_changed_lines: 800
    max_public_interfaces_changed: 3
    max_concern_families: 3
    min_test_to_production_ratio_percent: 20
```

JSON:

```json
{
  "checks": {
    "change": true,
    "change_rules": {
      "max_changed_files": 25,
      "max_changed_directories": 8,
      "max_changed_lines": 800,
      "max_public_interfaces_changed": 3,
      "max_concern_families": 3,
      "min_test_to_production_ratio_percent": 20
    }
  }
}
```

## Next queued AI features

These are the tracks currently being planned for follow-up implementation:

- batch verified fix planning
- `agent.permission-risk`
