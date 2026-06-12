# Local Development Setup

How to set up, run, and work with this project locally. Non-obvious dependencies, environment config, common setup issues.

- This machine has a stale `GOROOT=/Users/alex/apps/go` env var that breaks the homebrew Go toolchain. Run go commands as `env -u GOROOT go ...` (build, test, vet, gofmt).
- When iterating on check/rule logic, delete `.codeguard/cache.json` before self-scanning (`make codeguard-ci`). The scan cache keys on file hash + config hash but NOT the codeguard binary version, so rule-logic changes replay stale per-file findings — cross-file analyses (duplicate-code, import graphs) can appear to report zero findings when they're actually being skipped.
- Cross-file checks (clone detection, dependency graphs) only observe every file via `VisitTargetFiles` (cache-bypassing); `ScanTargetFiles` skips evaluators on cache hits and silently produces empty cross-file state on a second scan.
