package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/devr-tools/codeguard/codeguard/core"
)

func Write(w io.Writer, result core.Report, format string) error {
	switch strings.TrimSpace(format) {
	case "", "text":
		_, err := io.WriteString(w, Text(result))
		return err
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	default:
		return fmt.Errorf("unsupported report format %q", format)
	}
}

func Text(result core.Report) string {
	var b strings.Builder

	writeLogo(&b)
	writeReportHeader(&b, result)
	writeSectionTable(&b, result.Sections)
	writeSectionDetails(&b, result.Sections)
	writeSummary(&b, result.Summary)

	return b.String()
}
