package version

import (
	"runtime/debug"
	"strings"
)

const defaultNumber = "0.1.0"

// Number is the codeguard version. It must be a var (not a const) so the
// release build can override it via the linker: GoReleaser injects the git tag
// with `-X github.com/devr-tools/codeguard/internal/version.Number=v{{.Version}}`
// (see .goreleaser.yaml). The linker's -X flag only sets string vars, so a
// const would silently leave released binaries reporting this default. Version
// precedence is linker flags, embedded build info, then the compiled default.
var Number = defaultNumber

func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	Number = Resolve(Number, info)
}

// Resolve preserves an injected release version before consulting build info.
func Resolve(current string, info *debug.BuildInfo) string {
	if current != defaultNumber {
		return current
	}
	if moduleVersion := ModuleVersionFromBuildInfo(info); moduleVersion != "" {
		return moduleVersion
	}
	if developmentVersion := DevelopmentVersionFromBuildInfo(info); developmentVersion != "" {
		return developmentVersion
	}
	return current
}

// ModuleVersionFromBuildInfo returns a real module version embedded by Go.
func ModuleVersionFromBuildInfo(info *debug.BuildInfo) string {
	if info == nil {
		return ""
	}
	moduleVersion := strings.TrimSpace(info.Main.Version)
	if len(moduleVersion) < 2 || moduleVersion[0] != 'v' || moduleVersion[1] < '0' || moduleVersion[1] > '9' {
		return ""
	}
	return moduleVersion
}

// DevelopmentVersionFromBuildInfo identifies local source builds when Go
// embeds VCS settings but no module version.
func DevelopmentVersionFromBuildInfo(info *debug.BuildInfo) string {
	if info == nil || strings.TrimSpace(info.Main.Version) != "(devel)" {
		return ""
	}
	var revision string
	dirty := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = strings.TrimSpace(setting.Value)
		case "vcs.modified":
			dirty = setting.Value == "true"
		}
	}
	if revision == "" {
		return ""
	}
	if len(revision) > 8 {
		revision = revision[:8]
	}
	resolved := defaultNumber + "-dev+" + revision
	if dirty {
		resolved += ".dirty"
	}
	return resolved
}
