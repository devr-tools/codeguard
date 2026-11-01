package prompts

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// hiddenRuneClass ranks invisible characters by how directly they enable a
// prompt-injection: bidi controls reorder what a reviewer sees versus what
// the model reads, and tag characters smuggle whole invisible ASCII strings;
// zero-width characters hide or split words but carry no payload alone.
type hiddenRuneClass int

const (
	hiddenNone hiddenRuneClass = iota
	hiddenZeroWidth
	hiddenBidi
	hiddenTag
)

const blackFlagEmoji = 0x1F3F4

var hiddenRuneNames = map[rune]string{
	0x200B: "ZERO WIDTH SPACE", 0x200C: "ZERO WIDTH NON-JOINER", 0x200D: "ZERO WIDTH JOINER",
	0x2060: "WORD JOINER", 0x2061: "FUNCTION APPLICATION", 0x2062: "INVISIBLE TIMES",
	0x2063: "INVISIBLE SEPARATOR", 0x2064: "INVISIBLE PLUS", 0xFEFF: "ZERO WIDTH NO-BREAK SPACE", 0x180E: "MONGOLIAN VOWEL SEPARATOR",
	0x202A: "LEFT-TO-RIGHT EMBEDDING", 0x202B: "RIGHT-TO-LEFT EMBEDDING", 0x202C: "POP DIRECTIONAL FORMATTING",
	0x202D: "LEFT-TO-RIGHT OVERRIDE", 0x202E: "RIGHT-TO-LEFT OVERRIDE", 0x2066: "LEFT-TO-RIGHT ISOLATE",
	0x2067: "RIGHT-TO-LEFT ISOLATE", 0x2068: "FIRST STRONG ISOLATE", 0x2069: "POP DIRECTIONAL ISOLATE",
}

func classifyHiddenRune(r rune) hiddenRuneClass {
	switch {
	case r >= 0xE0000 && r <= 0xE007F:
		return hiddenTag
	case (r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069):
		return hiddenBidi
	case r == 0x200B || r == 0x200C || r == 0x200D || (r >= 0x2060 && r <= 0x2064) || r == 0xFEFF || r == 0x180E:
		return hiddenZeroWidth
	default:
		return hiddenNone
	}
}

// hiddenLineScan accumulates the invisible characters found on one line.
type hiddenLineScan struct {
	worst  hiddenRuneClass
	column int
	count  int
	runes  []rune
}

func (s *hiddenLineScan) add(r rune, class hiddenRuneClass, column int) {
	if s.count == 0 {
		s.column = column
	}
	s.count++
	if class > s.worst {
		s.worst = class
	}
	for _, seen := range s.runes {
		if seen == r {
			return
		}
	}
	s.runes = append(s.runes, r)
}

// hiddenUnicodeFindings reports each line of a governed prompt or agent file
// that contains invisible formatting characters. Legitimate uses are
// excluded: a leading byte-order mark, joiners inside emoji or non-Latin
// words, and tag characters that spell an emoji subdivision flag.
func hiddenUnicodeFindings(env support.Context, file string, data []byte) []core.Finding {
	findings := make([]core.Finding, 0)
	for idx, line := range splitLines(data) {
		scan := scanHiddenRunes(line, idx == 0)
		if scan.count == 0 {
			continue
		}
		level, confidence := "fail", core.ConfidenceHigh
		if scan.worst == hiddenZeroWidth {
			level, confidence = "warn", core.ConfidenceMedium
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     "prompts.hidden-unicode",
			Level:      level,
			Path:       file,
			Line:       idx + 1,
			Column:     scan.column,
			Message:    hiddenUnicodeMessage(scan),
			Confidence: confidence,
		}))
	}
	return findings
}

func scanHiddenRunes(line string, firstLine bool) hiddenLineScan {
	var scan hiddenLineScan
	var prev rune
	column := 0
	inFlagSequence := false
	for offset := 0; offset < len(line); {
		r, size := utf8.DecodeRuneInString(line[offset:])
		column++
		class := classifyHiddenRune(r)
		if class != hiddenTag {
			inFlagSequence = false
		}
		switch {
		case class == hiddenTag && (prev == blackFlagEmoji || inFlagSequence):
			inFlagSequence = r != 0xE007F
			class = hiddenNone
		case r == 0xFEFF && firstLine && offset == 0:
			class = hiddenNone
		case (r == 0x200C || r == 0x200D) && joinsNonASCII(prev, line[offset+size:]):
			class = hiddenNone
		}
		if class != hiddenNone {
			scan.add(r, class, column)
		}
		prev = r
		offset += size
	}
	return scan
}

// joinsNonASCII reports whether a joiner sits between two non-ASCII runes,
// as it does inside emoji sequences and Persian or Indic words.
func joinsNonASCII(prev rune, rest string) bool {
	next, _ := utf8.DecodeRuneInString(rest)
	return prev > utf8.RuneSelf && next > utf8.RuneSelf && next != utf8.RuneError
}

func hiddenUnicodeMessage(scan hiddenLineScan) string {
	names := make([]string, 0, 3)
	for _, r := range scan.runes {
		if len(names) == cap(names) {
			break
		}
		names = append(names, describeHiddenRune(r))
	}
	if len(scan.runes) > len(names) {
		names = append(names, fmt.Sprintf("and %d more", len(scan.runes)-len(names)))
	}
	reason := "can hide or split text the model reads but a reviewer cannot see"
	switch scan.worst {
	case hiddenBidi:
		reason = "can reorder the text a reviewer sees relative to what the model reads"
	case hiddenTag:
		reason = "can encode invisible ASCII instructions the model still reads"
	}
	return fmt.Sprintf("%d invisible Unicode character(s) (%s) %s; remove them", scan.count, strings.Join(names, ", "), reason)
}

func describeHiddenRune(r rune) string {
	if name, ok := hiddenRuneNames[r]; ok {
		return fmt.Sprintf("U+%04X %s", r, name)
	}
	return fmt.Sprintf("U+%04X TAG CHARACTER", r)
}
