# Getting Started

`codeguard` is a standalone Go service and CLI for repository checks around code quality, design boundaries, security, CI/CD hygiene, and AI prompt governance.

## Commands

```bash
codeguard init
codeguard validate -config codeguard.json
codeguard scan -config codeguard.json
```

## Current scope

- Go-first runtime support
- Standalone reusable package layout
- Config, runner, report, and CLI boundaries separated

Language-specific engines can be added later under the `codeguard/` package tree without changing the repo shape.
