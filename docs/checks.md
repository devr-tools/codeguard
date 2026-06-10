# Checks

This file documents the current check categories in `codeguard` and the config keys that control them.

## Top-level shape

```json
{
  "checks": {
    "quality": true,
    "design": true,
    "security": true,
    "prompts": true,
    "ci": true
  }
}
```

Each top-level boolean enables or disables an entire check family.

## Quality

Purpose:
- Go formatting and parse validation
- Maintainability thresholds
- Cyclomatic complexity checks

Config keys:

```json
{
  "checks": {
    "quality": true,
    "quality_rules": {
      "max_file_lines": 400,
      "max_function_lines": 80,
      "max_parameters": 5,
      "max_cyclomatic_complexity": 10
    }
  }
}
```

Current behavior:
- fails on parse errors
- fails on non-`gofmt` files
- warns when maintainability thresholds are exceeded

## Design

Purpose:
- Layer boundary enforcement
- Separation of concerns heuristics
- Clean-code naming heuristics
- SOLID-oriented heuristics

Config keys:

```json
{
  "checks": {
    "design": true,
    "design_rules": {
      "require_cmd_through_internal_cli": true,
      "forbid_internal_import_cmd": true,
      "forbid_service_import_internal": true,
      "forbid_service_import_cmd": true,
      "max_decls_per_file": 12,
      "max_methods_per_type": 8,
      "max_interface_methods": 5,
      "forbidden_package_names": ["util", "utils", "common", "helpers", "misc"]
    }
  }
}
```

Current behavior:
- fails on architecture boundary violations
- warns on principle drift such as overly generic package names, too many declarations, too many methods on a type, or oversized interfaces

## Security

Purpose:
- Hardcoded secret detection
- Private key detection
- Insecure TLS detection
- Shell execution review markers
- Optional `govulncheck`

Config keys:

```json
{
  "checks": {
    "security": true,
    "security_rules": {
      "govulncheck_mode": "auto",
      "govulncheck_command": "govulncheck"
    }
  }
}
```

`govulncheck_mode` values:
- `off`
- `auto`
- `required`

Current behavior:
- fails on blocking security findings
- warns on reviewable findings
- can surface per-vulnerability findings from `govulncheck`

## Prompts

Purpose:
- Prompt asset discovery
- Secret interpolation detection inside prompt files
- Unsafe instruction pattern detection

Config keys:

```json
{
  "checks": {
    "prompts": true,
    "prompt_rules": {
      "file_extensions": [".prompt", ".md", ".txt", ".tmpl", ".yaml", ".yml", ".json"],
      "path_contains": ["prompt", "system", "instruction", "template"],
      "forbid_secret_interpolation": true,
      "forbid_unsafe_instructions": true
    }
  }
}
```

Current behavior:
- fails on secret interpolation patterns
- warns on unsafe instruction patterns

## CI

Purpose:
- Workflow presence checks
- Workflow content checks
- Release file presence checks
- Automation entrypoint checks

Config keys:

```json
{
  "checks": {
    "ci": true,
    "ci_rules": {
      "require_workflow_dir": true,
      "required_workflow_files": [".github/workflows/ci.yml", ".github/workflows/cd.yml", ".github/workflows/release.yml"],
      "workflow_content_rules": [
        {
          "path": ".github/workflows/ci.yml",
          "required_contains": ["actions/checkout", "go test ./..."]
        },
        {
          "path": ".github/workflows/cd.yml",
          "required_contains": ["googleapis/release-please-action", "uses: ./.github/workflows/release.yml", "RELEASE_PLEASE_TOKEN"]
        },
        {
          "path": ".github/workflows/release.yml",
          "required_contains": ["goreleaser/goreleaser-action@v7", "sync-homebrew-formula", "Formula/codeguard.rb"]
        }
      ],
      "required_release_files": [".goreleaser.yaml", "Dockerfile.release", ".github/release-please-config.json", ".release-please-manifest.json", "CHANGELOG.md"],
      "required_automation_paths": ["Makefile", "scripts/commit.sh"]
    }
  }
}
```

Current behavior:
- fails when required workflow, release, or automation files are missing
- fails when required workflow content markers are missing

## Output

Config keys:

```json
{
  "output": {
    "format": "text"
  }
}
```

Supported values:
- `text`
- `json`

## Full example

See [examples/codeguard.json](/Users/alex/Documents/GitHub/codeguard/examples/codeguard.json:1) for the current full config.
