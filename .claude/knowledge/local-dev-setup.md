# Local Development Setup

How to set up, run, and work with this project locally. Non-obvious dependencies, environment config, common setup issues.

- **The self-scan cache is at `.codeguard/.codeguard/cache.json`**, because `cache.path` resolves relative to the config dir. Delete it before re-running `make codeguard-ci` after changing rule logic: cache keys don't cover check code, so a fix can look like it did nothing.
- The shell's `GOROOT` is stale. `make` targets unset it (`env -u GOROOT`). Direct `go` commands need `export GOROOT=/opt/homebrew/opt/go/libexec PATH=/opt/homebrew/opt/go/libexec/bin:$PATH` or `env -u GOROOT go ...`.
- **golangci-lint (CI-blocking, v2.12.2) must be pinned to the Go it was built with, or it silently under-reports.** Unpinned, it hits stdlib typecheck errors, which suppress staticcheck findings in your code, and still exits 0. `make lint-strict` is unpinned and fails here. Use:

  ```
  env -u GOROOT GOTOOLCHAIN=go1.26.3 GOCACHE=/private/tmp/codeguard-go-cache \
    GOLANGCI_LINT_CACHE=/private/tmp/golangci-lint-cache /Users/alex/.go/bin/golangci-lint run
  ```

  Set `GOTOOLCHAIN` from `golangci-lint --version`, not from go.mod.
- **golangci-lint takes a global lock** and aborts with `parallel golangci-lint is running` when runs overlap across worktrees or agents. Serialise runs, or give each its own `GOLANGCI_LINT_CACHE`. It is never a code problem.
