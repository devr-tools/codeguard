# Local Development Setup

How to set up, run, and work with this project locally. Non-obvious dependencies, environment config, common setup issues.

- **The self-scan cache lives at `.codeguard/.codeguard/cache.json`, not `.codeguard/cache.json`.** `cache.path` resolves relative to the config directory (`config.containConfigArtifactPaths`), and `make codeguard-ci` uses `-config .codeguard/codeguard.yaml`, so the default `.codeguard/cache.json` nests. When iterating on **rule logic**, delete that file before re-running `make codeguard-ci`: cache keys cover config + file content + the release-bumped scanner-version constant, so a code-only change to a check serves stale findings from the cache and looks like your fix didn't work.
- The Makefile runs Go via `env -u GOROOT go` (Makefile:6), so `make` targets work even with the shell's stale GOROOT; direct `go` commands need `export GOROOT=/opt/homebrew/opt/go/libexec && export PATH=$GOROOT/bin:$PATH` first.
