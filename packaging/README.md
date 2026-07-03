# Packaging: npm and PyPI

`codeguard` is a Go binary released by GoReleaser to GitHub Releases. The npm and
PyPI packages here are thin wrappers that ship those **prebuilt binaries** — no
Go toolchain, and no network access, at install time.

| Registry | Install | Package(s) |
| --- | --- | --- |
| npm  | `npm install -g @devr-tools/codeguard` | `@devr-tools/codeguard` (launcher) + one `@devr-tools/codeguard-<os>-<cpu>` per platform |
| PyPI | `pip install devr-codeguard` | one `devr_codeguard-<ver>-py3-none-<platform>.whl` per platform |

Both are published automatically by `.github/workflows/release.yml` for **stable**
releases only (prereleases are skipped, matching the Homebrew job).

## How it works

- **npm** uses esbuild-style `optionalDependencies`. The `@devr-tools/codeguard`
  launcher declares one optional dependency per platform (each constrained by
  `os`/`cpu`). npm installs only the package matching the host, and the
  `bin/codeguard` launcher `require.resolve`s that package's binary and execs it.
- **PyPI** ships one wheel per platform. The project is named **`devr-codeguard`**
  (the plain `codeguard` name is already taken), but the installed command is
  still `codeguard`: each wheel carries the binary as a *data script*
  (`devr_codeguard-<ver>.data/scripts/codeguard`), so pip drops `codeguard`
  straight onto PATH. There is no Python module and no shim.

## Layout

```
packaging/
  extract-binaries.sh          # download + extract release binaries into .staging/
  npm/
    launcher/bin/codeguard     # the launcher shim (committed source of truth)
    launcher/README.md         # README published with the main npm package
    build.sh                   # generate dist/ from .staging/ + version
    bootstrap-publish.sh       # one-time manual publish to seed trusted publishing
  pypi/
    build_wheels.py            # generate dist/*.whl from .staging/ + version
```

## Build and test locally

```bash
# 1. Stage binaries from a published release (needs an authenticated gh).
./packaging/extract-binaries.sh v0.7.0 0.7.0

# 2. npm packages -> packaging/npm/dist/
./packaging/npm/build.sh 0.7.0

# 3. wheels -> packaging/pypi/dist/
python3 packaging/pypi/build_wheels.py 0.7.0 packaging/.staging packaging/pypi/dist
```

`npm publish --dry-run <dir>` and `pipx run twine check packaging/pypi/dist/*.whl`
validate the artifacts without publishing.

## Release flow and the reusable-workflow caveat

Unlike a repo that triggers `release.yml` directly on a tag push, codeguard's
`release.yml` is a **reusable workflow** (`workflow_call` only) invoked by
`cd.yml`:

```
push to main -> cd.yml (release-please) -> stable-release job -> release.yml
```

The `publish-npm` and `publish-pypi` jobs live in **`cd.yml`** (the top-level
entry workflow), NOT in the reusable `release.yml`. They run after the
`stable-release` job (which builds the binaries and cuts the GitHub release) and
pull those binaries back down to assemble the packages.

**Why cd.yml and not release.yml:** OIDC trusted publishing matches the workflow
filename, and PEP 740 attestations additionally embed it in the signing
certificate's Build Config URI. When a publish step runs inside a *reusable*
workflow, the two OIDC claims split — `workflow_ref` = the caller (`cd.yml`),
`job_workflow_ref` = the reusable file (`release.yml`) — and no single publisher
can satisfy both: auth matches `job_workflow_ref` (release.yml) but the
attestation's Build Config URI comes from `workflow_ref` (cd.yml), so PyPI
rejects the upload with `400 Invalid attestations`. Publishing from `cd.yml`
directly collapses both claims to `cd.yml`, so **one publisher (`cd.yml`) works
for both npm and PyPI, with attestations on.**

> This mirrors how szr works: szr's `release.yml` is triggered directly on the
> tag push (it *is* the entry workflow), so both claims are `release.yml` there.
> codeguard's `release.yml` is reusable-only, so the entry workflow is `cd.yml`.

Configure both trusted publishers with workflow filename **`cd.yml`**.
`id-token: write` is set on the `cd.yml` publish jobs.

## One-time prerequisites (before the first automated release)

These are external-registry setup steps the CI cannot perform for you. Both
registries use OIDC trusted publishing — no long-lived tokens live in CI.

1. **npm org + scope.** Create/own the `@devr-tools` npm org (or org scope) so
   the scoped packages can be published publicly.

2. **npm trusted publishing (OIDC).** npm requires each package to *already
   exist* before you can add a trusted publisher, and trusted publishers are
   configured **per package**. So:

   a. **Bootstrap once** — publish all five packages from your machine (needs
      `npm login` or a token in `~/.npmrc` with publish rights to
      `@devr-tools`):

      ```bash
      ./packaging/npm/bootstrap-publish.sh v0.7.0 0.7.0
      ```

   b. **Configure a trusted publisher** for each package on npmjs.com
      (package → Settings → Trusted Publisher → GitHub Actions):
      - Organization/user: `devr-tools`
      - Repository: `codeguard`
      - Workflow filename: `cd.yml`  ← the caller, not release.yml
      - Environment: *(leave blank)*
      - Allowed actions: `npm publish`

      Packages: `@devr-tools/codeguard` plus
      `@devr-tools/codeguard-{darwin-x64,darwin-arm64,linux-x64,linux-arm64}`.

   After that, the `publish-npm` job publishes via OIDC (`id-token: write`,
   npm ≥ 11.5.1 which the job installs). No `NPM_TOKEN` needed, and provenance
   is attached automatically.

3. **PyPI trusted publisher (OIDC).** On PyPI, add a *pending publisher* for
   project `devr-codeguard` (the plain `codeguard` name is taken; the installed
   command is still `codeguard`):
   - Owner / repo: `devr-tools/codeguard`
   - Workflow filename: `cd.yml`  ← the publish job lives in cd.yml (same as npm)
   - Environment: *(leave blank — the job sets none)*

   This lets the `publish-pypi` job authenticate via `id-token: write` with no
   long-lived token. (Alternatively, set a `PYPI_API_TOKEN` secret and pass it
   to `pypa/gh-action-pypi-publish` with `password:`.)

## Platform matrix

Keep this in sync with the `.goreleaser.yaml` build matrix. Currently:

| GOOS/GOARCH | npm package | wheel platform tag(s) |
| --- | --- | --- |
| darwin/amd64  | `@devr-tools/codeguard-darwin-x64`   | `macosx_10_9_x86_64` |
| darwin/arm64  | `@devr-tools/codeguard-darwin-arm64` | `macosx_11_0_arm64` |
| linux/amd64   | `@devr-tools/codeguard-linux-x64`    | `manylinux2014_x86_64`, `musllinux_1_1_x86_64` |
| linux/arm64   | `@devr-tools/codeguard-linux-arm64`  | `manylinux2014_aarch64`, `musllinux_1_1_aarch64` |

GoReleaser builds no Windows target, so there is no Windows wrapper.
