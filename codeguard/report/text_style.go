package report

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/devr-tools/codeguard/codeguard/core"
)

func styleStatusBadge(status core.Status) string {
	label := strings.ToUpper(string(status))
	if status == core.StatusPass {
		label = "✓ " + label
	}
	return colorize(statusColor(status), "["+label+"]")
}

func severityBullet(severity core.Severity) string {
	switch severity {
	case core.SeverityError:
		return colorize(ansiRed, "•")
	case core.SeverityWarn:
		return colorize(ansiYellow, "•")
	default:
		return colorize(ansiBlue, "•")
	}
}

func styleSummaryCount(status core.Status, count int) string {
	if status == core.StatusPass {
		return colorize(statusColor(status), fmt.Sprintf("✓ %d", count))
	}
	return colorize(statusColor(status), fmt.Sprintf("%d", count))
}

func styleHeader(text string) string {
	return colorize(ansiCyanBold, text)
}

func styleLabel(text string) string {
	return colorize(ansiBold, text)
}

func statusColor(status core.Status) string {
	switch status {
	case core.StatusPass:
		return ansiGreen
	case core.StatusWarn:
		return ansiYellow
	case core.StatusFail:
		return ansiRed
	case core.StatusSkip:
		return ansiBlue
	default:
		return ""
	}
}

func colorize(code string, text string) string {
	if code == "" || !colorEnabled() {
		return text
	}
	return code + text + ansiReset
}

func colorEnabled() bool {
	noColor := strings.TrimSpace(os.Getenv("NO_COLOR"))
	term := strings.TrimSpace(os.Getenv("TERM"))
	return noColor == "" && term != "dumb"
}

func printableWidth(text string) int {
	width := 0
	inEscape := false
	for _, r := range text {
		switch {
		case r == '\x1b':
			inEscape = true
		case inEscape && r == 'm':
			inEscape = false
		case inEscape:
			continue
		default:
			width += runeWidth(r)
		}
	}
	return width
}

func runeWidth(r rune) int {
	if r == utf8.RuneError {
		return 1
	}
	if r == '✓' {
		return 1
	}
	if r == 0xFE0F {
		return 0
	}
	return 1
}

const (
	ansiReset      = "\x1b[0m"
	ansiBold       = "\x1b[1m"
	ansiRed        = "\x1b[31m"
	ansiGreen      = "\x1b[32m"
	ansiYellow     = "\x1b[33m"
	ansiBlue       = "\x1b[34m"
	ansiCyanBold   = "\x1b[1;36m"
	ansiBannerBlue = "\x1b[38;2;19;156;254m"
)
