# Integrations

## Pre-commit

`codeguard` ships a `.pre-commit-hooks.yaml` file so repositories can install it directly:

```yaml
repos:
  - repo: https://github.com/devr-tools/codeguard
    rev: v0.1.0
    hooks:
      - id: codeguard
        args: ["-config", "codeguard.yaml", "-profile", "startup"]
```

The packaged hook runs `codeguard scan -mode diff -base-ref HEAD` by default.

## GitHub Action

This repository also ships a composite action at `action.yml`:

```yaml
- name: CodeGuard
  uses: devr-tools/codeguard@v0.1.0
  with:
    config: codeguard.yaml
    profile: strict
    mode: diff
    base-ref: origin/main
    format: github
```

The action installs `github.com/devr-tools/codeguard/cmd/codeguard` and runs `codeguard scan`.
