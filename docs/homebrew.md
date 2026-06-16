# Homebrew Packaging

`codeguard` now has two Homebrew-related paths aligned with `devr-tools/cleanr`:

- stable-release automation that updates `Formula/codeguard.rb` in `devr-tools/homebrew-tap`
- a pull-request validation workflow at `.github/workflows/homebrew-validation.yml`

## Tap Sync

The stable release workflow updates the tap repository by:

- downloading the tagged GitHub source tarball
- computing its SHA256
- patching `Formula/codeguard.rb` in `devr-tools/homebrew-tap`
- pushing an automation branch when `RELEASE_PLEASE_TOKEN` is configured
- opening or updating the matching pull request automatically

If the token is unavailable, the push fails, or PR creation fails, the workflow writes manual follow-up instructions to the GitHub Actions summary.

That automation assumes the tap repository already contains `Formula/codeguard.rb`.

## Pull Request Validation

The Homebrew validation workflow checks packaging before merge by:

- cloning `devr-tools/homebrew-tap`
- replacing `Formula/codeguard.rb` in that temporary checkout with a source-build formula generated from the current checkout
- tapping the temporary clone as `devr-tools/tap`
- verifying `brew install --build-from-source devr-tools/tap/codeguard`
- verifying `brew test devr-tools/tap/codeguard` on Ubuntu and macOS

## Expected Submission Shape

When you are ready to ship a stable release:

1. Merge the release PR created by Release Please on `main`.
2. Let `.github/workflows/release.yml` publish the tag and assets.
3. Review the matching Homebrew tap PR if one is opened automatically.
4. If tap automation is skipped, apply the manual follow-up from the workflow summary.
