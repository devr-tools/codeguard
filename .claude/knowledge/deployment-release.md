# Deployment & Release

How code gets to production. Release processes, environment promotion, rollback procedures, gotchas.

## Release flow

- Stable releases: push to `main` → `cd.yml` runs `release-please` → when a release PR merges, the `stable-release` job calls the **reusable** `release.yml` (`workflow_call`) with `prerelease: false`. Prereleases go through `cd.yml`'s `prepare-prerelease` → `release.yml` with `prerelease: true`.
- `release.yml` has **no `on: push: tags` trigger** — this is deliberate. Tags are minted only through the release-please approval flow (or a `create_missing_tag: true` call from the trusted `cd.yml` caller). Do not add a tag-push trigger; it would let anyone pushing a tag bypass the approval flow.
- GoReleaser (`.goreleaser.yaml`) builds only **darwin + linux on amd64/arm64** — no Windows. Archives are `tar.gz` named `codeguard_v<version>_<os>_<arch>.tar.gz` (note the literal `v` before the version).

## npm + PyPI packaging (packaging/)

- npm/PyPI ship thin wrappers around the prebuilt GoReleaser binaries — no Go toolchain at install time. `packaging/extract-binaries.sh` downloads release assets, `npm/build.sh` and `pypi/build_wheels.py` assemble artifacts. `publish-npm`/`publish-pypi` jobs in `release.yml` run for stable releases only. See `packaging/README.md`.
- **GOTCHA — trusted publishing + reusable workflow (npm and PyPI differ!):** the publish jobs live in the reusable `release.yml`, called by `cd.yml`. The two registries match *different* OIDC claims, so they need *different* trusted-publisher workflow filenames:
  - **PyPI → `release.yml`.** warehouse matches `job_workflow_ref` (the file that runs the publish step). Confirmed in `warehouse/oidc/models/github.py` — it extracts the workflow filename from `job_workflow_ref`. Setting the publisher to `cd.yml` produces `invalid-publisher` (`Publisher with matching claims was not found`). The old "reusable workflows unsupported / warehouse#11096" docs note is stale.
  - **npm → `cd.yml`.** npm matches `workflow_ref` (the top-level caller), per npm's docs.
  - `id-token: write` must be present on both the `cd.yml` caller jobs (it is) and the `release.yml` publish jobs.
  - Debug tip: on `invalid-publisher`, the pypi-publish action prints the actual claims; compare the `job_workflow_ref` filename to what's configured.
- First automated publish needs one-time manual bootstrap: `packaging/npm/bootstrap-publish.sh` (npm requires a package to exist before a trusted publisher can be added) and a PyPI *pending publisher* for project `devr-codeguard`. Both are documented step-by-step in `packaging/README.md`.
- **PyPI project name is `devr-codeguard`, NOT `codeguard`** (the plain name was already taken on PyPI). The npm scope `@devr-tools/codeguard` is unaffected. The installed command is still `codeguard` regardless — in `build_wheels.py`, `PROJECT`/`DIST` (distribution name, hyphen vs. PEP 427 escaped `devr_codeguard`) are decoupled from `BIN`/the data-script name (`codeguard`), so `pip install devr-codeguard` yields a `codeguard` command.
