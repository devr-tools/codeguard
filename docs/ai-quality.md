# AI-Generated Code Quality

This brief tracks the AI-generated-code quality features currently implemented in `codeguard`.

## Implemented

- AI-failure-mode rule pack
  - `quality.ai.swallowed-error`
  - `quality.ai.narrative-comment`
  - `quality.ai.hallucinated-import`
  - `quality.ai.dead-code`
  - `quality.ai.over-mocked-test`
- Slop score
  - `slop_score` report artifact with weighted AI-signal components for CI trend reporting
- Provenance-aware policy
  - `quality.ai.provenance-policy`
  - configurable through `checks.quality_rules.ai_provenance`
  - supports `CODEGUARD_AI_ASSISTED`-style environment hints and commit trailers such as `AI-Assisted: true`
- Consistency-with-codebase checks
  - `quality.ai.local-idiom-drift`
  - currently compares test framework choices against the dominant local repository idiom for Go and TypeScript/JavaScript targets
- Optional semantic review for AI-assisted diffs
  - `quality.ai.semantic-doc-mismatch`
  - `quality.ai.semantic-error-message`
  - `quality.ai.semantic-test-coverage`
  - runs for changed files from patch/diff input, or from a git diff against the scan base ref during full scans
  - requires the semantic runtime to be explicitly enabled either through `ai.enabled` / `--ai` with a command-backed provider, or through `CODEGUARD_SEMANTIC_CHECKS=1`
  - shells out to the command in `CODEGUARD_SEMANTIC_COMMAND`, sends a bounded JSON payload on stdin, and expects JSON verdicts on stdout
  - caches verdicts by request content hash in a sibling cache file next to the normal scan cache
- Hybrid AI triage for static findings
  - optional provider-backed pass that tries to verify or dismiss existing findings conservatively
  - stays fully offline when `CODEGUARD_AI_TRIAGE_PROVIDER` is unset
  - caches provider verdicts by packaged finding content hash inside the normal scan cache
- Verified auto-fix
  - `codeguard.VerifyFix(...)` and `codeguard.GenerateVerifiedFix(ctx, req)` only return patches after diff-scoped verification and inferred or explicit verification tests pass in an isolated workspace
  - `codeguard fix -ai` exposes the same verified-fix flow from the CLI for one selected finding
- Natural-language custom rules
  - custom rule packs can use `natural_language` instructions alongside regex and path matchers
  - evaluation is command-driven through the optional AI runtime and produces normal custom-rule findings

## Hybrid triage runtime

Hybrid triage is environment-driven so it does not add new CLI flags or shared config schema.

- `CODEGUARD_AI_TRIAGE_PROVIDER=openai`
- `CODEGUARD_AI_TRIAGE_MODEL=<model-name>`
- `CODEGUARD_AI_TRIAGE_BASE_URL=<optional OpenAI-compatible base URL>`
- `CODEGUARD_AI_TRIAGE_API_KEY=<optional bearer token>`
- `CODEGUARD_AI_TRIAGE_TIMEOUT=20s`

When enabled, `codeguard` packages each active finding with rule metadata and a local source excerpt, asks the provider to return `keep` or `dismiss`, and emits an `ai_analysis` artifact in `triage` mode with the resulting verdicts.

## Current scope

- Hallucinated import detection is local-manifest based:
  - Go imports are checked against `go.mod` and the local module path
  - TypeScript and JavaScript imports are checked against `package.json`, workspace package names, built-in Node modules, and local relative files
- Over-mocked test detection is heuristic:
  - warns when mock setup strongly outweighs behavior assertions
- Dead-code detection is heuristic:
  - currently focuses on obvious constant-condition branches such as `if false` and `if (false)`
- Semantic review is opt-in:
  - can be enabled through the normal AI runtime or through `CODEGUARD_SEMANTIC_CHECKS=1`
  - scopes itself to changed files from diff or patch input, or from a git diff against the configured base ref during full scans, plus a small set of nearby test files
  - `ai.semantic.function_contract`, `ai.semantic.misleading_error_messages`, and `ai.semantic.test_behavior_coverage` control which semantic prompts are sent
  - the external semantic command must read a JSON request from stdin and return `{"verdicts":[...]}` with `rule_id`, `path`, `line`, `level`, and `message`
- Verified auto-fix is fail-closed:
  - fix generation requires an explicit AI provider plus `-ai`
  - the proposed diff must apply cleanly, pass a diff-scoped `codeguard` rerun, and pass inferred or explicit verification tests
  - inferred verification currently covers:
    - nearest Go package tests
    - nearest Python `unittest` files via `python3 -m unittest <test-file>`
    - nearest runnable Node test files via `node --test`
    - JavaScript/TypeScript package-manager `test` scripts from `package.json` as a conservative fallback
- Natural-language rules are opt-in:
  - set `rule_packs[].rules[].natural_language`
  - provide an AI runtime through `ai.provider`, typically the `command` provider for local or BYO model execution

## Follow-on opportunities

- add deeper TypeScript-aware nearest-test inference beyond generic package-script fallback
- add Python import-resolution support against lockfiles and environments
- expand idiom drift beyond test frameworks into error handling and naming style
- add PR-level provenance adapters for hosted review systems
