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

`codeguard init` writes `codeguard.yaml` by default. That file is a *starter*
config: it is complete and opinionated, and it states CodeGuard's own
conventions (a `.github/workflows/ci.yml`, a `test` gate, `.goreleaser.yaml`,
`Makefile`, tests under `tests/**`, and the `cmd/` → `internal/cli` package
layering). Edit or delete the keys that do not describe your repository.

Use `codeguard scan -folder <path>` when you want to scan only one folder.
`-path <path>` is accepted as an alias. If no config file exists and you did
not pass `-config`, a folder scan runs the built-in defaults with the
recommended check sections enabled — Code Quality, Design Patterns, Security,
AI Prompts and CI/CD, plus the sections CodeGuard selects by scan mode (Agent
Context on full scans, API Contracts on diff scans). This is *not* the config
`codeguard init` writes: the defaults assert nothing about your repository's
shape, so none of the required-file, workflow-content, test-layout or
package-layering policy above is enforced until you configure it. Add
`-profile startup`, `-profile strict`, `-profile enterprise`, or `-profile
ai-safe` to overlay a policy profile, which may also enable further sections
(for example `enterprise` adds Supply Chain and Delivery).

### CI flags and exit codes

`codeguard scan` exits 0 when the scan passes its exit policy and 1 otherwise.
Useful flags for CI:

- `-fail-on fail|warn|none` (config: `output.fail_on`): `fail` (default) exits
  1 when any section fails; `warn` also exits 1 when a section warns (strict
  mode); `none` reports without failing on findings (rollout mode). Errors —
  invalid config, a missing target, a crashed section (`scan.section.panic`)
  or a failed target walk (`scan.target-walk`) — exit 1 under every policy.
- `-min-confidence low|medium|high`: drop findings below that confidence; it
  maps onto `checks.min_confidence.default`.
- `-quiet`: no banner and no progress lines; findings and the summary are
  still printed. The banner is also skipped automatically when stdout is not a
  terminal, when `CI` is set, or when `NO_COLOR` is set.

```bash
codeguard scan -config codeguard.yaml -fail-on none -quiet      # rollout: report only
codeguard scan -config codeguard.yaml -fail-on warn             # strict: warnings block
codeguard scan -config codeguard.yaml -min-confidence medium    # hide low-confidence noise
```

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
- `text`, `json`, `sarif`, `github`, `github-comment`, and `cyclonedx` report formats
- Diff-mode filtering down to changed lines when Git history is available
- Production rollout tools such as baselines, waivers, diff scans, and rule metadata discovery

Language-specific engines can be added later without changing the repo shape.
