package support

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/cachefile"
)

func derivedCachePath(base string, suffix string) string {
	trimmed := strings.TrimSpace(base)
	if trimmed == "" {
		return ""
	}
	ext := filepath.Ext(trimmed)
	if ext == "" {
		return trimmed + suffix
	}
	return strings.TrimSuffix(trimmed, ext) + suffix + ext
}

func writeHistoryFile(path string, payload any) {
	_ = cachefile.Write(path, payload)
}
