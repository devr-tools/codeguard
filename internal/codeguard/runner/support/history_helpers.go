package support

import (
	"github.com/devr-tools/codeguard/internal/codeguard/cachefile"
)

func derivedCachePath(base string, suffix string) string {
	return cachefile.DerivedPath(base, suffix)
}

func writeHistoryFile(path string, payload any) {
	_ = cachefile.Write(path, payload)
}
