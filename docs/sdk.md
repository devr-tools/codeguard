# SDK Guide

The public Go SDK for this repository lives at:

```go
import "github.com/devr-tools/codeguard/pkg/codeguard"
```

Use the CLI when you want an operator-facing workflow. Use the SDK when you want to embed `codeguard` scans into another Go application or tool.

## Install

```bash
go get github.com/devr-tools/codeguard/pkg/codeguard
```

## Minimal example

```go
package main

import (
	"context"
	"log"
	"os"

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

## Common SDK entrypoints

- `codeguard.ExampleConfig()` returns a ready-to-edit starter config.
- `codeguard.ExampleConfigForProfile(name)` returns a starter config for a built-in profile.
- `codeguard.LoadConfigFile(path)` loads and validates a config file.
- `codeguard.ValidateConfig(cfg)` validates config without running a scan.
- `codeguard.Run(ctx, cfg)` runs a full scan.
- `codeguard.RunWithOptions(ctx, cfg, opts)` runs a full or diff scan.
- `codeguard.VerifyFix(ctx, cfg, finding, candidate, opts)` validates a proposed patch in a temp workspace, reruns `codeguard` against the diff, and executes verification tests before returning it.
- `codeguard.GenerateVerifiedFix(ctx, req)` asks a generator for a patch candidate and only returns it after the same verification flow passes.
- `codeguard.WriteReport(w, report, format)` writes `text`, `json`, `sarif`, or `github` output.
- `codeguard.WriteBaselineFile(path, entries)` writes a baseline file.
- `codeguard.BaselineEntriesFromReport(report)` extracts baseline entries from a report.
- `codeguard.Rules()` lists rule metadata for CLI-like discovery.
- `codeguard.RulesForConfig(cfg)` includes custom rule-pack metadata from config.
- `codeguard.ExplainRule(ruleID)` returns the metadata for one rule.
- `codeguard.ExplainRuleForConfig(cfg, ruleID)` resolves built-in and custom rules from config.
- `codeguard.Profiles()` lists built-in profile metadata.

## Typical flow

```go
cfg, err := codeguard.LoadConfigFile("codeguard.json")
if err != nil {
	log.Fatal(err)
}

report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{
	Mode:    codeguard.ScanModeDiff,
	BaseRef: "main",
})
if err != nil {
	log.Fatal(err)
}

if err := codeguard.WriteReport(os.Stdout, report, "json"); err != nil {
	log.Fatal(err)
}
```

## Verified fix flow

`VerifyFix` and `GenerateVerifiedFix` fail closed. They do not return a patch unless:

- the unified diff applies cleanly in an isolated temporary workspace
- a diff-scoped `codeguard` run returns no findings for the proposed change
- verification test commands pass

By default, the verifier infers nearest Go package tests from the changed files. Other languages should pass explicit `FixVerificationCommand` entries through `FixOptions.TestCommands` until broader test-command inference is added.
