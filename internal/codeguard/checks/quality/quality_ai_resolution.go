package quality

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const maxGitHeadMessageBytes = 1 << 20

var errGitHeadMessageTooLarge = errors.New("git HEAD message exceeds size limit")

type packageManifest struct {
	Name             string            `json:"name"`
	Dependencies     map[string]string `json:"dependencies"`
	DevDependencies  map[string]string `json:"devDependencies"`
	PeerDependencies map[string]string `json:"peerDependencies"`
}

// readPackageManifest reads the root package.json directly rather than through
// the corpus: the fixed filename is not walk-enumerated, so a direct uncapped
// read preserves the historical behavior.
func readPackageManifest(root string) (packageManifest, bool) {
	data, err := os.ReadFile(filepath.Join(root, "package.json")) //nolint:gosec // fixed filename under the scan-target root
	if err != nil {
		return packageManifest{}, false
	}
	return parsePackageManifest(data)
}

func parsePackageManifest(data []byte) (packageManifest, bool) {
	var manifest packageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return packageManifest{}, false
	}
	return manifest, true
}

func packageManifestDeps(manifest packageManifest) map[string]struct{} {
	deps := map[string]struct{}{}
	for name := range manifest.Dependencies {
		deps[name] = struct{}{}
	}
	for name := range manifest.DevDependencies {
		deps[name] = struct{}{}
	}
	for name := range manifest.PeerDependencies {
		deps[name] = struct{}{}
	}
	if strings.TrimSpace(manifest.Name) != "" {
		deps[strings.TrimSpace(manifest.Name)] = struct{}{}
	}
	return deps
}

func readGitHeadMessage(dir string) string {
	// TODO(harden): thread caller ctx once readGitHeadMessage accepts one.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "log", "-1", "--format=%B") //nolint:gosec // fixed git subcommand; dir is a config-supplied scan target path
	out := limitedBuffer{limit: maxGitHeadMessageBytes}
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return out.String()
}

// limitedBuffer prevents os/exec from buffering attacker-controlled command
// output without bound. Returning an error also stops the stdout copy once the
// limit is reached.
type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - b.Len()
	if remaining <= 0 {
		return 0, errGitHeadMessageTooLarge
	}
	if len(p) > remaining {
		n, _ := b.Buffer.Write(p[:remaining])
		return n, errGitHeadMessageTooLarge
	}
	return b.Buffer.Write(p)
}

func envFlagEnabled(keys []string) bool {
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			continue
		}
		switch strings.ToLower(value) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

func hasCommitTrailer(message string, trailers []string) bool {
	lowerMessage := strings.ToLower(message)
	for _, trailer := range trailers {
		if strings.Contains(lowerMessage, strings.ToLower(strings.TrimSpace(trailer))+":") {
			return true
		}
	}
	return false
}

func packageRoot(specifier string) string {
	if strings.HasPrefix(specifier, "@") {
		parts := strings.Split(specifier, "/")
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
	}
	return firstSegment(specifier)
}

func isNodeBuiltinPackage(root string) bool {
	if strings.HasPrefix(root, "node:") {
		return true
	}
	return slices.Contains([]string{
		"assert", "buffer", "child_process", "crypto", "events", "fs", "http",
		"https", "net", "os", "path", "stream", "timers", "url", "util", "zlib",
	}, root)
}
