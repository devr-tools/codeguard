
<p align="center">
  <img src="./img/cg.png" alt="codeguard placeholder banner" width="420">
</p>
  <a href="https://github.com/devr-tools/codeguard/actions/workflows/ci.yml"><img src="https://github.com/devr-tools/codeguard/actions/workflows/ci.yml/badge.svg" alt="CI status"></a>
  <a href="https://github.com/devr-tools/codeguard/actions/workflows/cd.yml"><img src="https://github.com/devr-tools/codeguard/actions/workflows/cd.yml/badge.svg" alt="CD status"></a>
  <a href="https://goreportcard.com/report/github.com/devr-tools/codeguard">
    <img src="https://goreportcard.com/badge/github.com/devr-tools/codeguard" alt="Go Report Card" />
  </a>
    <a href="https://www.linkedin.com/in/alxjohn">
    <img src="https://img.shields.io/badge/LinkedIn-alxjohn-blue?logo=linkedin" alt="LinkedIn" />
  </a>


`codeguard` is a standalone Go service and CLI for repository checks across code quality, design boundaries, security, CI/CD hygiene, AI prompt governance, and repo-specific policy rules.

It now supports repository exclusions, baselines, waivers, changed-lines diff scans, SARIF output, GitHub annotations, custom rule packs, natural-language custom rules through an optional AI runtime, policy profiles, scan caching, doctor checks, rule discovery from the CLI, native TypeScript/Python quality, design, and security heuristics, and language-specific command checks.

AI-generated-code quality coverage includes an AI-failure-mode rule pack, `slop_score` artifacts, provenance-aware review policy hooks, local idiom drift checks, optional provider-backed hybrid triage and semantic review passes, natural-language custom rules through an optional AI runtime, and a verified-fix flow that only returns patches after isolated patch validation plus test reruns succeed.

Rule discovery APIs expose per-check metadata, including `execution_model` (`go-native`, `language-agnostic`, or `command-driven`) and `language_coverage` (fixed target languages, `repository-wide`, or `configurable`).

## Installation

```bash
go install github.com/devr-tools/codeguard/cmd/codeguard@latest
```

Or build from source:

```bash
make build
```

Other install paths:

- GitHub Releases: tagged archives for direct download
- Homebrew: `brew install devr-tools/tap/codeguard`
- GitHub Marketplace Action: `Devr Codeguard`

npm (installs a prebuilt binary, no Go toolchain required):

```bash
npm install -g @devr-tools/codeguard
codeguard version
```

pip (installs a prebuilt binary per platform; the project is `devr-codeguard`
because the plain `codeguard` name is taken, but the command is still `codeguard`):

```bash
pip install devr-codeguard
codeguard version
```

```yaml
- name: Devr Codeguard
  uses: devr-tools/codeguard@v1.1.1
```

Or run in Docker:

```bash
docker build -t codeguard .
docker run --rm -v "$PWD:/workspace" -w /workspace codeguard scan
```

For local release automation:

```bash
make commit
make release
make release-check
make deploy
```

For SDK consumers:

```bash
go get github.com/devr-tools/codeguard/pkg/codeguard
```
## Quick Start

```bash
codeguard init
codeguard validate -config codeguard.yaml
codeguard doctor -config codeguard.yaml
codeguard scan -config codeguard.yaml
codeguard scan-history
codeguard rules
codeguard profiles
codeguard explain security.hardcoded-credential
codeguard baseline -config codeguard.yaml -output codeguard-baseline.json
```

`codeguard rules` prints each rule's level, execution model, language coverage, section, and title. `codeguard explain <rule-id>` includes the same metadata for a single rule.

By default, `codeguard` looks for `codeguard.yaml`, `codeguard.yml`, or `codeguard.json` in the repository root. If those are missing, it also checks for the same file names inside a `.codeguard/` directory.

If you point `-config` at a directory such as `.codeguard`, `codeguard` will look inside it for `codeguard.*` or `config.*` files.

Text output includes ANSI color and emoji markers by default. Set `NO_COLOR=1` if you want plain terminal output.

If you want a JSON starting point instead, use [examples/codeguard.json](examples/codeguard.json).

## Production Use

For production rollout, start in a narrow mode and expand deliberately:

1. Run `codeguard doctor` and `codeguard validate` in CI first so config and toolchain issues fail early.
2. Start with `codeguard scan -mode diff` on pull requests so only changed lines and diff-aware checks gate merges.
3. Create a baseline for legacy findings with `codeguard baseline` before turning on full-repo enforcement.
4. Enable stricter families such as `design`, `security`, `contracts`, `performance`, and `supply_chain` incrementally per repository.
5. Use `codeguard rules` and `codeguard explain <rule-id>` to document what a failure means before asking teams to act on it.

When a scan fails:

- `fail` findings should block merge or release until fixed, waived, or baselined intentionally.
- `warn` findings are non-blocking by default and are best used to drive cleanup, ownership, or gradual policy hardening.
- section names such as `Design Patterns`, `Security`, or `Code Quality` tell you what kind of action is expected.
- rule IDs are stable handles for waivers, baselines, dashboards, and agent workflows.

## SDK

Import the SDK from `github.com/devr-tools/codeguard/pkg/codeguard`.

```go
package main

import (
	"context"
	"log"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func main() {
	cfg := codeguard.ExampleConfig()
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		log.Fatal(err)
	}
	_ = report
}
```

## Docs

- [Getting started](docs/getting-started.md)
- [Production rollout](docs/production.md)
- [Features](docs/features.md)
- [Security & OWASP](docs/security.md)
- [AI-generated code quality](docs/ai-quality.md)
- [Agent-native features](docs/agent-native.md)
- [Integrations](docs/integrations.md)
- [Hook-pack examples](examples/hooks/README.md)
- [SDK guide](docs/sdk.md)
- [Release automation](docs/release-automation.md)
- [Homebrew packaging](docs/homebrew.md)
- [Checks reference](docs/checks.md)
- [Architecture](docs/architecture.md)
