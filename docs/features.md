# Features

This page lists the current `codeguard` feature surface and the main config entrypoints for operators.

## Core scan families

- `quality`
  - maintainability thresholds
  - clone detection
  - language-native quality heuristics for Go, Python, TypeScript, JavaScript, Rust, Java, C#, and Ruby
  - AI-quality heuristics such as swallowed errors, narrative comments, hallucinated imports, dead code, over-mocked tests, idiom drift, semantic review, provenance policy, and change-risk rollups
  - changed-line coverage gating in diff mode
- `design`
  - layering and boundary rules
  - import cycle and god-module detection
  - high-impact-change analysis and dependency graph artifacts
- `security`
  - hardcoded secrets and private keys
  - Go, Python, TypeScript, and JavaScript taint-style flow checks
  - insecure API heuristics
  - optional `govulncheck`
- `prompts`
  - prompt-asset governance
  - agent config and MCP config checks
  - dangerous instruction and standing-permission detection
- `ci`
  - workflow/release policy
  - test-quality heuristics
- `supply_chain`
  - manifest normalization
  - lockfile presence and drift validation
  - unpinned dependency detection
  - dependency and manifest license policy

## Agent-native features

- `codeguard serve --mcp`
- `codeguard validate-patch`
- `codeguard explain -format agent`
- verified auto-fix through SDK and CLI
- hook-pack examples for Claude Code and Cursor

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

## Parsers

- `parsers.treesitter: "off" | "auto"` (default `"off"`) selects the parsing
  substrate for TypeScript/TSX/JavaScript rules (`docs/treesitter-spike.md`).
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

## Next queued AI features

These are the tracks currently being planned for follow-up implementation:

- batch verified fix planning
- `agent.permission-risk`
