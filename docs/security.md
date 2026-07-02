# Security & OWASP

This page documents codeguard's own trust model and its OWASP Top 10 (2021)
coverage. For the catalogue of checks codeguard runs against *your* repository,
see [Checks reference](/Users/alex/Documents/GitHub/codeguard/docs/checks.md:1).

## Trust model: repository config is untrusted by default

codeguard is frequently run in CI against pull requests, including from
untrusted contributors. Its behavior is driven by `codeguard.yaml` and rule
packs, which live in the repository and are therefore controllable by whoever
opens the PR. To prevent a code-review tool from becoming a remote-code-execution
or credential-exfiltration vector, the following capabilities are **disabled by
default** and must be explicitly enabled by the trusted operator (via the
process environment or a CLI flag — never from the repo config itself):

| Capability | Default | Opt-in env | Opt-in flag |
| --- | --- | --- | --- |
| Run commands defined in config (`*_rules.language_commands`, `license_commands`, `ai.autofix.test_commands`, the `command` AI provider, nlrule/semantic command runtimes) | refused | `CODEGUARD_ALLOW_CONFIG_COMMANDS=1` | `--allow-config-commands` |
| Use an AI provider `baseURL` outside the built-in allowlist, and reach non-public addresses | refused | `CODEGUARD_ALLOW_CONFIG_AI_ENDPOINTS=1` | `--allow-config-ai-endpoints` |

For a repository you control end-to-end, enable the capabilities you need:

```bash
codeguard scan --allow-config-commands            # run configured commands
CODEGUARD_ALLOW_CONFIG_AI_ENDPOINTS=1 codeguard scan --ai   # custom/self-hosted LLM
```

### What the defaults protect against

- **Command injection / RCE in CI (A03 / A08).** Without the opt-in, codeguard
  refuses to execute any command supplied by the repository config.
- **Credential exfiltration & SSRF (A10 / A02).** AI provider base URLs from
  config are restricted to a small allowlist of known public hosts
  (`api.openai.com`, `api.anthropic.com`). The provider HTTP client also refuses
  to connect to loopback, private, and link-local addresses (including the cloud
  metadata endpoint), defending against DNS-rebinding and redirect-based SSRF.
  Provider responses are size-bounded to prevent memory exhaustion.
- **Path traversal / arbitrary file write (A01).** Config-controlled artifact
  paths (`baseline.path`, `cache.path`, `ai.cache.path`) are resolved relative
  to the config directory and rejected if they escape that directory tree.

Environment variables are the trust anchor because, in a standard
`pull_request` workflow, the environment is controlled by the workflow author
(base branch), not by the PR.

## OWASP Top 10 (2021) coverage

Every built-in security rule is tagged with its OWASP Top 10 (2021) category.
The mapping is surfaced in:

- `codeguard rules` — appends the category as a trailing column for security rules.
- `codeguard explain <rule-id>` — an `owasp:` line (text) / `owasp_category` field (`-format agent`).
- SARIF output — each rule carries `properties.tags` (`OWASP:Axx:2021`) and an `owasp` property, which GitHub code scanning surfaces.

Use the coverage report to see which categories have rules and which are gaps:

```bash
codeguard owasp                 # text report
codeguard owasp -format json    # machine-readable
```

Example:

```
OWASP Top 10 (2021) coverage: 9/10 categories have rules

[ok  ] A01:2021-Broken Access Control (2 rules)
[ok  ] A02:2021-Cryptographic Failures (11 rules)
[ok  ] A03:2021-Injection (24 rules)
[gap ] A04:2021-Insecure Design (0 rules)
[ok  ] A05:2021-Security Misconfiguration (4 rules)
[ok  ] A06:2021-Vulnerable and Outdated Components (1 rules)
[ok  ] A07:2021-Identification and Authentication Failures (1 rules)
[ok  ] A08:2021-Software and Data Integrity Failures (1 rules)
[ok  ] A09:2021-Security Logging and Monitoring Failures (2 rules)
[ok  ] A10:2021-Server-Side Request Forgery (SSRF) (2 rules)
```

`A04` (Insecure Design) is left as an explicit gap: it is a design-level risk
that static heuristics cannot reliably detect, and a false "covered" there
would be misleading. `A09` is covered by two heuristics that target the
code-visible slice of the category: secrets flowing into log output and raw
errors leaking to HTTP clients instead of being logged server-side.

### Newly added detection rules

These heuristic rules close the previously-empty categories. The misconfiguration
and crypto rules are text-based and default to `warn`; the SSRF rules use the
taint engine and default to `fail`.

| Rule | OWASP | Detects |
| --- | --- | --- |
| `security.hardcoded-credential` | A07 | values matching known credential formats (AWS, GitHub, GitLab, Slack, Stripe, Google, npm, PyPI, Docker, SendGrid, Twilio, Azure, DB connection strings, Bearer tokens) or a configured custom pattern; **fail** |
| `security.high-entropy-string` | A07 | opt-in Shannon-entropy heuristic for unknown/random secrets; **warn** |
| `security.cors-wildcard` | A05 | `Access-Control-Allow-Origin: *` |
| `security.debug-enabled` | A05 | framework debug flag enabled (`debug=True`) |
| `security.bind-all-interfaces` | A05 | binding to `0.0.0.0` |
| `security.dockerfile-root` | A05 | Dockerfile `USER root` |
| `security.weak-hash` | A02 | MD5 / SHA-1 used for security |
| `security.weak-cipher` | A02 | DES / RC4 / ECB mode |
| `security.insecure-deserialization` | A08 | `pickle`, unsafe `yaml.load`, Java `readObject`, `Marshal.load`, `unserialize` |
| `security.log-secret-exposure` | A09 | secret-named identifiers (password, token, api_key, …) inside the argument list of a Go/Python/TS/JS logging call, secret-named structured-log keys, and secret-labeled string literals concatenated or format-directed into log output |
| `security.unsanitized-error-response` | A09 | raw error values written directly into HTTP responses: Go `http.Error(w, err.Error(), …)` / `fmt.Fprintf(w, …, err)`, TS/JS `res.send(err)` / `res.json(err)` / `res.status(…).send(err.stack \|\| err.message)`, Python `return str(e)` / `HttpResponse(str(e))` inside `except` blocks |
| `security.ssrf.go` / `security.ssrf.python` | A10 | untrusted input flowing into an outbound HTTP request URL |

## Release integrity (supply chain)

codeguard's own releases are hardened against tampering:

- **Signed artifacts** — the `SHA256SUMS` checksum file is signed with keyless
  cosign (Sigstore/Fulcio); container images are signed with `cosign sign`.
- **SBOM** — a CycloneDX SBOM is generated per archive via `syft`.
- **SLSA provenance** — a signed in-toto build provenance attestation is produced
  over the artifact hashes and attached to the release.
- **Dependabot** — weekly updates for Go modules, GitHub Actions, and Docker base
  images keep dependencies current (A06).

To verify a downloaded release:

```bash
cosign verify-blob \
  --certificate SHA256SUMS.pem \
  --signature SHA256SUMS.sig \
  --certificate-identity-regexp 'https://github.com/devr-tools/codeguard/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  SHA256SUMS
```
