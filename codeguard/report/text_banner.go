package report

import (
	_ "embed"
	"strings"
)

//go:embed codeguard.txt
var codeguardLogoANSI string

type bannerHighlight struct {
	line  int
	start int
	end   int
	color string
}

var codeguardDBannerHighlights = []bannerHighlight{
	{line: 4, start: 87, end: 88, color: ansiBannerBlue},
	{line: 5, start: 82, end: 88, color: ansiBannerBlue},
	{line: 6, start: 82, end: 89, color: ansiBannerBlue},
	{line: 7, start: 82, end: 89, color: ansiBannerBlue},
	{line: 8, start: 83, end: 89, color: ansiBannerBlue},
}

var jimmyShieldBannerHighlights = []bannerHighlight{
	{line: 5, start: 13, end: 23, color: ansiBannerBlue},
	{line: 6, start: 13, end: 23, color: ansiBannerBlue},
	{line: 7, start: 13, end: 23, color: ansiBannerBlue},
	{line: 8, start: 13, end: 23, color: ansiBannerBlue},
}

var codeguardBannerHighlights = append(
	append([]bannerHighlight{}, codeguardDBannerHighlights...),
	jimmyShieldBannerHighlights...,
)

func writeLogo(b *strings.Builder) {
	if !colorEnabled() {
		return
	}
	b.WriteString(colorizeBanner(codeguardLogoANSI))
	b.WriteString("\n\n")
}

func colorizeBanner(banner string) string {
	lines := strings.Split(banner, "\n")
	for _, highlight := range codeguardBannerHighlights {
		if highlight.line >= len(lines) {
			continue
		}
		lines[highlight.line] = colorizeLineRange(
			lines[highlight.line],
			highlight.start,
			highlight.end,
			highlight.color,
		)
	}
	return strings.Join(lines, "\n")
}

func colorizeLineRange(line string, start int, end int, code string) string {
	if code == "" || start < 0 || end < start {
		return line
	}

	runes := []rune(line)
	if start >= len(runes) {
		return line
	}
	if end >= len(runes) {
		end = len(runes) - 1
	}

	return string(runes[:start]) + code + string(runes[start:end+1]) + ansiReset + string(runes[end+1:])
}
