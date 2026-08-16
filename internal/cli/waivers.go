package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func runWaivers(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || isHelpCommand(args[0]) {
		writeWaiversUsage(stdout)
		return 0
	}
	switch args[0] {
	case "audit":
		return runWaiversAudit(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown waivers command %q\n\n", args[0])
		writeWaiversUsage(stderr)
		return 1
	}
}

func runWaiversAudit(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("waivers audit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", service.DefaultConfigPath(), "config path")
	profile := fs.String("profile", "", "optional policy profile override")
	mode := fs.String("mode", string(service.ScanModeFull), "scan mode: full or diff")
	baseRef := fs.String("base-ref", "main", "base ref for diff scans")
	folderPath := fs.String("folder", "", "folder path to scan instead of all configured targets")
	pathAlias := fs.String("path", "", "alias for -folder")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	cfg, err := loadConfigWithProfile(*configPath, *profile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}
	scanMode, err := parseScanMode(*mode)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	targetPath, err := scanTargetPath(*folderPath, *pathAlias)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	report, err := service.RunWithOptions(context.Background(), cfg, service.ScanOptions{
		Mode:              scanMode,
		BaseRef:           strings.TrimSpace(*baseRef),
		TargetPath:        targetPath,
		EnableWaiverAudit: true,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "waiver audit scan: %v\n", err)
		return 1
	}
	audit := waiverAuditFromReport(report)
	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "", "text":
		return writeWaiverAuditText(stdout, audit)
	case "json":
		if audit == nil {
			audit = &service.WaiverAuditArtifact{}
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(audit); err != nil {
			_, _ = fmt.Fprintf(stderr, "write waiver audit: %v\n", err)
			return 1
		}
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "unsupported waiver audit format %q\n", *format)
		return 1
	}
}

func waiverAuditFromReport(report service.Report) *service.WaiverAuditArtifact {
	for _, artifact := range report.Artifacts {
		if artifact.WaiverAudit != nil {
			return artifact.WaiverAudit
		}
	}
	return nil
}

func writeWaiverAuditText(stdout io.Writer, audit *service.WaiverAuditArtifact) int {
	if audit == nil || len(audit.Waivers) == 0 {
		_, _ = fmt.Fprintln(stdout, "Waiver audit: no configured waivers")
		return 0
	}
	counts := map[string]int{}
	upgradeCounts := map[string]int{}
	for _, waiver := range audit.Waivers {
		counts[waiver.Status]++
		if waiver.UpgradeStatus != "" {
			upgradeCounts[waiver.UpgradeStatus]++
		}
	}
	_, _ = fmt.Fprintf(stdout, "Waiver audit: %d configured, %d active, %d unused, %d expired, %d unknown rule\n\n",
		len(audit.Waivers),
		counts[service.WaiverAuditStatusActive],
		counts[service.WaiverAuditStatusUnused],
		counts[service.WaiverAuditStatusExpired],
		counts[service.WaiverAuditStatusUnknownRule],
	)
	if upgradeCounts[service.WaiverAuditUpgradeStatusStaleAfterUpgrade] > 0 || upgradeCounts[service.WaiverAuditUpgradeStatusInconclusive] > 0 {
		_, _ = fmt.Fprintf(stdout, "Upgrade evidence: %d stale after upgrade, %d inconclusive\n\n",
			upgradeCounts[service.WaiverAuditUpgradeStatusStaleAfterUpgrade],
			upgradeCounts[service.WaiverAuditUpgradeStatusInconclusive],
		)
	}
	for _, waiver := range audit.Waivers {
		if waiver.Status == service.WaiverAuditStatusActive {
			continue
		}
		_, _ = fmt.Fprintf(stdout, "[WARN] waiver:%d %s", waiver.Index, waiver.Rule)
		if waiver.Path != "" {
			_, _ = fmt.Fprintf(stdout, " path=%s", waiver.Path)
		}
		_, _ = fmt.Fprintf(stdout, ": %s", waiverAuditStatusMessage(waiver))
		if waiver.UpgradeReason != "" {
			_, _ = fmt.Fprintf(stdout, " evidence=%q", waiver.UpgradeReason)
		}
		if waiver.Reason != "" {
			_, _ = fmt.Fprintf(stdout, " reason=%q", waiver.Reason)
		}
		_, _ = fmt.Fprintln(stdout)
	}
	if counts[service.WaiverAuditStatusUnused] == 0 &&
		counts[service.WaiverAuditStatusExpired] == 0 &&
		counts[service.WaiverAuditStatusUnknownRule] == 0 {
		_, _ = fmt.Fprintln(stdout, "[PASS] all waivers matched at least one finding in this scan")
	}
	return 0
}

func waiverAuditStatusMessage(waiver service.WaiverAuditEntry) string {
	if waiver.UpgradeStatus == service.WaiverAuditUpgradeStatusStaleAfterUpgrade {
		return "matched a finding on the previous CodeGuard version and no longer matches under the current version; review for removal"
	}
	if waiver.UpgradeStatus == service.WaiverAuditUpgradeStatusInconclusive {
		return "previously matched findings, but config or scan scope changed; review before treating it as fixed by upgrade"
	}
	switch waiver.Status {
	case service.WaiverAuditStatusUnused:
		return "did not match any finding in this scan; after an upgrade this may mean the underlying issue or false positive is fixed"
	case service.WaiverAuditStatusExpired:
		return fmt.Sprintf("expired on %s and no longer suppresses findings", waiver.ExpiresOn)
	case service.WaiverAuditStatusUnknownRule:
		return "references a rule that is not enabled in the current catalog"
	default:
		return waiver.Status
	}
}

func writeWaiversUsage(stdout io.Writer) {
	_, _ = fmt.Fprintln(stdout, `usage: codeguard waivers <command> [flags]

Commands:
  audit   Scan with waiver instrumentation and report stale waiver candidates

Examples:
  codeguard waivers audit -config codeguard.yaml
  codeguard waivers audit -folder ./service/api -format json`)
}
