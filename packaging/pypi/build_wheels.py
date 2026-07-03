#!/usr/bin/env python3
"""Build per-platform devr-codeguard wheels from staged GoReleaser binaries.

Each wheel carries the prebuilt binary as a *data script* (installed into the
environment's bin/Scripts directory), so `pip install devr-codeguard` puts
`codeguard` on PATH with no Python shim and no build step. Uses only the
standard library.

The PyPI project is `devr-codeguard` (the plain `codeguard` name is taken), but
the installed command is still `codeguard` — the distribution name and the
data-script name are independent.

Usage: build_wheels.py <version> <staging_dir> <out_dir>

The staging layout matches packaging/extract-binaries.sh:
    <staging>/<goos>_<goarch>/codeguard
"""
import base64
import hashlib
import os
import stat
import sys
import zipfile

# PyPI project name (canonical, with hyphen) vs. the escaped form used in wheel
# filenames and dist-info/.data directory names (PEP 427: non-alphanumeric runs
# -> "_"). The installed executable stays `codeguard` (see BIN / TARGETS).
PROJECT = "devr-codeguard"
DIST = "devr_codeguard"
BIN = "codeguard"
SUMMARY = (
    "Repository checks across code quality, design boundaries, security, "
    "CI/CD hygiene, and AI prompt governance."
)
HOMEPAGE = "https://github.com/devr-tools/codeguard"
DESCRIPTION = (
    "# codeguard\n\n"
    "Repository checks across code quality, design boundaries, security, "
    "CI/CD hygiene, and AI prompt governance.\n\n"
    "```bash\n"
    "pip install devr-codeguard\n"
    "codeguard version\n"
    "```\n\n"
    "This wheel bundles the prebuilt `codeguard` binary for your platform and "
    "installs it onto your PATH as `codeguard` — no build step and no Go "
    "toolchain required. The PyPI project is `devr-codeguard`; the command is "
    "`codeguard`.\n\n"
    "Full documentation: https://github.com/devr-tools/codeguard\n"
)

# staging-subdir -> (binary-name, [wheel platform tags])
# Linux ships manylinux2014 + musllinux_1_1 (same static binary) so Alpine works.
TARGETS = {
    "darwin_amd64": ("codeguard", ["macosx_10_9_x86_64"]),
    "darwin_arm64": ("codeguard", ["macosx_11_0_arm64"]),
    "linux_amd64": ("codeguard", ["manylinux2014_x86_64", "musllinux_1_1_x86_64"]),
    "linux_arm64": ("codeguard", ["manylinux2014_aarch64", "musllinux_1_1_aarch64"]),
}


def record_hash(data: bytes) -> str:
    digest = hashlib.sha256(data).digest()
    b64 = base64.urlsafe_b64encode(digest).rstrip(b"=").decode("ascii")
    return f"sha256={b64}"


def build_wheel(version: str, binary: bytes, binname: str, plat_tag: str, out_dir: str) -> str:
    tag = f"py3-none-{plat_tag}"
    dist_info = f"{DIST}-{version}.dist-info"
    data_dir = f"{DIST}-{version}.data/scripts"

    metadata = (
        "Metadata-Version: 2.1\n"
        f"Name: {PROJECT}\n"
        f"Version: {version}\n"
        f"Summary: {SUMMARY}\n"
        f"Home-page: {HOMEPAGE}\n"
        "License: Apache-2.0\n"
        "Classifier: License :: OSI Approved :: Apache Software License\n"
        "Requires-Python: >=3.7\n"
        "Description-Content-Type: text/markdown\n"
        "\n"
        f"{DESCRIPTION}"
    )
    wheel_meta = (
        "Wheel-Version: 1.0\n"
        "Generator: codeguard build_wheels.py\n"
        "Root-Is-Purelib: false\n"
        f"Tag: {tag}\n"
    )

    script_path = f"{data_dir}/{binname}"
    records = []
    files = [
        (script_path, binary, True),
        (f"{dist_info}/METADATA", metadata.encode(), False),
        (f"{dist_info}/WHEEL", wheel_meta.encode(), False),
    ]

    os.makedirs(out_dir, exist_ok=True)
    wheel_name = f"{DIST}-{version}-{tag}.whl"
    wheel_path = os.path.join(out_dir, wheel_name)

    with zipfile.ZipFile(wheel_path, "w", zipfile.ZIP_DEFLATED) as zf:
        for arcname, data, executable in files:
            info = zipfile.ZipInfo(arcname)
            # Store a full Unix mode (regular-file type bits + permissions) so
            # pip installs the bundled binary with its executable bit intact.
            mode = stat.S_IFREG | (0o755 if executable else 0o644)
            info.external_attr = mode << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            zf.writestr(info, data)
            records.append(f"{arcname},{record_hash(data)},{len(data)}")

        record_arc = f"{dist_info}/RECORD"
        records.append(f"{record_arc},,")
        zf.writestr(record_arc, "\n".join(records) + "\n")

    return wheel_path


def main() -> int:
    if len(sys.argv) != 4:
        print(__doc__)
        return 2
    version, staging, out_dir = sys.argv[1], sys.argv[2], sys.argv[3]

    built = []
    for subdir, (binname, plat_tags) in TARGETS.items():
        src = os.path.join(staging, subdir, binname)
        if not os.path.isfile(src):
            print(f"missing staged binary: {src}", file=sys.stderr)
            return 1
        with open(src, "rb") as fh:
            binary = fh.read()
        for plat_tag in plat_tags:
            path = build_wheel(version, binary, binname, plat_tag, out_dir)
            built.append(path)
            print(f"built {os.path.basename(path)}")

    print(f"{len(built)} wheels written to {out_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
