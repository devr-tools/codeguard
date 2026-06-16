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

Or in GitHub Actions from GitHub Marketplace:

```yaml
- name: Devr Codeguard
  uses: devr-tools/codeguard@v0.2.0
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
codeguard explain prompts.secret-interpolation
codeguard baseline -config codeguard.yaml -output codeguard-baseline.json
```

`codeguard init` writes `codeguard.yaml` by default.

If you prefer a JSON example, start from [examples/codeguard.json](/Users/alex/Documents/GitHub/codeguard/examples/codeguard.json:1).

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

Language-specific engines can be added later without changing the repo shape.
