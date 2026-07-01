package cli

import (
	"context"
	"io"

	root "github.com/devr-tools/codeguard"
	"github.com/devr-tools/codeguard/internal/version"
	"github.com/devr-tools/codeguard/internal/whatsnew"
)

// writeWhatsNew renders the "What's New" banner above the usage menu: the
// running version, an upstream update notice (from a cached, opt-out release
// check), and the latest changelog highlights. Any failure degrades to showing
// less, never blocking the menu.
func writeWhatsNew(stdout io.Writer) {
	rel, _ := whatsnew.LatestFromChangelog(root.Changelog)
	latest, _ := whatsnew.DefaultChecker().LatestVersion(context.Background())
	whatsnew.Render(stdout, version.Number, latest, rel, whatsnew.ColorForWriter(stdout))
}
