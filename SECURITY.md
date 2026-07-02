# Security Policy

## Reporting a vulnerability

We take the security of codeguard seriously. If you believe you have found a
security vulnerability, please report it privately — **do not open a public
issue, pull request, or discussion for security reports.**

Preferred channel: use GitHub's private vulnerability reporting for this
repository. Go to the **Security** tab → **Report a vulnerability**
(<https://github.com/devr-tools/codeguard/security/advisories/new>). This opens a
private advisory visible only to you and the maintainers.

If private reporting is unavailable to you, contact a maintainer directly rather
than disclosing publicly, and we will open a private advisory on your behalf.

Please include, where possible:

- a description of the vulnerability and its impact;
- the affected version (`codeguard version`) and platform;
- step-by-step reproduction, ideally with a minimal repository or config; and
- any proof-of-concept, logs, or suggested remediation.

## Our commitment

- **Acknowledgement** within **3 business days** of your report.
- **Triage and severity assessment** (CVSS-based) within **10 business days**,
  including whether we accept the report and an initial remediation plan.
- **Progress updates** at least every **10 business days** until resolution.
- **Coordinated disclosure**: we aim to ship a fix and publish an advisory
  within **90 days** of triage, and will credit reporters who wish to be named.

Please give us a reasonable opportunity to remediate before any public
disclosure.

## Supported versions

codeguard is pre-1.0 and released from the latest tag. Security fixes are
applied to the most recent release only; please upgrade to the latest version
before reporting.

| Version | Supported |
| --- | --- |
| Latest release | ✅ |
| Older releases | ❌ |

## Scope

In scope: the codeguard CLI, its GitHub Action, released container images, and
the release/supply-chain pipeline in this repository.

The product's own trust model (running untrusted repository configuration in CI)
and its OWASP Top 10 coverage are documented in
[docs/security.md](docs/security.md). Reports that the tool executes
config-supplied commands, reaches non-allowlisted network endpoints, or writes
outside the configured directory **without the documented opt-in** are in scope
and valued.

Out of scope: findings that require the operator to have explicitly enabled an
untrusted capability (`--allow-config-commands`,
`CODEGUARD_ALLOW_CONFIG_AI_ENDPOINTS`, etc.), and vulnerabilities in
third-party dependencies that have no impact on codeguard as shipped.
