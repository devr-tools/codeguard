# Features

This page lists the current `codeguard` feature surface and the main config entrypoints for operators.

## Core scan families

- `quality`
  - maintainability thresholds, including Go cyclomatic complexity (`quality_rules.cyclomatic_counting`: `legacy` by default in 1.x, `gocyclo` to match gocyclo)
  - clone detection
  - language-native quality heuristics for Go, Python, TypeScript, JavaScript, Rust, Java, C++, C#, and Ruby
  - local-quality precision heuristics for naming, function shape, error handling, defensive programming, and maintainability deltas where the active build includes them
  - structural smell heuristics such as god object, feature envy, middle man, message chains, data clumps, and switch-on-type
  - AI-quality heuristics such as swallowed errors, narrative comments, hallucinated imports, dead code, over-mocked tests, idiom drift, semantic review, provenance policy, and change-risk rollups
  - opt-in toolchain dead-code evidence for Go, Rust, C++, Python, TypeScript, and JavaScript through compiler diagnostics, linker/symbol artifacts, static import graphs, and trusted bundler reports
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
  - agent instruction files, Claude Code settings (`.claude/settings*.json`), and MCP config checks, parsed structurally
  - dangerous instruction and standing-permission detection
  - MCP server risk: shell-wrapped commands, unpinned registry launches, plaintext remote transport, unauthenticated remote servers, and literal credentials in `env`/`headers`
  - hidden Unicode characters in governed files
- `ci`
  - workflow/release policy (opt-in: nothing is required until configured)
  - GitHub Actions workflow security, with no configuration: script injection from untrusted `${{ }}` contexts, dangerous `pull_request_target`/`workflow_run` checkouts, `write-all` or missing `permissions`, `curl | sh` in `run:`, and secrets in fork-triggered jobs
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

- `codeguard serve --mcp` (stdio or `--http`); `scan` and `validate_patch` return the full 1.10 report by default, and `format: "summary"` or any filter input returns a filtered, paginated summary instead
- `codeguard validate-patch`
- `codeguard explain -format agent`
- `codeguard context init|update` generates and refreshes `AGENTS.md`/`CLAUDE.md` from detected facts
- verified auto-fix through SDK and CLI, and `codeguard fix-batch` for verifying several deterministic fixes as one patch
- hook-pack examples for Claude Code, Cursor, and Devin

See [Agent-native features](agent-native.md) for details.

## CLI and CI gating

- `codeguard init` detects languages, entrypoints, workflows, and manifests and writes a small config; `codeguard scan` with no config uses the same repository-aware defaults. See [Getting started](getting-started.md#quick-start).
- `-fail-on fail|warn|none` (`output.fail_on`), `-min-confidence low|medium|high`, and `-quiet` control the exit policy, the confidence floor, and progress output. Operational failures exit 1 under every policy.
- `codeguard rules -section|-search|-level|-enabled` filters the catalog; `codeguard explain` shows examples, config keys with their resolved values, suppression syntax, and profiles.
- Mistyped flags, rule ids, profiles, formats, and `-set` keys fail before the scan with a "did you mean" suggestion.
- SARIF output carries `security-severity`, CWE tags, rule help, taint code flows, and `partialFingerprints` for GitHub code scanning. See [Checks → SARIF output](checks.md#sarif-output-github-code-scanning).

## Baseline governance

- `codeguard baseline` creates a new accepted-debt snapshot.
- `codeguard baseline audit` classifies existing entries as active through an
  exact, context, or path-insensitive content fingerprint, or as stale/invalid.
- `codeguard baseline prune -check` provides a non-mutating CI gate;
  `-write` atomically removes only stale entries, and `-output` creates a
  reviewable candidate file.
- `codeguard baseline policy -compare-baseline <path>` rejects configured net
  growth, maximum-entry violations, and newly baselined prohibited rule
  families while allowing existing approved debt.
- Audit reports group active debt by rule, ownership area, and risk family and
  provide deterministic evidence samples with confidence/language counts.
- Context/content collisions from identical snippets are reported and
  preserved. Governance never forces one-to-one matching or changes v1.7.3
  suppression semantics.
- `codeguard scan -include-suppressed -format json` emits individual baseline,
  waiver, and inline suppression records when explicitly requested; default
  reports remain unchanged.

See [Production rollout](production.md) for configuration, safe CI usage,
review workflow, and exit-code behavior.

## Repository traversal

The file walk honours `.gitignore` files and `.git/info/exclude` by default
(`respect_gitignore: false` turns this off; tracked files are always scanned).
See [Checks → `.gitignore`](checks.md#gitignore).

Full scans always prune directories named `node_modules` and `cdk.out` at any
depth. Dependency manifests and lockfiles remain in scope for supply-chain,
license, vulnerability, and lockfile-integrity checks; installed and generated
source is not treated as first-party code.

Directories named `vendor` are also pruned by default. Repositories that commit
or patch vendored source can opt in explicitly while retaining the normal
per-file, corpus-byte, AST, and file-count limits:

```yaml
scan_vendored_source: true
```

Configured `exclude` patterns still take precedence, so an explicitly excluded
vendor subtree remains excluded when vendored-source scanning is enabled.

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
  - the local-quality precision rollout supports the same review goal through the exact `naming.*`, `function.*`, `error.*`, `defensive.*`, selected `maintainability.*`, and structural/history-aware `smell.*` IDs listed by `codeguard rules` on the active build
  - structural smell rules such as `smell.refused-bequest` are reported as change-quality evidence: they explain when a design shape makes future changes less safe or harder to isolate, not just whether an individual line is syntactically suspicious

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

### Config provenance

Every config codeguard writes (`codeguard init`, `WriteConfigFile`) records the
release that produced it as a top-level `codeguard_version`. Loading reports
what the file says rather than the running binary, so a config written by a
different release is visible on inspection.

The stamp is provenance only: it is never validated, so a config written by an
older or newer release always loads, and it is excluded from the per-file cache
fingerprints, so a release upgrade does not discard cached findings.

```yaml
codeguard_version: v1.11.0
name: my-repo
```

### Minimum confidence policy

Drop findings the scanner is not confident enough about, globally or per
section, and optionally report low-confidence failures as warnings. Omitting
the block admits every finding, which is the default behavior. Section keys are
the ids reported in scan output, such as `security`, `supply_chain`, or `custom`.

Both settings are applied when a section is finalized, after the per-file
findings cache, so changing a threshold re-renders a scan rather than
re-running one, and neither changes a finding's fingerprint or its baseline
match. Every removed finding is counted per rule as `confidence_filtered` and
per section as `confidence_filtered_count`.

YAML:

```yaml
checks:
  min_confidence:
    default: medium
    sections:
      security: high
      quality: low
  confidence_demotion: true
```

JSON:

```json
{
  "checks": {
    "min_confidence": {
      "default": "medium",
      "sections": {
        "security": "high",
        "quality": "low"
      }
    },
    "confidence_demotion": true
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
