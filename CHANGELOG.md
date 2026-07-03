# Changelog

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
