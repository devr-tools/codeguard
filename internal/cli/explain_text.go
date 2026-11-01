package cli

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode"
)

// writeExplainText renders explain output for a terminal. The leading block
// (id, title, section, level, execution model, language coverage,
// description, owasp, how to fix) keeps its long-standing line format.
func writeExplainText(stdout io.Writer, out explainOutput) {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\ntitle: %s\nsection: %s\nlevel: %s\n", out.ID, out.Title, out.Section, out.Level)
	confidence := out.Confidence
	if confidence == "" {
		confidence = "unspecified (set per finding; treated as medium)"
	}
	fmt.Fprintf(&b, "confidence: %s\n", confidence)
	fmt.Fprintf(&b, "execution model: %s\nlanguage coverage: %s\n%s\n", out.ExecutionModel, languageCoverageText(out.LanguageCoverage), out.Description)
	if out.OWASPCategory != "" {
		fmt.Fprintf(&b, "owasp: %s\n", out.OWASPCategory)
	}
	if strings.TrimSpace(out.HowToFix) != "" {
		fmt.Fprintf(&b, "how to fix: %s\n", out.HowToFix)
	}
	if len(out.Aliases) > 0 {
		fmt.Fprintf(&b, "aliases: %s\n", strings.Join(out.Aliases, ", "))
	}
	if out.Docs != "" {
		fmt.Fprintf(&b, "docs: %s\n", out.Docs)
	}
	writeExplainExamples(&b, out)
	writeExplainConfig(&b, out)
	writeExplainSuppression(&b, out)
	if len(out.Profiles) > 0 {
		fmt.Fprintf(&b, "\nprofiles that enable it: %s\n", strings.Join(out.Profiles, ", "))
	} else {
		b.WriteString("\nprofiles that enable it: none (enable it explicitly with the keys above)\n")
	}
	_, _ = io.WriteString(stdout, b.String())
}

func languageCoverageText(coverage explainLanguageCoverageOutput) string {
	switch coverage.Mode {
	case "fixed":
		return strings.Join(coverage.Languages, ", ")
	case "":
		return "repository-wide"
	default:
		return coverage.Mode
	}
}

func writeExplainExamples(b *strings.Builder, out explainOutput) {
	for _, example := range out.Examples {
		fmt.Fprintf(b, "\nexample (%s, %s):\n", example.Language, example.Path)
		b.WriteString("  bad:\n")
		b.WriteString(indentBlock(visibleText(example.Bad), "    "))
		b.WriteString("  good:\n")
		b.WriteString(indentBlock(visibleText(example.Good), "    "))
		if len(example.Overrides) > 0 {
			fmt.Fprintf(b, "  reproduce with: codeguard scan -set %s\n", strings.Join(example.Overrides, " -set "))
		}
	}
}

func writeExplainConfig(b *strings.Builder, out explainOutput) {
	if len(out.ConfigKeys) == 0 {
		return
	}
	if out.ConfigSource != "" {
		fmt.Fprintf(b, "\nconfig (values from %s):\n", out.ConfigSource)
	} else {
		b.WriteString("\nconfig:\n")
	}
	width := 0
	for _, key := range out.ConfigKeys {
		width = max(width, len(key.Key))
	}
	for _, key := range out.ConfigKeys {
		value := ""
		if out.ConfigSource != "" {
			value = " = " + configValueText(key)
		}
		fmt.Fprintf(b, "  %-*s  %s%s\n", width, key.Key, key.Role, value)
	}
	if out.Enabled != nil {
		state := "no"
		if *out.Enabled {
			state = "yes"
		}
		fmt.Fprintf(b, "  enabled under this config: %s\n", state)
	}
	if out.Enabled != nil && !*out.Enabled {
		if sets := enableSets(out.ConfigKeys); sets != "" {
			fmt.Fprintf(b, "  enable for one run: codeguard scan%s\n", sets)
		}
		return
	}
	if key := disableKey(out.ConfigKeys); key != "" {
		fmt.Fprintf(b, "  disable for one run: codeguard scan -set %s=false\n", key)
	}
	if out.SectionKey != "" && out.SectionKey != "custom" {
		fmt.Fprintf(b, "  disable the whole section: add %s to checks.disabled\n", out.SectionKey)
	}
}

// disableKey picks the most specific enable key: the last one listed, which
// is a rule-level switch when the rule has one and the section toggle
// otherwise.
func disableKey(keys []explainConfigKeyOutput) string {
	key := ""
	for _, candidate := range keys {
		if candidate.Role == "enable" {
			key = candidate.Key
		}
	}
	return key
}

// enableSets renders `-set key=true` for every enable key that is off in the
// resolved config: explicitly false, or an unset section toggle.
func enableSets(keys []explainConfigKeyOutput) string {
	var b strings.Builder
	for _, key := range keys {
		if key.Role != "enable" {
			continue
		}
		off := key.Value == false || (key.Unset && strings.Count(key.Key, ".") == 1)
		if off {
			b.WriteString(" -set ")
			b.WriteString(key.Key)
			b.WriteString("=true")
		}
	}
	return b.String()
}

func configValueText(key explainConfigKeyOutput) string {
	if key.Unset || key.Value == nil {
		return "(unset)"
	}
	value := reflect.ValueOf(key.Value)
	if value.Kind() == reflect.Slice {
		parts := make([]string, 0, value.Len())
		for i := 0; i < value.Len(); i++ {
			parts = append(parts, fmt.Sprint(value.Index(i).Interface()))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	if text, ok := key.Value.(string); ok {
		if text == "" {
			return `""`
		}
		return text
	}
	return fmt.Sprint(key.Value)
}

func writeExplainSuppression(b *strings.Builder, out explainOutput) {
	b.WriteString("\nsuppress:\n")
	fmt.Fprintf(b, "  inline (same or previous line): %s\n", out.Suppression.Inline)
	b.WriteString("  waiver (codeguard.yaml):\n")
	b.WriteString(indentBlock(out.Suppression.Waiver, "    "))
}

func indentBlock(text string, prefix string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	var b strings.Builder
	for _, line := range lines {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString(prefix)
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// visibleText makes invisible format characters (bidi controls, zero-width
// spaces, tag characters) visible as <U+XXXX> so an example for
// prompts.hidden-unicode shows what it is about.
func visibleText(text string) string {
	if !strings.ContainsFunc(text, isInvisibleFormatRune) {
		return text
	}
	var b strings.Builder
	for _, r := range text {
		if isInvisibleFormatRune(r) {
			fmt.Fprintf(&b, "<U+%04X>", r)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isInvisibleFormatRune(r rune) bool {
	return unicode.Is(unicode.Cf, r)
}
