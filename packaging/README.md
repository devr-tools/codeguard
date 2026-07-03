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

The `publish-npm` and `publish-pypi` jobs live inside `release.yml` (where the
release binaries are built). The two registries match **different** OIDC claims
when that publish step runs inside a reusable workflow, so they need **different**
trusted-publisher workflow filenames:

| Registry | Matches claim | Configure workflow |
| --- | --- | --- |
| PyPI | `job_workflow_ref` — the file that runs the publish step | **`release.yml`** |
| npm  | `workflow_ref` — the top-level caller | **`cd.yml`** |

`id-token: write` is set on both the `cd.yml` caller jobs and the `release.yml`
publish jobs (both parent and child need it for OIDC).

> This is confirmed against the live warehouse source
> (`warehouse/oidc/models/github.py`), which extracts the workflow filename from
> the `job_workflow_ref` claim — i.e. `release.yml` here, not the `cd.yml` caller.
> An older PyPI docs page claimed reusable workflows were unsupported
> (warehouse#11096); that is stale — naming the reusable `release.yml` works.

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
   - Workflow filename: `release.yml`  ← the file that runs the publish step,
     NOT the `cd.yml` caller (PyPI matches `job_workflow_ref`)
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
