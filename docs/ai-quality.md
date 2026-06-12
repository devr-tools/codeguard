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

## Current scope

- Hallucinated import detection is local-manifest based:
  - Go imports are checked against `go.mod` and the local module path
  - TypeScript and JavaScript imports are checked against `package.json`, workspace package names, built-in Node modules, and local relative files
- Over-mocked test detection is heuristic:
  - warns when mock setup strongly outweighs behavior assertions
- Dead-code detection is heuristic:
  - currently focuses on obvious constant-condition branches such as `if false` and `if (false)`

## Follow-on opportunities

- add Python import-resolution support against lockfiles and environments
- expand idiom drift beyond test frameworks into error handling and naming style
- add PR-level provenance adapters for hosted review systems
