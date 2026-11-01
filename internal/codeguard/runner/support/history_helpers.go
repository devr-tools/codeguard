package support

import (
	"github.com/devr-tools/codeguard/internal/codeguard/cachefile"
)

func derivedCachePath(base string, suffix string) string {
	return cachefile.DerivedPath(base, suffix)
}
