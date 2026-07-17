# Production Rollout

This guide is for teams using `codeguard` as a real merge gate, scheduled audit,
or agent-facing policy service.

## Goals

In production, `codeguard` should do three things well:

- block genuinely risky changes with low ambiguity
- keep historical debt from drowning out new regressions
- make findings understandable enough that humans and agents can act on them

## Recommended rollout order

1. Validate the environment.

   Run:

   ```bash
   codeguard validate -config codeguard.yaml
   codeguard doctor -config codeguard.yaml
   ```

   This catches broken config, missing binaries, target-path issues, and optional
   command integrations before they fail inside CI.

2. Start with pull-request diff scans.

   Run `codeguard scan -mode diff` in CI first. This keeps the initial gate tight
   around changed code instead of failing the whole repository for historical debt.

3. Capture legacy findings in a baseline.

   If the repository already has known issues, write a baseline and check it in:

   ```bash
   codeguard baseline -config codeguard.yaml -output codeguard-baseline.json
   ```

   Then reference it from config so new regressions still fail while existing debt
   stays visible but suppressed.

4. Add full-repository scans.

   Keep diff scans as the merge gate, then add a scheduled or pre-release full scan
   to catch cross-repo issues that are not visible from a small diff.

5. Tighten by family, not all at once.

   A practical order is:

   - `security`
   - `quality`
   - `ci`
   - `design`
   - `contracts`
   - `performance`
   - `supply_chain`

   That ordering usually gives the fastest signal-to-noise improvement.

## How to read a failed scan

Every finding has four pieces that matter operationally:

- section: the owning area, such as `Security`, `Design Patterns`, or `Code Quality`
- rule ID: the stable identifier, such as `design.layer-boundary`
- level: `fail` or `warn`
- message: the repository-specific explanation with path and location

Use them this way:

- `fail` means the finding is intended to block merge or release unless it is fixed,
  waived, or suppressed through an explicit baseline.
- `warn` means the scan is surfacing risk or cleanup work without blocking by default.
- the rule ID is what you use in waivers, dashboards, automation, and `codeguard explain`.
- the message is optimized for the concrete instance, not the whole policy; read the
  rule explanation when you need broader context.

Helpful commands:

```bash
codeguard rules
codeguard explain design.layer-boundary
codeguard explain security.hardcoded-credential
```

`codeguard rules` is best for catalog discovery. `codeguard explain` is best when a
team or agent needs the meaning of one specific failure.

## Choosing what should block

Use blocking failures for:

- credential leaks
- contract breaks
- architecture violations with clear ownership boundaries
- unsafe prompt or MCP config patterns
- CI policy requirements

Use warnings for:

- maintainability drift
- cleanup-oriented design heuristics
- stability and reachability nudges
- performance smells that still need human review

If a rule is too noisy, do not normalize ignoring it. Either tune the config,
baseline the current debt, or disable that rule family intentionally.

## Baselines, waivers, and policy discipline

Use each mechanism for a different purpose:

- baseline: repository already has known findings; suppress them so only new
  regressions gate progress
- waiver: a specific known exception with a reason and optional expiry
- config change: the rule truly does not fit the repository's architecture or risk model

Good production hygiene:

- keep waivers narrow by rule and path
- add reasons that another engineer can evaluate later
- expire temporary waivers
- review baselines periodically so they do not become permanent blind spots

## Suggested CI pattern

For most teams:

- pull requests: `codeguard scan -mode diff`
- nightly or scheduled: `codeguard scan`
- release branches: `codeguard scan` plus contracts and supply-chain enforcement

Prefer SARIF or GitHub output when you want code-host annotations, and JSON when
another system or agent will consume the report programmatically.

## Human and agent workflows

For humans:

- treat the section as the routing signal
- treat the rule ID as the policy anchor
- treat the message as the local debugging hint

For agents:

- fetch rule metadata through `codeguard explain <rule-id>` or the MCP `explain` tool
- preserve the rule ID in summaries and fix proposals
- avoid collapsing `warn` and `fail` into one severity bucket
- use file path and line as the patch target, but use the rule explanation to avoid
  fixing the symptom while preserving the policy violation

## Design policy in production

Design checks are most effective when introduced in layers:

1. Start with graph warnings such as cycles, reachability, and stability.
2. Add public-surface and production/test isolation policies.
3. Add layer, domain, capability, and data-ownership boundaries once the
   repository's intended architecture is explicit.

This is especially important in mixed-language repositories where Go, TypeScript,
Python, Rust, and C++ may have different maturity levels in their local module layout.

## References

- [Getting started](getting-started.md)
- [Checks reference](checks.md)
- [Features](features.md)
- [Integrations](integrations.md)
