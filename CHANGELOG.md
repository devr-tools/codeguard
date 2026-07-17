# Changelog

## [1.1.0](https://github.com/devr-tools/codeguard/compare/v1.0.0...v1.1.0) (2026-07-17)


### Features

* **cpp:** add tooling and security analysis ([b93d681](https://github.com/devr-tools/codeguard/commit/b93d6819a936b11b18fdca5493bb181ef2e6edc1))
* **cpp:** expand checks across all categories ([29ea77e](https://github.com/devr-tools/codeguard/commit/29ea77ec7ad575e9bd875b2387ec655dac437631))
* **cpp:** expand coverage with tooling, security, and dependency analysis ([#50](https://github.com/devr-tools/codeguard/issues/50)) ([16e8146](https://github.com/devr-tools/codeguard/commit/16e8146820aede9480b2358ee9c567f229b30697))

## [1.0.0](https://github.com/devr-tools/codeguard/compare/v0.8.3...v1.0.0) (2026-07-16)


### ⚠ BREAKING CHANGES

* promote performance rules to their own opt-in check section ([#41](https://github.com/devr-tools/codeguard/issues/41))
* the performance rules' ids renamed from quality.* to performance.* (quality.n-plus-one-query -> performance.n-plus-one-query, quality.go.alloc-in-loop -> performance.go.alloc-in-loop, quality.sync-io-in-request-path -> performance.sync-io-in-request-path, quality.unbounded-goroutines-in-loop -> performance.unbounded-goroutines-in-loop, plus the typescript/javascript mirrors and quality.python.sync-io-in-async). Their detect_* toggles moved from quality_rules to performance_rules, and the section is opt-in: set checks.performance: true to keep running these rules, and update any waivers or baselines that reference the old ids.

### Features

* 30% faster scans, enforceable AI-readiness score, and doc-truth rules ([#44](https://github.com/devr-tools/codeguard/issues/44)) ([718dd08](https://github.com/devr-tools/codeguard/commit/718dd085f2ecf11bf663901f0ecb057cf962d133))
* **context:** add AI-and-human-readiness rules and broaden README drift ([6caac9a](https://github.com/devr-tools/codeguard/commit/6caac9a0a1f687c18a280a238b43d77cf834cd23))
* **context:** AI-readiness gate, doc-truth rules, and 30% faster scans ([1f4f130](https://github.com/devr-tools/codeguard/commit/1f4f130ac7e309ba97e737a457f0687b067569b0))
* **context:** AI-readiness gate, doc-truth rules, and 30% faster scans (re-land [#44](https://github.com/devr-tools/codeguard/issues/44)) ([#46](https://github.com/devr-tools/codeguard/issues/46)) ([75b3f02](https://github.com/devr-tools/codeguard/commit/75b3f02c5cfca86eb38d12b8a5c8e7730d733e5b))
* **context:** enforceable legibility threshold and recalibrated score components ([604dd52](https://github.com/devr-tools/codeguard/commit/604dd52cf9503d1db43b34d094f97584c64d9e52))
* **context:** persist repo_legibility score history with report flag ([e54f643](https://github.com/devr-tools/codeguard/commit/e54f6430a3f8312f7ab2a4e108f29607a6b9b6fd))
* **parsers:** wire the Python tree-sitter grammar and upgrade Python N+1 detection ([07a0b24](https://github.com/devr-tools/codeguard/commit/07a0b24c487feead16df877723e9839736f0926a))
* **performance:** add AI-assisted semantic performance lens ([eabd7fe](https://github.com/devr-tools/codeguard/commit/eabd7feb33dcc1d10c02b3798f1a90f434c88b7e))
* **performance:** add diff-only loop-nesting complexity regression rule ([2e8bcef](https://github.com/devr-tools/codeguard/commit/2e8bcef06e819083a569338dee90a3d15a40c57e))
* **performance:** add framework-aware rules for Django, SQLAlchemy, React, and Express ([b49ce7b](https://github.com/devr-tools/codeguard/commit/b49ce7b11801509711972eedd29d1fa2084f1f03))
* **performance:** add loop-hygiene, concurrency, and memory-pressure rules ([54bfca9](https://github.com/devr-tools/codeguard/commit/54bfca96401947cae75ad29ade4dfc904508d126))
* **performance:** add measured budgets and benchmark regression gates ([2c5e4ba](https://github.com/devr-tools/codeguard/commit/2c5e4ba62fc7d928f4e025878078f8d9a4a043ea))
* **performance:** publish a performance_score artifact with history and report flag ([5c5a738](https://github.com/devr-tools/codeguard/commit/5c5a73872340609fa1e13c21bcf11a963e5342c0))
* promote performance rules to their own opt-in check section ([ede8b37](https://github.com/devr-tools/codeguard/commit/ede8b37e4a6ddb2b467bfbe50bc611fa648fd3c6))
* promote performance rules to their own opt-in check section ([#41](https://github.com/devr-tools/codeguard/issues/41)) ([81a575b](https://github.com/devr-tools/codeguard/commit/81a575b60c7404dbec1db45240557112187103bf))
* suggest enabling the performance section in scan output ([81d9d46](https://github.com/devr-tools/codeguard/commit/81d9d46e2f11e0913b8fd8e66d984020aa582476))


### Performance Improvements

* **quality:** hash clone tokens once and roll the window hash ([bbf5b69](https://github.com/devr-tools/codeguard/commit/bbf5b69147e09f77e037a830c834091f95834d80))
* **quality:** route AI check reads through the shared scan corpus ([b271cfb](https://github.com/devr-tools/codeguard/commit/b271cfb6abde0b34378ea7fb63e2bea33730f709))
* **runner:** count lines by scanning bytes instead of allocating ([17a4593](https://github.com/devr-tools/codeguard/commit/17a45937f071d67cde965b454c3e3000df6d0c40))

## [0.8.3](https://github.com/devr-tools/codeguard/compare/v0.8.2...v0.8.3) (2026-07-16)


### Bug Fixes

* **ci:** pin cosign to the v2 line so release signing keeps the .sig/.pem contract ([9ee93c4](https://github.com/devr-tools/codeguard/commit/9ee93c49f54d6d596ed2cbf62bef8f5bc14561b8))
* **ci:** pin cosign to v2 line to unbreak release signing ([#39](https://github.com/devr-tools/codeguard/issues/39)) ([218f078](https://github.com/devr-tools/codeguard/commit/218f078a1162fc3f2480f7e672de971d2aa72057))

## [0.8.2](https://github.com/devr-tools/codeguard/compare/v0.8.1...v0.8.2) (2026-07-16)


### Bug Fixes

* **quality:** keep exec error in coverage-command failure chain ([0666e42](https://github.com/devr-tools/codeguard/commit/0666e42bfcf237310c33dd0a79a23fd50df25b17))
* **security:** surface invalid secret patterns instead of silently dropping them ([5445fde](https://github.com/devr-tools/codeguard/commit/5445fdeb8da732682a9f7f6cccd62ed648eb7ac2))

## [0.8.1](https://github.com/devr-tools/codeguard/compare/v0.8.0...v0.8.1) (2026-07-03)


### Bug Fixes

* **release:** disable PyPI attestations for reusable-workflow publish ([28a147d](https://github.com/devr-tools/codeguard/commit/28a147d14834ae20cf59749fcd6f21f51f02a568))

## [0.8.0](https://github.com/devr-tools/codeguard/compare/v0.7.0...v0.8.0) (2026-07-03)


### Features

* **packaging:** publish codeguard to npm and PyPI ([6a03f0e](https://github.com/devr-tools/codeguard/commit/6a03f0ef2f5a45dd7ffa3b46075e32c6601f997a))
* **packaging:** publish codeguard to npm and PyPI ([#29](https://github.com/devr-tools/codeguard/issues/29)) ([bf39a7a](https://github.com/devr-tools/codeguard/commit/bf39a7a53aea44545a09ec93e3feeead87b707b7))

## [0.7.0](https://github.com/devr-tools/codeguard/compare/v0.6.1...v0.7.0) (2026-07-02)


### Features

* **cli:** add What's New banner and task-grouped menu ([c1df999](https://github.com/devr-tools/codeguard/commit/c1df9990f0255dcadc09aa459da1a7278a06e4df))
* **cli:** What's New banner and task-grouped menu ([#21](https://github.com/devr-tools/codeguard/issues/21)) ([45e56f7](https://github.com/devr-tools/codeguard/commit/45e56f7bcecf1a9edd7d3dd02ca515df58482f1a))
* execute checks-improvement plan tiers 1-6 ([733862f](https://github.com/devr-tools/codeguard/commit/733862f22781b9b1ed32f16c2993f8c92d7e353c))
* **parsers:** tree-sitter TypeScript engine behind parsers.treesitter flag ([5d09161](https://github.com/devr-tools/codeguard/commit/5d09161c908437341994ece22841726da8aacaf4))


### Bug Fixes

* **ci:** extract version from var Number in homebrew validation ([01d7a24](https://github.com/devr-tools/codeguard/commit/01d7a24593da880123f0a50a7c3868467a58b4e0))
* **ci:** extract version from var Number in homebrew validation ([#23](https://github.com/devr-tools/codeguard/issues/23)) ([ec21f3b](https://github.com/devr-tools/codeguard/commit/ec21f3b60404e13647855b5837a53ae7778ec268))
* **version:** make Number a var so ldflags injection works ([6b1ccad](https://github.com/devr-tools/codeguard/commit/6b1ccadbfb0e259d093a81900ccfd2603032e9e4))


### Performance Improvements

* **scan:** parse/read files once, parallelize sections, scope cache ([d181a4f](https://github.com/devr-tools/codeguard/commit/d181a4fc75ec5ffaac62ce867f244f607b514b43))
* **scan:** parse/read files once, parallelize sections, scope cache ([#24](https://github.com/devr-tools/codeguard/issues/24)) ([c79d9ec](https://github.com/devr-tools/codeguard/commit/c79d9ec1bfcaf036466ba588b19c974c06238f95))

## [0.6.1](https://github.com/devr-tools/codeguard/compare/v0.6.0...v0.6.1) (2026-07-01)


### Bug Fixes

* **security:** harden untrusted-input handling and wire lint cleanup ([34c7f87](https://github.com/devr-tools/codeguard/commit/34c7f87c8f6d0bf55917abcb3f5a05e81fcf3669))

## [0.6.0](https://github.com/devr-tools/codeguard/compare/v0.5.0...v0.6.0) (2026-06-30)


### Features

* **security:** hardcoded secret/credential detection + git-history scan ([f2f6c61](https://github.com/devr-tools/codeguard/commit/f2f6c61e171c46eb8af7b554e506c719a077aaf2))
* **security:** hardcoded secret/credential detection + git-history scan ([#17](https://github.com/devr-tools/codeguard/issues/17)) ([c1afab7](https://github.com/devr-tools/codeguard/commit/c1afab751ac0d7d5270ae78bd6936c590fe4a2c5))


### Bug Fixes

* **security:** anchor Slack webhook regex to satisfy CodeQL ([52de05a](https://github.com/devr-tools/codeguard/commit/52de05aec540115392936e6b2caacf91fa4ee466))

## [0.5.0](https://github.com/devr-tools/codeguard/compare/v0.4.0...v0.5.0) (2026-06-19)


### Features

* mcp server features ([2efd47c](https://github.com/devr-tools/codeguard/commit/2efd47c9bd701f5508bb53f33ce3e5af37d0b03d))
* **mcp:** HTTP transport, Devin pack, verified fix, sampling/roots ([#15](https://github.com/devr-tools/codeguard/issues/15)) ([44de1bb](https://github.com/devr-tools/codeguard/commit/44de1bba2adfe040536f2178f6015bc744d4c72e))
* **mcp:** HTTP transport, Devin pack, verified fix, sampling/roots, streaming ([0bfab1d](https://github.com/devr-tools/codeguard/commit/0bfab1d1955fb5010addcf4541b3c82f37c809d5))

## [0.4.0](https://github.com/devr-tools/codeguard/compare/v0.3.0...v0.4.0) (2026-06-18)


### Features

* add OWASP-aware security metadata, trust-policy enforcement ([#10](https://github.com/devr-tools/codeguard/issues/10)) ([358d4ba](https://github.com/devr-tools/codeguard/commit/358d4baf3d07eb87e5d1bce24dcd2384403a3db8))
* feat: add OWASP-aware security metadata, trust-policy enforcement, and quality-check cleanup ([9111a58](https://github.com/devr-tools/codeguard/commit/9111a586c5f006ce8d80a0eccbdcc30fbc324ea4))
* fix cd push security updates ([685a3ff](https://github.com/devr-tools/codeguard/commit/685a3ff4ecce755f25a5d9889ebfee277f8e39d0))

## [0.3.0](https://github.com/devr-tools/codeguard/compare/v0.2.0...v0.3.0) (2026-06-17)


### Features

* add supply chain policies, semantic review, and AI change risk ([9b2dc1f](https://github.com/devr-tools/codeguard/commit/9b2dc1f25893a98e33cdd2d5336d0dd690b67387))
* add supply chain policies, semantic review, and AI change risk ([#8](https://github.com/devr-tools/codeguard/issues/8)) ([3ae9728](https://github.com/devr-tools/codeguard/commit/3ae97289ff8003eb3c071c2ad5697858f2c58419))

## [0.2.0](https://github.com/devr-tools/codeguard/compare/v0.1.0...v0.2.0) (2026-06-16)


### Features

* add contracts check family for API/contract drift detection ([1406bd5](https://github.com/devr-tools/codeguard/commit/1406bd5f4a68b1d7a469c325e79fde6fb536b875))
* feat(inital release) ([b6620e3](https://github.com/devr-tools/codeguard/commit/b6620e3024ffa5326eb06a612b5804270ce8250c))
* **inital release:** initial release ([f7cc9cd](https://github.com/devr-tools/codeguard/commit/f7cc9cdfa030d42666c3b60f886b8938751fd7cd))
* **parsers,security:** deepen pure-Go parsers and add Go/Python taint analysis ([9f42f4a](https://github.com/devr-tools/codeguard/commit/9f42f4a75d9916b0eff040b57edafc09af7b1c64))
* **typescript:** cross-module taint analysis with function summaries ([d8fc21c](https://github.com/devr-tools/codeguard/commit/d8fc21c56f9290e6a23955816bf7a534a118fc14))

## Changelog
