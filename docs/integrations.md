# Integrations

## Pre-commit

`codeguard` ships a `.pre-commit-hooks.yaml` file so repositories can install it directly:

```yaml
repos:
  - repo: https://github.com/devr-tools/codeguard
    rev: v1.11.0
    hooks:
      - id: codeguard
        args: ["-config", "codeguard.yaml", "-profile", "startup"]
```

The packaged hook runs `codeguard scan -config codeguard.yaml -mode diff -base-ref HEAD` by default.

## GitHub Action

This repository also ships the GitHub Marketplace action `Devr Codeguard` from `action.yml`:

```yaml
- name: Devr Codeguard
  uses: devr-tools/codeguard@v1.11.0
  with:
    config: codeguard.yaml
    profile: strict
    mode: diff
    base-ref: origin/main
    format: github
```

The action installs `github.com/devr-tools/codeguard/cmd/codeguard` and runs `codeguard scan`.

For pull request workflows, the action can also publish a sticky fix-oriented comment:

```yaml
- name: Devr Codeguard
  uses: devr-tools/codeguard@v1.11.0
  with:
    config: codeguard.yaml
    mode: diff
    base-ref: origin/main
    format: github
    comment-fix-mode: sticky
```

`format: github` preserves workflow annotations. `comment-fix-mode: sticky` adds or updates a PR comment with concrete fix suggestions derived from the same findings. A sticky update only edits a comment written by the token's own account (`github-actions[bot]` for the default `GITHUB_TOKEN`, or the token owner for a personal access token), so a marker comment posted by anyone else is ignored and a new comment is created instead.

### Inline PR review comments

The action can also post findings as line-level pull request review comments, in the style of reviewdog's `github-pr-review` reporter. It is off by default:

```yaml
permissions:
  contents: read
  pull-requests: write

steps:
  - uses: actions/checkout@<sha> # pin to a full commit SHA
    with:
      fetch-depth: 0
  - name: Devr Codeguard
    uses: devr-tools/codeguard@v1.11.0
    with:
      config: codeguard.yaml
      mode: diff
      base-ref: origin/main
      review-comments: "on"
      review-comment-limit: "25"
```

- `review-comments`: `on` posts inline review comments; `off` (the default) does not. Quote `"on"` so YAML keeps it a string.
- `review-comment-limit`: the most inline comments one run posts (default `25`). The review body lists the rest by path, line, rule, and message.

The step runs `codeguard scan -format json` and makes one review per run with `POST /repos/{owner}/{repo}/pulls/{n}/reviews`. That review uses `event: COMMENT`, so it never approves a pull request or requests changes. The step runs only for `pull_request` and `pull_request_target` events, the same gate as the sticky comment. It needs `pull-requests: write`.

- **Diff lines only.** The step reads the PR's files (`GET /pulls/{n}/files`) and parses each patch into new-file line numbers. A finding gets an inline comment only when its line is an added or context line on the RIGHT side of the diff, because GitHub rejects comments outside a diff hunk. Findings outside the diff are counted but not commented; the sticky comment and annotations still cover them. Files without a patch, such as binary or very large files, get no inline comments. Only unsuppressed `fail` and `warn` findings with a file line are considered, ordered fail-first, so the cap keeps the most severe ones.
- **No duplicates across runs.** Each comment starts with a hidden `<!-- codeguard-review:<fingerprint> -->` marker. The fingerprint is the finding's context fingerprint, which survives edits that only shift its line. Before posting, the step lists the PR's review comments and skips findings already commented. Like the sticky comment, it only trusts markers in comments written by the token's own account. A marker posted by anyone else can't suppress a finding.
- **Rejected comments.** If GitHub rejects the whole review with `422`, for example because a new push moved a line out of the diff, the step posts each comment on its own against the scanned head commit. It skips the ones GitHub still rejects and lists them in a body-only review. Other API errors fail the step.
- **Suggestions.** A comment carries a ```` ```suggestion ```` block only when codeguard knows the exact replacement. Today that is a line holding nothing but a `debugger` statement (`quality.typescript.debugger-statement`, `quality.javascript.debugger-statement`), where the suggestion deletes the line. Every other finding gets its rule id, message, how-to-fix text, and a link to the rule docs. Guided fix templates are advice, not replacements, so they never become suggestions. `security.*` findings and findings with secret metadata never get a suggestion block, because a suggestion repeats source text.
- **Escaping.** Messages and how-to-fix text come from the scanned repository, so they are flattened to one line with markdown and HTML metacharacters escaped. `@` mentions are defused. Suggestion fences are longer than any backtick run in the replacement.

Finding paths are relative to the scan target. Inline comments line up with PR paths when the target is the repository root.

`fail-on` sets when the action fails the job, and passes `-fail-on` to `codeguard scan`:

```yaml
- name: Devr Codeguard
  uses: devr-tools/codeguard@v1.11.0
  with:
    config: codeguard.yaml
    fail-on: warn
```

- `fail`: fails when a section fails.
- `warn`: also fails when a section warns.
- `none`: fails only on operational errors, such as a crashed section.

Leave it empty (the default) to use `output.fail_on` from the config, or `fail` if the config doesn't set it. Because `fail-on` is newer than v1.11.0, pin a release that includes it.

Every input reaches the action's shell steps through `env:` and is quoted as a
shell variable. Inputs are never interpolated into the `run:` scripts, so an
input value can't inject shell commands into the caller's job.

### GitHub code scanning (SARIF)

To show codeguard findings in the repository's **Security → Code scanning**
tab, write SARIF and upload it with `github/codeql-action/upload-sarif`:

```yaml
permissions:
  contents: read
  security-events: write

steps:
  - uses: actions/checkout@v4
  - uses: actions/setup-go@v5
    with:
      go-version: stable
  - run: go install github.com/devr-tools/codeguard/cmd/codeguard@latest
  - name: codeguard (SARIF)
    run: codeguard scan -config codeguard.yaml -format sarif -fail-on none > codeguard.sarif
  - uses: github/codeql-action/upload-sarif@v3
    if: always()
    with:
      sarif_file: codeguard.sarif
      category: codeguard
```

`-fail-on none` lets the upload step run when findings fail the scan.
Operational failures, such as a crashed section, still exit 1 and are also
recorded as `executionSuccessful: false` in the SARIF invocation.

In code scanning, each alert shows:
- **Severity.** Security rules carry `security-severity` and appear as
  critical, high, medium or low. Other rules appear as error or warning
- **Rule help.** The rule description, how-to-fix guidance, a before/after
  example, CWE links, and a link to the rule's entry in
  [docs/rule-index.md](rule-index.md)
- **Taint traces.** Go, Python, C++ and TypeScript/JavaScript taint findings
  include a code flow from source to sink
- **CWE tags.** Tags use CodeQL's format (`external/cwe/cwe-078`), so a CWE
  filter groups codeguard alerts with CodeQL alerts
- **Stable alerts.** `partialFingerprints` keeps an alert stable across commits
  when unrelated edits move it

The full field reference and the severity mapping are in
[Checks → SARIF output](checks.md#sarif-output-github-code-scanning). The
marketplace action does not upload SARIF yet. Use the workflow above until it
has a SARIF upload input.

## Agent Hook Packs

`codeguard` now ships example hook packs under [examples/hooks](../examples/hooks/README.md).

Included packs:

- Claude Code
  - [examples/hooks/claude-code/pre-tool-use.sh](../examples/hooks/claude-code/pre-tool-use.sh)
  - [examples/hooks/claude-code/post-edit.sh](../examples/hooks/claude-code/post-edit.sh)
- Cursor
  - [examples/hooks/cursor/before-apply.sh](../examples/hooks/cursor/before-apply.sh)
  - [examples/hooks/cursor/after-edit.sh](../examples/hooks/cursor/after-edit.sh)
  - [examples/hooks/cursor/mcp.json.example](../examples/hooks/cursor/mcp.json.example)
- Devin
  - [examples/hooks/devin/mcp-http.json.example](../examples/hooks/devin/mcp-http.json.example)
  - [examples/hooks/devin/mcp-stdio.json.example](../examples/hooks/devin/mcp-stdio.json.example)
  - [examples/hooks/devin/run-http.sh](../examples/hooks/devin/run-http.sh)
  - [examples/hooks/devin/setup-snapshot.sh](../examples/hooks/devin/setup-snapshot.sh)

The packs are script-first on purpose:

- pre-write hooks call `codeguard validate-patch`
- post-edit hooks call `codeguard scan -mode diff`
- the Cursor pack also includes an MCP server example for `codeguard serve --mcp`
- the Devin pack provides HTTP and stdio MCP configs plus host scripts (see below)

Start with:

```bash
export CODEGUARD_CONFIG=codeguard.yaml
export CODEGUARD_PROFILE=ai-safe
export CODEGUARD_BASE_REF=origin/main
```

Then wire the scripts into your host's hook or workflow settings.

## MCP Server

`codeguard serve --mcp` runs an MCP server (hand-rolled JSON-RPC 2.0, no extra
dependencies). It advertises `tools`, `resources`, `prompts`, and `logging`
capabilities and negotiates protocol versions `2025-11-25` (current) and
`2025-06-18` (compatibility).

### Tools, resources, prompts

- Tools: `scan`, `validate_patch`, `validate_config`, `explain`, `list_rules`,
  `verify_fix`, `propose_fix` (read-only), and `apply_fix` (destructive — writes
  the working tree). `scan` streams a progress notification per check section.
- Resources: `codeguard://rules` (rule catalog), `codeguard://config` (active
  configuration), and the `codeguard://rules/{rule_id}` template (per-rule
  explanation).
- Prompts: `review-diff`, `triage-findings`, `explain-rule`.

### Filtering and paginating findings

`scan` and `validate_patch` have two response shapes:

- **Full report (the default).** A call that passes none of the inputs in the
  table below gets the full report, exactly as in 1.10: top-level `sections[]`
  with every finding and all of its evidence, plus `summary` and `artifacts`.
  Existing MCP clients keep working unchanged. `format: "report"` asks for it
  explicitly.
- **Summary-first (recommended).** On a real repository the full report is
  thousands of findings, which floods an agent's context window. Pass
  `format: "summary"` (or any filter or paging input below) to get a compact
  summary and one page of findings instead. New clients should always opt in.

The full report stays the default for the rest of 1.x. The next major
version may make the summary the default, so pass `format` explicitly if you
depend on either shape.

Optional inputs (all tools that return findings). Passing any of them, with any
non-null value (even `verbose: false` or `max_findings: 50`), selects the
summary shape; combining one with `format: "report"` is an error:

| Input | Type | Meaning |
|---|---|---|
| `format` | `report` \| `summary` | Response shape. Omitted: `summary` if any other input below is passed, else `report`. |
| `sections` | string[] | Only findings from these section ids (`security`, `quality`, `prompts`, ...). Unknown ids are rejected. |
| `rules` | string[] | Rule ids. An entry ending in `.` or `*` is a prefix (`security.`, `prompts.secret*`); anything else is exact. |
| `min_level` | `warn` \| `fail` | Drop findings below this level. |
| `min_confidence` | `low` \| `medium` \| `high` | Sets `checks.min_confidence.default` before the scan, exactly like the CLI `-min-confidence` flag (per-section overrides still apply, and section status reflects it). Unspecified confidence counts as medium. |
| `paths` | string[] | Path globs: `*` within a segment, `**` across segments (`internal/**`, `cmd/*/main.go`). Findings without a path never match. |
| `max_findings` | integer | Page size, default `50`, capped at `500`. `0` returns only the summary (plus a cursor). |
| `cursor` | string | `next_cursor` from a previous call. |
| `verbose` | boolean | Add `title`, `severity`, `why`, `column`, `fingerprint`, and `metadata` per finding, non-operational diagnostics, and report artifacts. |

`scan` additionally accepts `changed_only: true`, which runs diff mode against
`base_ref` (default `main`; the working tree is compared, so uncommitted edits
count). It is equivalent to `mode: "diff"` and is rejected together with
`mode: "full"`. Like the other inputs it selects the summary shape; use
`mode: "diff"` to keep the full report. `validate_patch` takes `diff` (required unless `cursor` is set).

Summary response (`structuredContent`; the text content carries the same JSON):

```json
{
  "name": "my-repo",
  "scan_id": "9ff5f7ebf68e0a766b35470e3548b0db",
  "summary": {
    "passed_sections": 0, "warned_sections": 1, "failed_sections": 1,
    "total_findings": 14, "suppressed_findings": 0,
    "matched_findings": 14, "returned_findings": 3,
    "filtered_out": {"total": 0, "sections": 0, "rules": 0, "min_level": 0, "paths": 0, "min_confidence": 0},
    "top_rules": [{"rule_id": "prompts.secret-interpolation", "level": "fail", "count": 7}],
    "sections": [{"id": "prompts", "name": "AI Prompts", "status": "fail", "findings": 13, "matched": 13}]
  },
  "findings": [
    {"rule_id": "prompts.secret-interpolation", "level": "fail", "confidence": "medium", "section": "prompts",
     "location": "other/x.prompt:1", "path": "other/x.prompt", "line": 1,
     "message": "prompt contains secret interpolation pattern",
     "how_to_fix": "Remove secret placeholders from prompt assets and inject secrets outside the prompt text."}
  ],
  "offset": 0,
  "truncated": true,
  "next_cursor": "eyJ2IjoxLCJz...",
  "filters": {"max_findings": 3}
}
```

- `total_findings` is the report total; `matched_findings` is what survived the
  filters. `filtered_out.sections|rules|min_level|paths` count findings the
  filters removed (each attributed to the first filter it failed), and
  `filtered_out.min_confidence` counts findings the confidence policy dropped
  during the scan (these are not part of `total_findings`).
- Findings are ordered by level (fail first), then confidence (high first),
  then path, line, and rule id, so the order is deterministic.
- Operational diagnostics (a crashed check, an unwalkable target) are always
  listed under their section, because they mean the section is incomplete.
- **Pagination.** The server keeps the last 8 reports in memory for 30 minutes.
  A cursor pages through the exact report it was issued for — no re-scan, so
  pages never overlap or skip findings even if the tree changes. Pass the same
  `sections`, `rules`, `min_level`, `min_confidence`, and `paths` with the
  cursor (a mismatch is an error); `max_findings` and `verbose` may change
  between pages, and scan-shaping inputs (`config_path`, `mode`, ...) are
  ignored. An expired cursor returns an error asking you to re-run the call.
- **Output schema.** `tools/list` advertises one `outputSchema` per tool whose
  top-level properties are a superset of the 1.10 schema, so a client that
  ignores schema combinators validates either shape. A `oneOf` with a `report`
  branch (requires `sections` and `summary`) and a `summary` branch (requires
  `summary`, `findings`, and `truncated`) says which fields each shape carries;
  exactly one branch matches any response.
- **Compatibility.** The full report is still the default in 1.x, as described
  above. Summary-shape clients that need every finding follow `next_cursor`
  (or pass `max_findings: 500` per page); the CLI `codeguard scan -format json`
  also emits the full report.

### Verified auto-fix

- `verify_fix` takes a caller-supplied candidate `diff` plus the `finding` it
  addresses, applies it in an isolated workspace, re-scans the changed lines,
  runs the nearest inferred tests, and returns the result only if it passes.
- `propose_fix` generates the candidate first — via the client's LLM through MCP
  `sampling` when the client supports it (no API key needed), otherwise a
  configured AI provider — then runs the same verification.
- `apply_fix` verifies and, only on success, writes the diff to the working
  tree; when the client supports `elicitation` it asks the user to confirm
  first. It is the one destructive tool.
- All fail closed. Verification test execution is trust-gated, so run the
  server with `CODEGUARD_ALLOW_CONFIG_COMMANDS=1` to let the inferred tests run;
  otherwise verification fails closed. On failure, `verify_fix`/`propose_fix`
  return `structuredContent` with the attempted diff and remaining findings.

### Server→client requests (sampling, roots, elicitation)

The server consumes the client's `sampling`, `roots`, and `elicitation`
capabilities when the client advertises them at `initialize`. These are
server-initiated JSON-RPC requests: over stdio they interleave on the existing
streams; over Streamable HTTP the client must open the `GET {mcp-path}` SSE
stream (using the `Mcp-Session-Id` returned by `initialize`) over which the
server sends `sampling/createMessage`, `roots/list`, and `elicitation/create`,
and answer them on subsequent POSTs. Advertised `roots` widen the allowed set
for `config_path` confinement and are cached per connection (invalidated on
`notifications/roots/list_changed`); `elicitation` is used by `apply_fix` to
confirm before writing.

### Security

A caller-supplied `config_path` is confined to the server's config directory,
the working directory, and any client-advertised roots; paths outside are
rejected with a generic error so the HTTP transport is not a filesystem oracle.

### Transports

**stdio (default)** — for local subprocess hosts (Claude Code, Cursor):

```bash
codeguard serve --mcp -config codeguard.yaml -profile ai-safe
```

**Streamable HTTP** — for remote / cloud-hosted hosts (e.g. Devin):

```bash
codeguard serve --mcp --http \
  --addr 0.0.0.0:8080 \
  --mcp-path /mcp \
  --auth-token "$CODEGUARD_MCP_AUTH_TOKEN" \
  --auth-header Authorization \
  -config codeguard.yaml -profile ai-safe
```

The HTTP endpoint:

- accepts JSON-RPC over `POST {mcp-path}`; `tools/call` streams progress as
  `text/event-stream` (SSE), other methods return a single `application/json`
  response, and notifications return `202`.
- enforces an optional static bearer token (constant-time compared); a blank
  token disables auth and should only be used behind a private network. The
  token also falls back to `$CODEGUARD_MCP_AUTH_TOKEN`.
- exposes `GET /healthz` for health checks, caps request body size, limits
  concurrent tool executions, and drains in-flight requests on SIGINT/SIGTERM.
- mints an `Mcp-Session-Id` on `initialize`; `GET {mcp-path}` with that
  session id opens the server-initiated SSE stream used for sampling, roots,
  and elicitation requests, and `DELETE` ends the session.

### Devin

Devin connects to a custom MCP server over HTTP (recommended), SSE, or stdio.
See the [Devin pack README](../examples/hooks/devin/README.md)
for the full walkthrough. In short:

1. Host the server with `examples/hooks/devin/run-http.sh` behind a reachable
   URL (TLS/ingress/tunnel).
2. In Devin → Settings → MCP Marketplace → Add a custom MCP: Transport `HTTP`,
   Server URL `https://…/mcp`, Authentication `Auth Header` →
   `Authorization: Bearer <token>`.
3. Click **Test listing tools** — Devin runs `initialize` + `tools/list` and
   should discover the eight tools listed above.

For a binary-in-snapshot setup instead, use `setup-snapshot.sh` and the stdio
config in `mcp-stdio.json.example`.

## MCP Smoke Harness

`codeguard serve --mcp` is covered by a host-shaped smoke harness in `tests/mcp/testdata/transcripts/` and `tests/mcp/smoke_test.go`.

The harness launches the local MCP server, replays NDJSON transcripts that model real host request flows, and validates the returned JSON-RPC/MCP responses. The current profiles target:

- `editor-current`
- `editor-compat`
- `review-agent`
- `scan-agent`
- `resource-agent`
- `prompt-agent`
- `streaming-agent` (per-section progress)
- `verify-fix-agent` (verified fix fail-closed)

Bidirectional server→client flows (sampling, roots) and the HTTP transport are
covered by `tests/mcp/sampling_test.go` and `tests/mcp/http_test.go`, which act
as an MCP client and answer the server's `sampling/createMessage` and
`roots/list` requests.

Run it with:

```bash
go test ./tests/mcp -run TestMCPHostSmokeProfiles
```

Current scope:

- validates host-like `initialize`, `tools/list`, `tools/call`, `ping`, config/patch/explain, and `resources`/`prompts` flows
- validates the server's newline-delimited stdio JSON-RPC transport
- the Streamable-HTTP transport is covered by `tests/mcp/http_test.go` (initialize, auth, SSE tool calls, resources/prompts, health)

Out of scope:

- automating the actual desktop/editor hosts themselves
- `Content-Length` framed stdio compatibility
- OAuth-based HTTP authentication (static bearer only)
