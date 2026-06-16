
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

```yaml
- name: Devr Codeguard
  uses: devr-tools/codeguard@v0.2.0
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
codeguard rules
codeguard profiles
codeguard explain security.hardcoded-secret
codeguard baseline -config codeguard.yaml -output codeguard-baseline.json
```

`codeguard rules` prints each rule's level, execution model, language coverage, section, and title. `codeguard explain <rule-id>` includes the same metadata for a single rule.

By default, `codeguard` looks for `codeguard.yaml`, `codeguard.yml`, or `codeguard.json` in the repository root. If those are missing, it also checks `.codeguard/codeguard.yaml`, `.codeguard/codeguard.yml`, and `.codeguard/codeguard.json`.

If you point `-config` at a directory such as `.codeguard`, `codeguard` will look inside it for `codeguard.*` or `config.*` files.

Text output includes ANSI color and emoji markers by default. Set `NO_COLOR=1` if you want plain terminal output.

If you want a JSON starting point instead, use [examples/codeguard.json](/Users/alex/Documents/GitHub/codeguard/examples/codeguard.json:1).

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

- [Getting started](/Users/alex/Documents/GitHub/codeguard/docs/getting-started.md:1)
- [AI-generated code quality](/Users/alex/Documents/GitHub/codeguard/docs/ai-quality.md:1)
- [Agent-native features](/Users/alex/Documents/GitHub/codeguard/docs/agent-native.md:1)
- [Integrations](/Users/alex/Documents/GitHub/codeguard/docs/integrations.md:1)
- [Hook-pack examples](/Users/alex/Documents/GitHub/codeguard/examples/hooks/README.md:1)
- [SDK guide](/Users/alex/Documents/GitHub/codeguard/docs/sdk.md:1)
- [Release automation](/Users/alex/Documents/GitHub/codeguard/docs/release-automation.md:1)
- [Homebrew packaging](/Users/alex/Documents/GitHub/codeguard/docs/homebrew.md:1)
- [Checks reference](/Users/alex/Documents/GitHub/codeguard/docs/checks.md:1)
- [Architecture](/Users/alex/Documents/GitHub/codeguard/docs/architecture.md:1)
