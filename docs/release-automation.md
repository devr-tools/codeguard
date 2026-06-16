# Release Automation

This repository follows the same branch-driven release shape as `devr-tools/cleanr`.

## Workflows

### `.github/workflows/cd.yml`

Branch-driven CD entry point.

It currently:

- runs on pushes to `develop`, `master`, and `main`
- computes prerelease tags automatically for `develop`
- reuses `.github/workflows/release.yml` for prerelease packaging
- runs Release Please on `main` and `master`

### `.github/workflows/release.yml`

Reusable publisher invoked by CD or manual dispatch.

It currently:

- supports `workflow_dispatch` and `workflow_call`
- normalizes and validates tags before release
- runs GoReleaser using `.goreleaser.yaml`
- uploads release archives and checksums
- publishes a GHCR image for Linux `amd64` and `arm64`
- publishes a multi-arch GHCR manifest for the release tag
- syncs `Formula/codeguard.rb` in `devr-tools/homebrew-tap` for stable releases

### `.github/workflows/homebrew-validation.yml`

Pull request validation for the Homebrew packaging path.

It currently:

- runs on pull requests targeting `develop`, `master`, or `main`
- patches `Formula/codeguard.rb` in a temporary checkout of `devr-tools/homebrew-tap`
- builds `codeguard` from a source archive generated from the current checkout
- verifies `brew install --build-from-source devr-tools/tap/codeguard`
- verifies `brew test devr-tools/tap/codeguard`

## Release Please Files

Stable branch release preparation is driven by:

- `.github/release-please-config.json`
- `.release-please-manifest.json`
- `CHANGELOG.md`
- `internal/version/version.go`

## Required Secrets

- `GITHUB_TOKEN`: used by the release workflow for GitHub Releases and GHCR publishing
- `RELEASE_PLEASE_TOKEN`: used for Release Please PRs and Homebrew tap automation

## Published Outputs

Each tagged release currently publishes:

- `darwin/amd64` archive
- `darwin/arm64` archive
- `linux/amd64` archive
- `linux/arm64` archive
- `SHA256SUMS`
- `ghcr.io/devr-tools/codeguard:<tag>`

## Local Developer Commands

```bash
make build
make release
make release-check
```

## Related Docs

- [Getting started](getting-started.md)
- [Homebrew packaging](homebrew.md)
- [Docs index](README.md)
