# Benchmarks

CodeGuard benchmarks should answer three separate questions:

1. How fast is a PR-time scan?
2. How much useful signal does the detector corpus produce?
3. How often can fix generation propose and verify an acceptable change?

Do not collapse these lanes into a single competitive score. A tool can be
fast and noisy, slow and precise, or strong in a narrow category such as
secrets. Publish lane-level numbers with the exact corpus identity, command
line, tool version, and host shape.

## Competitive benchmark analysis

The useful comparison set is not "every static analyzer." Compare CodeGuard
against tools with overlapping user jobs, then label the overlap:

| Tool | Main overlap with CodeGuard | Benchmark lane |
| --- | --- | --- |
| Semgrep CE / Semgrep Code | multi-language SAST, custom policy rules, SARIF/JSON output | detector quality, PR runtime |
| CodeQL CLI | deep semantic security queries for supported languages, SARIF output | detector quality, full-analysis runtime |
| Gitleaks | hardcoded secret detection, JSON/SARIF reports | secrets precision/recall, runtime |
| TruffleHog | secret detection with verification-oriented workflows, JSON output | secrets precision/recall, verified-secret rate |
| Trivy | dependency, filesystem, IaC, and secret scanning, SARIF/JSON output | supply-chain/IaC/secrets lanes |
| Snyk CLI | SAST, dependency, license, container, and IaC scanning, JSON/SARIF output | detector quality, supply-chain lane |
| SonarScanner CLI / SonarQube CLI | code quality, security, local change feedback, server-backed analysis | quality/security signal, workflow fit |

Recommended first pass:

1. Use CodeGuard's existing security corpus as the first detector-quality lane.
   It already has line-level ground truth in `tests/corpus/expectations.yaml`.
2. Add adapters that normalize every external tool finding to
   `(tool, rule, path, line, severity, category, message)`.
3. Map external rules into coarse categories instead of pretending rule IDs are
   equivalent. Suggested initial categories are `secrets`, `tls`,
   `shell-exec`, `dynamic-code`, `unsafe-html`, `ssrf`, `taint`,
   `crypto-weakness`, `deserialization`, `cors`, and `container-user`.
4. Score each category as TP/FN/FP against the same fixture files. Count a
   hit as a TP when it lands on the expected file and category; use exact line
   matching where the external tool provides stable line numbers, otherwise
   allow file-level matching and report that separately.
5. Run the frozen PR benchmark for PR-time performance, but keep tools with
   different operating models honest. For example, CodeQL database creation
   and Sonar server upload/analysis should be reported as separate phases, not
   hidden inside or outside the timing window.

Current CodeGuard detector baseline from:

```sh
env -u GOROOT GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/corpus -run TestCorpusExpectations -v
```

| Metric | Value |
| --- | ---: |
| True positives | 68 |
| False negatives | 0 |
| False positives | 1 |
| Precision | 0.986 |
| Recall | 1.000 |

These numbers are only the in-repository security fixture baseline. They are
not a claim about public-project performance, supply-chain coverage, or fix
quality.

### External tool command shapes

Pin exact versions and run from a clean checkout. Prefer SARIF where available
because CodeGuard already imports external reports and GitHub code scanning
uses SARIF 2.1.0.

```sh
semgrep scan --config p/security-audit --sarif --output semgrep.sarif .
codeql database create codeql-db --language=<language> --source-root .
codeql database analyze --format=sarifv2.1.0 --output=codeql.sarif codeql-db <suite-or-pack>
gitleaks dir --redact --report-format sarif --report-path gitleaks.sarif .
trufflehog filesystem --json .
trivy filesystem --format sarif --output trivy.sarif .
snyk code test . --sarif-file-output=snyk-code.sarif
sonar-scanner -Dsonar.projectBaseDir=. -Dsonar.sources=.
```

The exact Semgrep ruleset, CodeQL query suite, Trivy scanners, Snyk
organization policy, and Sonar quality profile materially change results and
must be published next to the numbers.

### Output bundle

Each benchmark run should publish:

- `corpus.json` from `codeguard-benchmark export`
- raw tool reports, preferably SARIF or JSON
- normalized finding JSON
- per-category TP/FN/FP metrics
- cold and warm runtime summaries
- tool versions and command lines
- host metadata: OS, architecture, CPU model, core count, memory, and whether
  network access was enabled during scan execution

### External report normalization

Saved external scanner reports can be normalized without installing or running
the scanners during the benchmark report step:

```sh
go run ./cmd/codeguard-benchmark external \
  -report gitleaks=/tmp/gitleaks.sarif \
  -report trivy=/tmp/trivy.json \
  -report trufflehog=/tmp/trufflehog.json \
  -report semgrep=/tmp/semgrep.sarif \
  -json-out /tmp/external-normalized.json \
  -markdown-out /tmp/external-summary.md
```

The `external` command supports Gitleaks, Trivy, TruffleHog, and Semgrep saved
reports. It accepts SARIF 2.1.0 plus pragmatic JSON shapes from those tools and
emits a normalized schema with `(tool, rule_id, category, severity, path, line,
column, message)`. The Markdown summary compares raw counts by tool, category,
path, and line. It deliberately does not score TP/FN/FP; scoring still belongs
to a fixture-backed corpus lane where ground truth is available.

### Sources for comparator capabilities

- Semgrep Community Edition describes JSON/SARIF output and deterministic
  syntax-rule scanning: <https://semgrep.dev/products/community-edition/>
- CodeQL CLI documents `database analyze` with SARIF output:
  <https://docs.github.com/en/code-security/reference/code-scanning/codeql/codeql-cli-manual/database-analyze>
- Gitleaks documents JSON, CSV, JUnit, and SARIF report formats:
  <https://github.com/gitleaks/gitleaks>
- Trivy documents filesystem scanning:
  <https://trivy.dev/docs/dev/guide/references/configuration/cli/trivy_filesystem/>
- Snyk CLI documents `snyk code test` JSON/SARIF export:
  <https://docs.snyk.io/developer-tools/snyk-cli/scan-and-maintain-projects-using-the-cli/snyk-cli-for-snyk-code/view-snyk-code-cli-results>
- SonarScanner CLI documents CI analysis through `sonar-scanner`:
  <https://docs.sonarsource.com/sonarqube-cloud/analyzing-source-code/scanners/sonarscanner-cli>
- TruffleHog documents JSON scanner output and resource considerations:
  <https://trufflesecurity.com/docs/running-the-scanner>

## Frozen PR benchmarks

CodeGuard benchmarks PR-time scans using a small, versioned corpus of frozen
public pull-request checkouts. The repository does not fetch or vendor those
projects: the benchmark operator provisions each checkout at the exact commit
listed in the manifest, then runs the harness locally or in a dedicated CI
job.

The manifest schema is versioned and machine-readable. Each entry records a
repository, PR number, immutable base/head revisions, language, a worktree
name relative to `-work-root`, and a relative CodeGuard configuration path.
Use [manifest.example.json](../benchmarks/manifest.example.json) as the
onboarding template. Do not replace immutable revisions with branch names.

Export the corpus identity for an auditable result bundle:

```sh
go run ./cmd/codeguard-benchmark export \
  -manifest benchmarks/manifest.json -out corpus.json
```

After provisioning the listed worktrees beneath a single directory, measure
each diff scan:

```sh
go run ./cmd/codeguard-benchmark run \
  -manifest benchmarks/manifest.json \
  -work-root /private/tmp/codeguard-benchmark-worktrees \
  -binary ./dist/codeguard -warm-repeats 3 -out results.json
```

Results contain a first-process `cold` run and the requested `warm` repeats
for each entry. "Cold" means a fresh CodeGuard process; it does not claim to
clear the host filesystem cache or alter the repository's configured cache.
Record p50/p95 separately for cold and warm runs, and include the exported
corpus metadata next to any published figures.

Runtime is only one benchmark lane. Use the existing ground-truth detector
corpus for precision/noise, and verified-fix fixtures for proposal coverage,
verifier acceptance, and independently validated acceptance. Do not collapse
the three lanes into a single competitive score.
