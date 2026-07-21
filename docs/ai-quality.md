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
- Change-risk rollup
  - `quality.ai.change-risk`
  - `change_risk` report artifact
  - configurable through `checks.quality_rules.ai_change_risk`
  - aggregates slop-score signals, semantic findings, coverage gaps, diff breadth, and AI provenance into a review-priority score
- Consistency-with-codebase checks
  - `quality.ai.local-idiom-drift`
  - currently compares test framework choices against the dominant local repository idiom for Go and TypeScript/JavaScript targets
- Optional semantic review for AI-assisted diffs
  - `quality.ai.semantic-doc-mismatch`
  - `quality.ai.contract-drift`
  - `quality.ai.semantic-error-message`
  - `quality.ai.semantic-test-coverage`
  - `quality.ai.semantic-test-adequacy`
  - `quality.ai.semantic-runtime`
  - request enrichment now adds framework metadata and contract hints for changed Express handlers and middleware, React components, and Next.js route/component files, so semantic runtimes can reason about handlers, props, route segments, request/response semantics, and middleware ordering with better local context
  - runs for changed files from patch/diff input, or from a git diff against the scan base ref during full scans
  - requires the semantic runtime to be explicitly enabled either through `ai.enabled` / `--ai` with a command-backed provider, or through `CODEGUARD_SEMANTIC_CHECKS=1`
  - uses the command from `ai.provider.type=command` plus `ai.provider.command`/`args` when configured; otherwise it falls back to `CODEGUARD_SEMANTIC_COMMAND`
  - sends a bounded JSON payload on stdin and expects JSON verdicts on stdout
  - emits `quality.ai.semantic-runtime` at `fail` level when semantic review is enabled but no command is configured, or when the command crashes or returns invalid JSON
  - caches verdicts by request content hash in a sibling cache file next to the normal scan cache
- Hybrid AI triage for static findings
  - optional provider-backed pass that tries to verify or dismiss existing findings conservatively
  - supports OpenAI-compatible endpoints (`openai`) and the native Anthropic Messages API (`anthropic`)
  - stays fully offline when `CODEGUARD_AI_TRIAGE_PROVIDER` is unset
  - caches provider verdicts by packaged finding content hash inside the normal scan cache
  - retries 429/5xx/network failures with exponential backoff plus jitter, honors `Retry-After`, and degrades gracefully (the scan completes with findings kept and an error verdict recorded)
- Verified auto-fix
  - `codeguard.VerifyFix(...)` and `codeguard.GenerateVerifiedFix(ctx, req)` only return patches after diff-scoped verification and inferred or explicit verification tests pass in an isolated workspace
  - `codeguard fix -ai` exposes the same verified-fix flow from the CLI for one selected finding
  - `codeguard fix-batch -input fixes.json` verifies explicitly supplied, catalogued deterministic fixes together in one isolated workspace and returns only their aggregate patch. It never modifies the working tree. The input is a JSON object with an `items` array of `{ "finding": { ... }, "candidate": { "diff": "..." } }` entries; use `-format json` to retain included, skipped, and failed item details.
- Natural-language custom rules
  - custom rule packs can use `natural_language` instructions alongside regex and path matchers
  - evaluation is command-driven through the optional AI runtime and produces normal custom-rule findings
  - per-verdict caching: each evaluation is cached in the scan cache under a SHA1 of the rule fingerprint, runtime fingerprint, file path, file content hash, and prompt version, so an unchanged file plus an unchanged rule never re-invokes the runtime
- Agent-ready fix templates
  - curated rules carry a `fix_template` with a short imperative fix plus a before/after snippet
  - exposed through `codeguard explain <rule> -format agent` and the MCP `explain` tool

## Hybrid triage runtime

Hybrid triage is environment-driven so it does not add new CLI flags or shared config schema.

- `CODEGUARD_AI_TRIAGE_PROVIDER=openai|anthropic`
- `CODEGUARD_AI_TRIAGE_MODEL=<model-name>` (defaults to `claude-sonnet-4-6` for `anthropic`)
- `CODEGUARD_AI_TRIAGE_BASE_URL=<optional provider base URL>`
- `CODEGUARD_AI_TRIAGE_API_KEY=<optional credential>` (for `anthropic`, falls back to `ANTHROPIC_API_KEY`)
- `CODEGUARD_AI_TRIAGE_TIMEOUT=20s`

When enabled, `codeguard` packages each active finding with rule metadata and a local source excerpt, asks the provider to return `keep` or `dismiss`, and emits an `ai_analysis` artifact in `triage` mode with the resulting verdicts.

The `anthropic` provider posts to the Anthropic Messages API (`POST {base_url}/messages` with `x-api-key` and `anthropic-version: 2023-06-01` headers). The same provider is available to the shared AI runtime (auto-fix and natural-language rules) through `ai.provider.type: "anthropic"` in config; `ai.provider.api_key_env` defaults to `ANTHROPIC_API_KEY` and `ai.provider.model` defaults to `claude-sonnet-4-6`.

## Provider retry and timeout controls

All HTTP providers (OpenAI-compatible and Anthropic, in triage and the shared runtime) share the same retry behavior: exponential backoff with jitter on 429 responses, 5xx responses, and network errors, honoring the `Retry-After` header when present. Provider failures never crash a scan — triage keeps the static findings and records an error verdict, and fix generation simply reports the error.

- `CODEGUARD_AI_MAX_RETRIES=3` — retries after the first failed attempt
- `CODEGUARD_AI_RETRY_BASE_DELAY=250ms` — first backoff delay; subsequent delays double up to an 8s cap
- `CODEGUARD_AI_TIMEOUT=30s` — per-request timeout for the shared AI runtime providers
- `CODEGUARD_AI_TRIAGE_TIMEOUT=20s` — per-request timeout for triage providers

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
  - includes per-file `frameworks` metadata in the JSON request when changed source snapshots match known framework patterns
  - includes a structured `prompt` template in the JSON request so command-backed semantic runtimes receive explicit review instructions, response requirements, per-rule focus areas, and framework-specific reasoning guidance
  - each framework entry can now carry low-level `signals` plus higher-level `hints` that summarize likely contracts such as `middleware-next-chain`, `component-props-contract`, `client-component`, `route-segment-component`, and `route-handler-contract`
  - the prompt template now teaches `quality.ai.contract-drift` and `quality.ai.semantic-test-adequacy` to explicitly reason about props contracts, route `params` or `searchParams`, request or response contract shifts, and Express middleware sequencing
  - current framework coverage is still intentionally narrow but broader than the first slice: Express route and middleware modules, React component files, and Next.js route/component conventions (`app/**/route.*`, `pages/api/**`, `app/**/page.*`, `app/**/layout.*`, `app/**/loading.*`, `app/**/error.*`, and `next/server` request/response patterns)
  - `ai.semantic.function_contract`, `ai.semantic.contract_drift`, `ai.semantic.misleading_error_messages`, `ai.semantic.test_behavior_coverage`, and `ai.semantic.test_adequacy` control which semantic prompts are sent
  - the external semantic command must read a JSON request from stdin and return `{"verdicts":[...]}` with `rule_id`, `path`, `line`, `level`, and `message`

### Reference semantic runner

The repo now ships a scaffold runner at `examples/semantic/reference_runner.py`.

Example wiring:

```bash
export CODEGUARD_SEMANTIC_CHECKS=1
export CODEGUARD_SEMANTIC_COMMAND="python3 examples/semantic/reference_runner.py"
```

What it does:
- reads the semantic request JSON from stdin
- renders the canonical prompt text from `prompt`, `frameworks`, `diff`, `source_files`, and `test_files`
- prints that prompt to stderr when `CODEGUARD_SEMANTIC_REFERENCE_PRINT_PROMPT=1`
- defaults to scaffold mode and returns `{"verdicts":[]}` so it is safe as a no-op

Backend modes:

- scaffold mode:
  - `CODEGUARD_SEMANTIC_REFERENCE_MODE=scaffold`
  - returns an empty `verdicts` array
- local command mode:
  - `CODEGUARD_SEMANTIC_REFERENCE_MODE=command`
  - `CODEGUARD_SEMANTIC_REFERENCE_LOCAL_COMMAND="python3 my-semantic-backend.py"`
  - sends `{"request":<original request>,"prompt_text":"..."}` to the backend command on stdin
  - expects `{"verdicts":[...]}` on stdout
- OpenAI-compatible mode:
  - `CODEGUARD_SEMANTIC_REFERENCE_MODE=openai`
  - `CODEGUARD_SEMANTIC_REFERENCE_OPENAI_BASE_URL=https://api.openai.com/v1`
  - `CODEGUARD_SEMANTIC_REFERENCE_OPENAI_API_KEY=...`
  - `CODEGUARD_SEMANTIC_REFERENCE_OPENAI_MODEL=gpt-5`
  - posts the rendered prompt to `/chat/completions` and expects the model message content to be a JSON object with `verdicts`

This is intended as the canonical starting point for custom semantic commands. The prompt assembly stays fixed in one place, and you can swap only the backend transport.

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
