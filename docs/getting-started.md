# Getting Started

`codeguard` is a standalone Go service and CLI for repository checks around code quality, design boundaries, security, CI/CD hygiene, and AI prompt governance.

## Install

```bash
go install github.com/devr-tools/codeguard/cmd/codeguard@latest
```

Or from this repository:

```bash
make build
```

`codeguard version` reports the embedded module version for binaries installed
with `go install module@version`. Local source builds report
`0.1.0-dev+<revision>` (with a `.dirty` marker for modified worktrees) when Go
embeds VCS metadata.

Or in GitHub Actions from GitHub Marketplace:

```yaml
- name: Devr Codeguard
  uses: devr-tools/codeguard@v1.1.1
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
codeguard scan -config codeguard.yaml -folder ./internal/codeguard
codeguard scan -folder ./internal/codeguard -profile startup
codeguard waivers audit -config codeguard.yaml
codeguard rules
codeguard profiles
codeguard explain prompts.secret-interpolation
codeguard baseline -config codeguard.yaml -output codeguard-baseline.json
```

`codeguard init` writes `codeguard.yaml` by default.

Use `codeguard scan -folder <path>` when you want to scan only one folder. `-path <path>` is accepted as an alias. If no config file exists and you did not pass `-config`, folder scans use CodeGuard's built-in default config; add `-profile startup`, `-profile strict`, `-profile enterprise`, or `-profile ai-safe` to choose a default profile.

After upgrading CodeGuard, run `codeguard waivers audit -config codeguard.yaml` to identify waiver cleanup candidates. CodeGuard stores audit snapshots with version, config fingerprint, scan scope, and matched finding fingerprints; it only marks a waiver as stale after upgrade when prior comparable evidence shows the waiver matched findings before and matches none now. JSON output is available with `-format json`.

If you prefer a JSON example, start from [examples/codeguard.json](/Users/alex/Documents/GitHub/codeguard/examples/codeguard.json:1).

## First production setup

Use this sequence for a real repository:

1. `codeguard init`
2. `codeguard validate -config codeguard.yaml`
3. `codeguard doctor -config codeguard.yaml`
4. `codeguard scan -mode diff -config codeguard.yaml`
5. `codeguard waivers audit -config codeguard.yaml` after CodeGuard upgrades
6. `codeguard baseline -config codeguard.yaml -output codeguard-baseline.json` if the repo has pre-existing debt

After that, add a full scan in scheduled CI and tighten check families
incrementally. The detailed rollout guidance lives in [Production rollout](production.md).

## Understanding results

- `fail` findings are intended to block until fixed, waived, or baselined.
- `warn` findings are advisory by default and are best used for gradual cleanup.
- `codeguard rules` lists every rule with section, level, execution model, and language coverage.
- `codeguard explain <rule-id>` explains what a specific failed check means and how to fix it.

## SDK import path

The public SDK import path is:

```go
import "github.com/devr-tools/codeguard/pkg/codeguard"
```

Minimal example:

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

## Current scope

- Go-first runtime support
- Standalone reusable package layout
- Config, runner, report, and CLI boundaries separated
- Exclusions, waivers, and baselines for incremental rollout
- Custom rule packs for repo-specific regex, content, and path policies
- Policy profiles such as `startup`, `strict`, `enterprise`, and `ai-safe`
- Cached file-hash scan results for faster repeat runs
- `doctor` checks for config, Git, govulncheck, targets, and cache setup
- `text`, `json`, `sarif`, and `github` report formats
- Diff-mode filtering down to changed lines when Git history is available
- Production rollout tools such as baselines, waivers, diff scans, and rule metadata discovery

Language-specific engines can be added later without changing the repo shape.
