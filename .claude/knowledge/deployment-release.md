# Deployment & Release

How code gets to production. Release processes, environment promotion, rollback procedures, gotchas.

## Release flow

- Stable releases: a push to `main` runs `cd.yml` → `release-please`. Merging the release PR triggers the `stable-release` job, which calls the reusable `release.yml` (`workflow_call`). Prereleases run through `cd.yml`'s `prepare-prerelease`. Manual stable runs are blocked (`guard-manual-stable`).
- **Never add an `on: push: tags` trigger to `release.yml`.** It would let a tag push bypass the release-please approval flow.
- GoReleaser builds darwin and linux on amd64/arm64 only (no Windows). Archives are named `codeguard_v<version>_<os>_<arch>.tar.gz`, with a literal `v`.
- **Keep cosign pinned to v2** (`cosign-release: v2.6.3` in `release.yml`). cosign v3 uses the bundle format, ignores `.goreleaser.yaml`'s `--output-signature`/`--output-certificate`, and fails the release. The `.sig`/`.pem` outputs are a documented contract in `docs/security.md`. Migrating means changing `.goreleaser.yaml` and the docs together.

## npm + PyPI (packaging/)

- The wrappers ship the prebuilt GoReleaser binaries (`packaging/extract-binaries.sh`, `npm/build.sh`, `pypi/build_wheels.py`). See `packaging/README.md`.
- **`publish-npm`/`publish-pypi` must stay in `cd.yml`, not the reusable `release.yml`.** Both trusted publishers are registered against `cd.yml`. Inside a reusable workflow, PyPI auth matches `job_workflow_ref` (release.yml) while the PEP 740 attestation uses `workflow_ref` (cd.yml), so no single publisher satisfies both. On `invalid-publisher` or `400 Invalid attestations`, compare the OIDC claims the action prints.
- The PyPI project is **`devr-codeguard`**; the npm package is `@devr-tools/codeguard`. Both install a `codeguard` command.

## Local release-readiness check

- Reproduce the release binary: `CGO_ENABLED=0 GOOS=<os> GOARCH=<arch> go build -trimpath -tags=$(GRAMMAR_TAGS) -ldflags "-s -w -X github.com/devr-tools/codeguard/internal/version.Number=v<ver>" ./cmd/codeguard`. Signing and SBOMs only run in CI.
- Wrappers from local binaries: stage them as `<staging>/<goos>_<goarch>/codeguard`, then run `npm/build.sh <ver> <staging>` and `python3 pypi/build_wheels.py <ver> <staging> <out>`. Install the result and run `codeguard version`.
- MCP smoke test: use `codeguard serve --mcp`, since plain `serve` exits. macOS has no `timeout` binary.
- Upgrade test: build the previous tag from `git archive vX.Y.Z` with the same flags, write a config and baseline with it, then scan with both binaries. New detections can turn a baselined repo red; call that out in release notes.
