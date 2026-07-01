package version

// Number is the codeguard version. It must be a var (not a const) so the
// release build can override it via the linker: GoReleaser injects the git tag
// with `-X github.com/devr-tools/codeguard/internal/version.Number=v{{.Version}}`
// (see .goreleaser.yaml). The linker's -X flag only sets string vars, so a
// const would silently leave released binaries reporting this default.
var Number = "0.1.0"
