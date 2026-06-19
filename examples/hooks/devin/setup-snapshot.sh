#!/bin/sh

# Install the codeguard binary into a Devin machine snapshot for STDIO mode
# (Option B). Run this from Devin's machine setup so `codeguard` is on PATH in
# every session.
#
# Environment:
#   CODEGUARD_VERSION  version/ref to install (default: latest)
#   GOBIN              install target for `go install` (default: Go's default)

set -eu

version="${CODEGUARD_VERSION:-latest}"

if command -v codeguard >/dev/null 2>&1; then
	echo "setup-snapshot.sh: codeguard already on PATH ($(command -v codeguard))"
	codeguard version || true
	exit 0
fi

if command -v go >/dev/null 2>&1; then
	echo "setup-snapshot.sh: installing codeguard@${version} via go install"
	go install "github.com/devr-tools/codeguard/cmd/codeguard@${version}"
	# Ensure the Go bin dir is on PATH for subsequent steps.
	gobin="${GOBIN:-$(go env GOPATH)/bin}"
	case ":$PATH:" in
		*":$gobin:"*) ;;
		*) echo "setup-snapshot.sh: add $gobin to PATH (e.g. echo 'export PATH=\"$gobin:\$PATH\"' >> ~/.profile)" >&2 ;;
	esac
	exit 0
fi

echo "setup-snapshot.sh: Go toolchain not found." >&2
echo "Install Go, or download a release binary from" >&2
echo "  https://github.com/devr-tools/codeguard/releases" >&2
echo "and place 'codeguard' on PATH." >&2
exit 1
