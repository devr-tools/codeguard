package report

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/codeguard/core"
)

func writeReportHeader(b *strings.Builder, result core.Report) {
	fmt.Fprintf(b, "%s %s\n", styleHeader("CodeGuard Report"), result.Name)
	fmt.Fprintf(b, "%s %s\n\n", styleLabel("Generated:"), result.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z"))
	if result.ScanMode == "" {
		return
	}
	fmt.Fprintf(b, "%s %s\n", styleLabel("Scan Mode:"), result.ScanMode)
	if result.BaseRef != "" {
		fmt.Fprintf(b, "%s %s\n", styleLabel("Base Ref:"), result.BaseRef)
	}
	b.WriteByte('\n')
}

func writeSectionDetails(b *strings.Builder, sections []core.SectionResult) {
	for _, section := range sections {
		writeSectionDetail(b, section)
	}
}

func writeSectionDetail(b *strings.Builder, section core.SectionResult) {
	fmt.Fprintf(b, "%s %s\n", styleStatusBadge(section.Status), section.Name)
	if section.Note != "" {
		fmt.Fprintf(b, "  %s\n", section.Note)
	}
	for _, finding := range section.Findings {
		writeFinding(b, finding)
	}
	b.WriteByte('\n')
}

func writeFinding(b *strings.Builder, finding core.Finding) {
	prefix := severityBullet(finding.Severity)
	if finding.Path != "" {
		fmt.Fprintf(b, "  %s %s: %s (%s)\n", prefix, finding.Path, finding.Message, finding.Severity)
		return
	}
	fmt.Fprintf(b, "  %s %s (%s)\n", prefix, finding.Message, finding.Severity)
}

func writeSummary(b *strings.Builder, summary core.Summary) {
	fmt.Fprintf(
		b,
		"%s %s  %s %s  %s %s  %s %s\n",
		styleSummaryCount(core.StatusPass, summary.PassedSections),
		styleLabel("pass"),
		styleSummaryCount(core.StatusWarn, summary.WarnedSections),
		styleLabel("warn"),
		styleSummaryCount(core.StatusFail, summary.FailedSections),
		styleLabel("fail"),
		styleSummaryCount(core.StatusSkip, summary.SkippedSections),
		styleLabel("skip"),
	)
}
