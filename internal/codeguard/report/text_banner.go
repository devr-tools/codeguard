package report

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
)

const fallbackTextBanner = `
  ____          _                         _
 / ___|___   __| | ___  __ _ _   _  __ _| |__   __| |
| |   / _ \ / _` + "`" + ` |/ _ \/ _` + "`" + ` | | | |/ _` + "`" + ` | '_ \ / _` + "`" + ` |
| |__| (_) | (_| |  __/ (_| | |_| | (_| | |_) | (_| |
 \____\___/ \__,_|\___|\__, |\__,_|\__,_|_.__/ \__,_|
                       |___/
`

const (
	brandNavy = "38;2;10;18;60"
	brandBlue = "38;2;37;169;255"
)

func brandBanner() string {
	logo, err := loadBannerAsset()
	if err != nil {
		return colorize(fallbackTextBanner, brandNavy)
	}
	banner := strings.TrimRight(string(logo), "\n")
	if noColor() {
		return banner + "\n"
	}
	lines := strings.Split(banner, "\n")
	for idx, line := range lines {
		lines[idx] = colorizeBannerLine(idx, line)
	}
	return strings.Join(lines, "\n") + "\n"
}

func loadBannerAsset() ([]byte, error) {
	paths := []string{"img/codeguard.txt"}
	if _, file, _, ok := runtime.Caller(0); ok {
		paths = append(paths, filepath.Join(filepath.Dir(file), "..", "..", "..", "img", "codeguard.txt"))
	}
	var firstErr error
	for _, path := range paths {
		logo, err := os.ReadFile(path) //nolint:gosec // fixed banner asset path, not user input
		if err == nil {
			return logo, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, firstErr
}

func colorizeBannerLine(lineIdx int, line string) string {
	var b strings.Builder
	runes := []rune(line)
	currentColor := ""
	for runeIdx, r := range runes {
		nextColor := bannerRuneColor(lineIdx, runeIdx, r)
		if nextColor != currentColor {
			if currentColor != "" {
				b.WriteString("\x1b[0m")
			}
			if nextColor != "" {
				b.WriteString("\x1b[" + nextColor + "m")
			}
			currentColor = nextColor
		}
		b.WriteRune(r)
	}
	if currentColor != "" {
		b.WriteString("\x1b[0m")
	}
	return b.String()
}

func bannerRuneColor(lineIdx int, runeIdx int, r rune) string {
	if r == '⠀' || unicode.IsSpace(r) {
		return ""
	}
	if isShieldAccent(lineIdx, runeIdx) || isWordmarkDAccent(lineIdx, runeIdx) {
		return brandBlue
	}
	return brandNavy
}

func isShieldAccent(lineIdx int, runeIdx int) bool {
	return lineIdx >= 4 && lineIdx <= 9 && runeIdx >= 12 && runeIdx <= 21
}

func isWordmarkDAccent(lineIdx int, runeIdx int) bool {
	switch lineIdx {
	case 4:
		return runeIdx >= 44 && runeIdx <= 46
	case 5, 6, 7, 8:
		return runeIdx >= 67 && runeIdx <= 88
	case 9:
		return runeIdx >= 55 && runeIdx <= 61
	case 10:
		return runeIdx >= 56 && runeIdx <= 58
	default:
		return false
	}
}
