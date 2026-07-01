package whatsnew

import (
	"io"
	"os"
	"strings"
)

// Brand ANSI SGR codes. devrBlue matches codeguard's brand blue (RGB
// 37,169,255) used by the report banner (internal/codeguard/report).
const (
	devrBlue     = "38;2;37;169;255"
	devrBlueBold = "1;38;2;37;169;255"
	dim          = "2"
	reset        = "\x1b[0m"
)

// ColorForWriter reports whether ANSI color should be emitted to w: only when
// w is a terminal (char device), NO_COLOR is unset, and TERM is not "dumb".
// Writing to a pipe, file, or bytes.Buffer yields false, so redirected output
// and tests stay plain text.
func ColorForWriter(w io.Writer) bool {
	if strings.TrimSpace(os.Getenv("NO_COLOR")) != "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// paint wraps s in the given SGR code when color is enabled, otherwise returns
// s unchanged.
func paint(color bool, code, s string) string {
	if !color || code == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + reset
}

// Blue renders s in codeguard's devr blue when color is enabled.
func Blue(s string, color bool) string { return paint(color, devrBlue, s) }

// BlueBold renders s in bold devr blue when color is enabled.
func BlueBold(s string, color bool) string { return paint(color, devrBlueBold, s) }

// Faint renders s dimmed when color is enabled.
func Faint(s string, color bool) string { return paint(color, dim, s) }
