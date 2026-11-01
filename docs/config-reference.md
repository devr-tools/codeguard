# Configuration key reference

This page documents configuration keys that the section prose in
[checks.md](checks.md) does not cover, grouped by the section that owns them.
Each table gives the key's path, its default after `ApplyDefaults` runs (the
base profile, before any `profile` adjustments), and what it controls.

- Paths are relative to the config root. For example
  `checks.quality_rules.risk_scoring.security_weight` is `security_weight`
  under `risk_scoring` under `quality_rules` under `checks`.
- YAML and JSON use the same key names.
- *Reserved* keys are parsed and validated but no check reads them yet.
  Setting them is harmless and has no effect today.
- `tests/rules/docs_coverage_test.go` walks the config structs' `yaml` tags and
  fails when a key appears nowhere under `docs/`. Document new keys here, or in
  the owning section of `checks.md`.

## Quality

### `checks.quality_rules.risk_scoring`

Diff-mode file-risk ranking. It produces the `file_risk` and `pr_hotspots`
artifacts. Each changed file collects weighted components, and the file's
score is their sum, capped at 100. These keys do not change any finding's
level.

| Key | Default | Meaning |
| --- | --- | --- |
| `enabled` | `true` | Builds the risk artifacts on diff scans. Full scans never build them. |
| `max_hotspots` | `5` | How many top-ranked files `pr_hotspots` lists. |
| `changed_file_weight` | `5` | Base weight every changed file receives. |
| `fail_finding_weight` | `30` | Added per fail-level finding on the file. |
| `warn_finding_weight` | `15` | Added per warn-level finding on the file. |
| `security_weight` | `10` | Added per `security.*` finding, on top of the level weight. |
| `supply_chain_weight` | `10` | Added per `supply_chain.*` finding, on top of the level weight. |
| `coverage_gap_weight` | `15` | Added when `quality.coverage-delta` reports the file below the changed-line coverage policy. |
| `ai_provenance_weight` | `15` | Added to every file in a target whose change is tagged AI-assisted (see `quality_rules.ai_provenance`). |
| `ai_signal_weight` | `10` | Added per `quality.ai.*` finding other than `quality.ai.provenance-policy`. |
| `slop_score_divisor` | `10` | The target's `slop_score` divided by this value (integer division) becomes a `slop_score` component. |

### `checks.quality_rules.ai_checks`

Toggles for the AI-failure-mode pack. An unset toggle means enabled.

| Key | Default | Meaning |
| --- | --- | --- |
| `hallucinated_import` | enabled | Emits `quality.ai.hallucinated-import` for imports that resolve to no manifest dependency, standard library package, or local file. |
| `error_style_drift` | enabled | Emits `quality.ai.error-style-drift` when a file's error handling diverges from the repository's dominant convention. |
| `naming_drift` | enabled | Emits `quality.ai.naming-drift` when a file's identifier casing diverges from the repository's dominant convention. |
| `slop_history_limit` | `0` (keep 100) | Number of `slop_score` history entries kept per target in the cache directory. `0` or unset keeps 100. |

### `checks.quality_rules.naming`

| Key | Default | Meaning |
| --- | --- | --- |
| `allowed_abbreviations` | `api`, `cli`, `cpp`, `db`, `html`, `http`, `https`, `id`, `io`, `ip`, `js`, `json`, `orm`, `rpc`, `sdk`, `sql`, `tcp`, `ts`, `ui`, `url`, `uuid`, `xml`, `yaml` | Extra abbreviations `naming.unknown-abbreviation` accepts. They add to a built-in list (it includes `ast`, `aws`, `grpc`, `tls` and others) rather than replacing it. |
| `role_suffix_warn_threshold` | `4` | `naming.role-suffix-overuse` fires when a file has at least this many distinct identifiers ending in Manager, Helper, Util, Utils, Service, or Processor. `0` or a negative value falls back to 4. |

### Other quality keys

| Key | Default | Meaning |
| --- | --- | --- |
| `checks.quality_rules.dead_code.include_tests` | `false` | Includes test files and test-only targets in toolchain dead-code analysis (`quality.dead-code.toolchain`) for Python, Rust, and C++. When off, symbols that only tests use are not reported, and test code is not a dead-code candidate. |
| `checks.quality_rules.maintainability_history` | see below | Groups the maintainability-history settings. |
| `checks.quality_rules.maintainability_history.report_as_findings` | `false` | Emits the history-derived maintainability results as findings. When off, they are published only as a report artifact. |
| `checks.quality_rules.cpp_tooling.clang_format_command` | `clang-format` | Executable that `quality.cpp.clang-format` runs. A non-default value is a config-supplied command, so it is subject to the [trust gate](security.md). |
| `checks.quality_rules.cpp_tooling.compiler_command` | `clang++` | Compiler that `quality.cpp.compiler-parse` runs with `-fsyntax-only`. It is subject to the same trust gate. |

## Design

All under `checks.design_rules`.

| Key | Default | Meaning |
| --- | --- | --- |
| `detect_import_cycles` | `true` | Enables the per-language `design.*.import-cycle` rules. They run on the module dependency graph. |
| `detect_god_modules` | `true` | Enables `design.god-module`. |
| `god_module_threshold` | `25` | `design.god-module` fires when a module's fan-in plus fan-out exceeds this value. |
| `detect_high_impact_changes` | `true` | Enables `design.high-impact-change`. It runs on diff scans only. |
| `high_impact_change_threshold` | `10` | `design.high-impact-change` fires when a changed module has more transitive dependents than this value. |
| `language_diff_commands` | none | Per-language commands, shaped like `language_commands`, that run only in diff mode. A non-zero exit emits `design.diff-command-check` at fail level. Each command receives `CODEGUARD_DIFF_BASE_DIR`, `CODEGUARD_DIFF_HEAD_DIR`, `CODEGUARD_DIFF_TARGET_DIR`, and `CODEGUARD_DIFF_BASE_REF`, so a contract-diff tool can compare the base and head trees. Commands are subject to the trust gate. |

## Security

All under `checks.security_rules`.

| Key | Default | Meaning |
| --- | --- | --- |
| `taint_go` | `true` | Enables source-to-sink taint analysis for Go files. |
| `taint_python` | `true` | Enables source-to-sink taint analysis for Python files. |
| `typescript_taint_max_depth` | `8` | Maximum call depth the TypeScript/JavaScript semantic engine follows when it traces taint across functions and modules (`security.typescript.taint-flow`, `security.javascript.taint-flow`). |

## Supply chain

All under `checks.supply_chain_rules`.

| Key | Default | Meaning |
| --- | --- | --- |
| `detect_provenance` | `true` | Enables `supply_chain.missing-provenance`. It fires when build or publish automation exists without provenance, attestation, SBOM, or signing evidence. |
| `allowed_licenses` | empty (no allowlist) | When non-empty, `supply_chain.denied-license` also reports manifest and dependency licenses that are not on this list. A license with definitive evidence fails; one inferred without definitive evidence warns. `denied_licenses` is checked first. |

## Reliability and data correctness

| Key | Default | Meaning |
| --- | --- | --- |
| `checks.reliability_rules.max_inline_goroutines_per_function` | `4` | `reliability.missing-concurrency-limit` fires when one Go function starts more than this many inline goroutines, or one TypeScript or JavaScript file starts more than this many detached async operations, with no concurrency limiter in sight. |
| `checks.reliability_rules.max_retry_attempts` | `3` | Reserved. It is validated as non-negative, but the retry rules use their own evidence today. |
| `checks.data_rules.max_writes_without_transaction` | `1` | `data.missing-transaction-boundary` fires when the scanned code performs more than this many persistence writes without transaction evidence. |
| `checks.data_rules.max_unbounded_read_rows` | `1000` | Reserved. It is validated as non-negative, but `data.unbounded-read` does not compare against it yet. |

## Observability

All under `checks.observability_rules`. Patterns are case-insensitive
substrings of the source line or path.

| Key | Default | Meaning |
| --- | --- | --- |
| `sensitive_name_patterns` | `password`, `passwd`, `secret`, `token`, `api_key`, `apikey`, `authorization`, `cookie`, `ssn`, `email` | Names that make a log call `observability.sensitive-log-data`. |
| `high_cardinality_label_patterns` | `user_id`, `userid`, `email`, `request_id`, `requestid`, `trace_id`, `session_id`, `uuid`, `path`, `url` | Metric label names that trigger `observability.high-cardinality-label`. |
| `critical_path_patterns` | `handler`, `controller`, `consumer`, `job`, `worker`, `payment`, `checkout`, `auth`, `migration` | Paths treated as critical for `observability.critical-path-uninstrumented`. |
| `healthcheck_path_patterns` | `health`, `healthz`, `ready`, `readyz`, `live`, `livez` | Endpoint names that `observability.shallow-health-check` inspects. |
| `instrumentation_evidence_patterns` | `span`, `trace`, `metric`, `counter`, `histogram`, `observe`, `instrument`, `prometheus` | Tokens that count as instrumentation evidence on a critical path. |
| `structured_logger_patterns` | `logger.`, `logrus.`, `zap.`, `slog.`, `zerolog.`, `structlog.`, `logging.` | Reserved. `observability.unstructured-log` uses its own built-in logger list today. |

## Operations

All under `checks.operations_rules`.

| Key | Default | Meaning |
| --- | --- | --- |
| `owner_file_patterns` | `CODEOWNERS`, `.github/CODEOWNERS`, `OWNERS`, `owners.yaml`, `catalog-info.yaml`, `service.yaml`, `service.yml` | Files that satisfy `operations.missing-owner`. |
| `runbook_path_patterns` | `runbook`, `runbooks`, `docs/runbooks`, `ops`, `operations` | Paths that satisfy `operations.missing-runbook`. |
| `critical_path_patterns` | `cmd/`, `internal/`, `service`, `api`, `worker`, `consumer`, `job`, `payment`, `auth`, `deploy`, `migrations` | The operations rules run only when a target has a file on one of these paths. Observability has a separate list with the same key name. |

## Delivery

All under `checks.delivery_rules`.

| Key | Default | Meaning |
| --- | --- | --- |
| `rollback_evidence_patterns` | `rollback`, `roll back`, `revert`, `restore`, `previous version` | Text that satisfies `delivery.missing-rollback-strategy`. |
| `kill_switch_patterns` | `feature flag`, `feature_flag`, `kill switch`, `killswitch`, `rollout`, `launchdarkly` | Text that satisfies `delivery.high-risk-change-without-kill-switch`. |
| `post_deploy_verification_patterns` | `smoke`, `health`, `synthetic`, `post-deploy`, `post deploy`, `curl`, `slo` | Text in a deployment workflow that satisfies `delivery.missing-post-deploy-verification`. |
| `migration_path_patterns` | `migrations/**`, `db/migrate/**`, `alembic/**` | Globs for migration files that `delivery.unsafe-migration-order` inspects. They add to the built-in `migrations/`, `db/migrate/`, and `alembic/` fragments. |
| `high_risk_path_patterns` | `**/payment/**`, `**/payments/**`, `**/auth/**`, `**/checkout/**`, `**/billing/**`, `**/migration/**`, `**/migrations/**` | Globs where high-risk behavior in a source file needs kill-switch evidence (`delivery.high-risk-change-without-kill-switch`). |
| `bootstrap_path_patterns` | `cmd/**`, `config/**`, `configs/**`, `**/config/**`, `**/bootstrap/**`, `scripts/**`, `.github/**` | Globs treated as bootstrap or config code. `delivery.high-risk-change-without-kill-switch` skips them, and `quality.environment-branching` allows environment branching there. |

## API contracts

| Key | Default | Meaning |
| --- | --- | --- |
| `checks.contract_rules.migration_paths` | `migrations/`, `db/migrate/`, `alembic/` | Directory fragments that mark a file as a migration for `contracts.migration-destructive` and `contracts.non-expand-contract-migration`. Every `.sql` file counts as a migration too. |

## PR summary production risk

All under `checks.production_risk`. They add to the `enabled`,
`warn_threshold`, and `fail_threshold` keys shown in
[checks.md](checks.md#pr-summary-production-risk).

| Key | Default | Meaning |
| --- | --- | --- |
| `reliability_weight` | `12` | Weight for each `reliability.*` finding. |
| `data_weight` | `15` | Weight for each `data.*` finding and for `contracts.non-expand-contract-migration`. |
| `fail_weight` | `25` | Added when the contributing finding is fail-level. |
| `warn_weight` | `10` | Added when the contributing finding is warn-level. |

## AI

| Key | Default | Meaning |
| --- | --- | --- |
| `ai.hybrid_triage` | see below | Groups the settings for LLM triage of deterministic findings. `ai.hybrid_triage.enabled` (default `true`, and it needs `ai.enabled`) turns triage on. |
| `ai.hybrid_triage.suppress_dismissed` | `true` | Reserved. Validated, but the triage runtime does not read it yet. |
| `ai.hybrid_triage.candidate_sections` | `Code Quality`, `Design Patterns`, `Security`, `Custom Rules` | Reserved. These are the sections intended for triage candidates. |
| `ai.hybrid_triage.candidate_severities` | `warn`, `fail` | Reserved. These are the levels intended for triage candidates. |
| `ai.autofix.verify_tests` | `true` | Reserved. `codeguard fix` verification runs `ai.autofix.test_commands` whenever they are configured. |
| `ai.autofix.max_fixes` | `5` | Reserved. Validated as non-negative. |

## Custom rule packs

Keys on an entry in `rule_packs[].rules[]`, alongside `paths`,
`content_regex`, and the others shown in
[checks.md](checks.md#custom-rule-packs).

| Key | Default | Meaning |
| --- | --- | --- |
| `path_regex` | none | Go regular expression matched against the target-relative path. The rule applies only to files that match. An invalid expression fails validation. |
| `ai_prompt` | none | Reserved. Accepted for forward compatibility, but no runtime reads it. Use `natural_language` for LLM-evaluated rules. |

## Baseline governance

All under `baseline.governance`. They are used by
`codeguard baseline policy` and `codeguard baseline audit` (see
[features.md](features.md#baseline-governance)).

| Key | Default | Meaning |
| --- | --- | --- |
| `max_entries` | `0` (no limit) | `baseline policy` reports a `max_entries` violation when the baseline has more entries than this. |
| `forbid_growth` | `false` | `baseline policy` reports a `growth` violation when the baseline has more entries than the `-compare-baseline` file. |
| `prohibited_new_rule_prefixes` | none | Rule-id prefixes, such as `security.`, that may not appear in newly added baseline entries. |
| `require_no_stale_entries` | `false` | Reserved. Use `codeguard baseline prune -check` to gate on stale entries. |
| `ownership[].owner` | none | Owner label for baseline entries whose path matches the sibling `pattern`. `baseline audit` groups active debt by it. |
| `sample_limit` | `0` (3 samples) | Evidence samples per group in `baseline audit` output. `0` or unset means 3. |
