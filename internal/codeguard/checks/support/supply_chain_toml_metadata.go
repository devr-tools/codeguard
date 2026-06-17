package support

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func applyManifestIdentityLine(manifest *core.SupplyChainManifest, line string, lineNo int) {
	if manifest == nil {
		return
	}
	if strings.HasPrefix(line, "name") && strings.Contains(line, "=") {
		if values := quotedStringPattern.FindAllStringSubmatch(line, -1); len(values) > 0 {
			manifest.Name = strings.TrimSpace(values[0][1])
		}
	}
	if strings.HasPrefix(line, "license") && strings.Contains(line, "=") {
		manifest.License = parseTOMLLicenseValue(line)
		manifest.LicenseLine = lineNo
	}
}
