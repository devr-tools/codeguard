package prompts

import (
	"regexp"
	"strings"
)

var (
	mcpPipeToShell = regexp.MustCompile(`\b(?:curl|wget)\b[^|]*\|\s*(?:sudo\s+)?(?:ba|z|da)?sh\b`)
	exactSemver    = regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.\-]+)?$`)
	commitRef      = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	imageDigest    = regexp.MustCompile(`@sha256:[0-9a-f]{64}$`)
)

// packageLaunch describes a server command that downloads and runs a package
// from a registry at launch time.
type packageLaunch struct {
	runner          string
	spec            string
	pinned          bool
	floatingTag     bool
	autoInstall     bool
	autoInstallFlag string
}

// launcherKind groups registry launchers by how they name a package version.
type launcherKind int

const (
	launcherNode launcherKind = iota + 1
	launcherPython
	launcherContainer
)

// npmValueFlags, pythonValueFlags and containerValueFlags take a separate
// value token, which must be skipped to find the package argument.
var (
	npmValueFlags       = map[string]struct{}{"--registry": {}, "--cache": {}, "--userconfig": {}, "-c": {}, "--call": {}}
	pythonValueFlags    = map[string]struct{}{"--with": {}, "--python": {}, "-p": {}, "--index-url": {}, "--index": {}, "--extra-index-url": {}, "--python-version": {}, "-w": {}}
	containerValueFlags = map[string]struct{}{
		"-e": {}, "--env": {}, "--env-file": {}, "-v": {}, "--volume": {}, "--mount": {}, "--name": {}, "--network": {},
		"-p": {}, "--publish": {}, "-u": {}, "--user": {}, "-w": {}, "--workdir": {}, "--entrypoint": {}, "--platform": {},
		"--pull": {}, "-l": {}, "--label": {}, "--cap-add": {}, "--cap-drop": {}, "-h": {}, "--hostname": {}, "--memory": {}, "-m": {},
	}
)

// detectPackageLaunch recognizes npx/bunx/pnpm dlx/yarn dlx/npm exec, uvx/uv tool
// run/pipx run, and docker/podman run, and reports whether the package or
// image they fetch is pinned to an immutable version.
func detectPackageLaunch(tokens []string) (packageLaunch, bool) {
	if len(tokens) == 0 {
		return packageLaunch{}, false
	}
	runner, kind, rest := classifyLauncher(tokens)
	if kind == 0 {
		return packageLaunch{}, false
	}
	launch := packageLaunch{runner: runner}
	var spec string
	switch kind {
	case launcherNode:
		spec = nodePackageArg(rest, &launch)
	case launcherPython:
		spec = pythonPackageArg(rest)
	case launcherContainer:
		spec = positionalArg(rest, containerValueFlags)
	}
	if spec == "" || isLocalPath(spec) {
		return packageLaunch{}, false
	}
	launch.spec = spec
	switch kind {
	case launcherNode:
		launch.pinned, launch.floatingTag = nodeSpecPinned(spec)
	case launcherPython:
		launch.pinned, launch.floatingTag = pythonSpecPinned(spec)
	case launcherContainer:
		launch.pinned, launch.floatingTag = imageSpecPinned(spec)
	}
	return launch, true
}

func classifyLauncher(tokens []string) (string, launcherKind, []string) {
	base := commandBase(tokens[0])
	rest := tokens[1:]
	switch base {
	case "npx", "bunx", "pnpx":
		return base, launcherNode, rest
	case "pnpm", "yarn", "bun":
		if len(rest) > 0 && (rest[0] == "dlx" || (base == "bun" && rest[0] == "x")) {
			return base + " " + rest[0], launcherNode, rest[1:]
		}
	case "npm":
		if len(rest) > 0 && (rest[0] == "exec" || rest[0] == "x") {
			return "npm " + rest[0], launcherNode, rest[1:]
		}
	case "uvx":
		return base, launcherPython, rest
	case "uv":
		if len(rest) > 1 && rest[0] == "tool" && rest[1] == "run" {
			return "uv tool run", launcherPython, rest[2:]
		}
	case "pipx":
		if len(rest) > 0 && rest[0] == "run" {
			return "pipx run", launcherPython, rest[1:]
		}
	case "docker", "podman":
		if len(rest) > 0 && rest[0] == "run" {
			return base + " run", launcherContainer, rest[1:]
		}
	}
	return "", 0, nil
}

func nodePackageArg(args []string, launch *packageLaunch) string {
	explicit := ""
	for idx := 0; idx < len(args); idx++ {
		arg := args[idx]
		switch {
		case arg == "-y" || arg == "--yes":
			launch.autoInstall, launch.autoInstallFlag = true, arg
		case arg == "-p" || arg == "--package":
			if idx+1 < len(args) && explicit == "" {
				explicit = args[idx+1]
			}
			idx++
		case strings.HasPrefix(arg, "--package="):
			if explicit == "" {
				explicit = strings.TrimPrefix(arg, "--package=")
			}
		case arg == "--":
			if explicit != "" {
				return explicit
			}
			return positionalArg(args[idx+1:], npmValueFlags)
		case strings.HasPrefix(arg, "-"):
			if _, takesValue := npmValueFlags[arg]; takesValue {
				idx++
			}
		default:
			if explicit != "" {
				return explicit
			}
			return arg
		}
	}
	return explicit
}

func pythonPackageArg(args []string) string {
	for idx := 0; idx < len(args); idx++ {
		arg := args[idx]
		switch {
		case arg == "--from" || arg == "--spec":
			if idx+1 < len(args) {
				return args[idx+1]
			}
			return ""
		case strings.HasPrefix(arg, "--from=") || strings.HasPrefix(arg, "--spec="):
			return arg[strings.IndexByte(arg, '=')+1:]
		case strings.HasPrefix(arg, "-"):
			if _, takesValue := pythonValueFlags[arg]; takesValue {
				idx++
			}
		default:
			return arg
		}
	}
	return ""
}

func positionalArg(args []string, valueFlags map[string]struct{}) string {
	for idx := 0; idx < len(args); idx++ {
		arg := args[idx]
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
		if _, takesValue := valueFlags[arg]; takesValue {
			idx++
		}
	}
	return ""
}

func isLocalPath(spec string) bool {
	return strings.HasPrefix(spec, ".") || strings.HasPrefix(spec, "/") || strings.HasPrefix(spec, "~") ||
		strings.HasPrefix(spec, "file:") || strings.Contains(spec, "${") || (len(spec) > 2 && spec[1] == ':' && (spec[2] == '\\' || spec[2] == '/'))
}

// nodeSpecPinned reports whether an npm package spec names an exact version
// (name@1.2.3) or a commit (github:org/repo#<sha>). The second result marks
// an explicit floating dist-tag such as @latest or @next.
func nodeSpecPinned(spec string) (bool, bool) {
	if hash := strings.LastIndexByte(spec, '#'); hash >= 0 {
		return commitRef.MatchString(spec[hash+1:]), false
	}
	at := strings.LastIndexByte(spec, '@')
	if at <= 0 {
		return false, false
	}
	version := spec[at+1:]
	if exactSemver.MatchString(version) {
		return true, false
	}
	return false, isFloatingTag(version)
}

// pythonSpecPinned accepts name==1.2.3, uvx's name@1.2.3, and VCS URLs
// pinned to a commit.
func pythonSpecPinned(spec string) (bool, bool) {
	if strings.HasPrefix(spec, "git+") {
		at := strings.LastIndexByte(spec, '@')
		return at > 0 && commitRef.MatchString(spec[at+1:]), false
	}
	if idx := strings.Index(spec, "==="); idx > 0 {
		return spec[idx+3:] != "", false
	}
	if idx := strings.Index(spec, "=="); idx > 0 {
		return !strings.ContainsAny(spec[idx+2:], "*,"), false
	}
	if at := strings.LastIndexByte(spec, '@'); at > 0 {
		version := spec[at+1:]
		return exactSemver.MatchString(version) || pep440Exact(version), isFloatingTag(version)
	}
	return false, false
}

func pep440Exact(version string) bool {
	if version == "" || strings.ContainsAny(version, "*<>=~^, ") {
		return false
	}
	return version[0] >= '0' && version[0] <= '9'
}

// imageSpecPinned accepts a digest-pinned image or an explicit non-floating
// tag; a bare image name is implicitly :latest.
func imageSpecPinned(spec string) (bool, bool) {
	if imageDigest.MatchString(spec) {
		return true, false
	}
	lastSegment := spec[strings.LastIndexByte(spec, '/')+1:]
	colon := strings.LastIndexByte(lastSegment, ':')
	if colon < 0 {
		return false, true
	}
	tag := lastSegment[colon+1:]
	if isFloatingTag(tag) {
		return false, true
	}
	return true, false
}

func isFloatingTag(version string) bool {
	switch strings.ToLower(version) {
	case "latest", "next", "canary", "nightly", "beta", "alpha", "edge", "main", "master", "stable", "*", "x":
		return true
	}
	return false
}
