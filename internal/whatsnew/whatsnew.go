// Package whatsnew renders codeguard's "What's New" banner: the current
// version, the latest release available upstream (when a cached update check
// has one), and a few highlights parsed from the embedded CHANGELOG.md.
package whatsnew

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// maxHighlights caps how many changelog bullets the banner shows so it stays a
// glance-able banner rather than a full changelog dump.
const maxHighlights = 5

// Release describes a single changelog entry.
type Release struct {
	Version    string
	Date       string
	Highlights []string
}

var (
	// releaseHeading matches a release-please version heading, e.g.
	// "## [0.6.1](https://…/compare/v0.6.0...v0.6.1) (2026-07-01)".
	releaseHeading = regexp.MustCompile(`^##\s+\[([0-9][^\]]*)\][^)]*\)(?:\s+\((\d{4}-\d{2}-\d{2})\))?`)
	// bulletPrefix matches a markdown list item marker.
	bulletPrefix = regexp.MustCompile(`^[-*]\s+`)
	// markdownLink strips trailing commit/PR reference links such as
	// " ([34c7f87](https://…))" that release-please appends to each bullet.
	markdownLink = regexp.MustCompile(`\s*\(\[[^\]]+\]\([^)]*\)\)`)
	// boldMarker removes markdown bold emphasis around conventional-commit
	// scopes, turning "**security:**" into "security:".
	boldMarker = regexp.MustCompile(`\*\*`)
)

// LatestFromChangelog parses the most recent release section out of a
// conventional-changelog document and returns its version, date, and a
// de-duplicated list of highlight bullets. ok is false when no release heading
// is found.
func LatestFromChangelog(markdown string) (Release, bool) {
	lines := strings.Split(markdown, "\n")

	var rel Release
	found := false
	seen := make(map[string]bool)

	for _, line := range lines {
		if m := releaseHeading.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			if found {
				break // reached the previous release; stop collecting.
			}
			found = true
			rel.Version = m[1]
			rel.Date = m[2]
			continue
		}
		if !found {
			continue
		}
		if len(rel.Highlights) >= maxHighlights {
			continue
		}
		if bullet, ok := cleanBullet(line); ok && !seen[bullet] {
			seen[bullet] = true
			rel.Highlights = append(rel.Highlights, bullet)
		}
	}

	return rel, found
}

// cleanBullet turns a raw markdown bullet line into display text, returning
// ok=false for lines that are not bullets.
func cleanBullet(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !bulletPrefix.MatchString(trimmed) {
		return "", false
	}
	trimmed = bulletPrefix.ReplaceAllString(trimmed, "")
	trimmed = markdownLink.ReplaceAllString(trimmed, "")
	trimmed = boldMarker.ReplaceAllString(trimmed, "")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}

// NormalizeVersion strips a leading "v" and surrounding whitespace so versions
// from git tags ("v0.6.1") and manifests ("0.6.1") compare equal.
func NormalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// CompareVersions compares two dotted numeric versions, returning -1 if a < b,
// 0 if equal, and 1 if a > b. Non-numeric or pre-release suffixes on a segment
// are ignored for ordering; a version with more segments sorts higher when the
// shared prefix is equal (1.2 < 1.2.1).
func CompareVersions(a, b string) int {
	as := strings.Split(NormalizeVersion(a), ".")
	bs := strings.Split(NormalizeVersion(b), ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		av := segmentValue(as, i)
		bv := segmentValue(bs, i)
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	return 0
}

func segmentValue(segments []string, i int) int {
	if i >= len(segments) {
		return 0
	}
	seg := segments[i]
	// Trim any pre-release/build suffix, e.g. "1-rc1" -> "1".
	if idx := strings.IndexAny(seg, "-+"); idx >= 0 {
		seg = seg[:idx]
	}
	n, err := strconv.Atoi(seg)
	if err != nil {
		return 0
	}
	return n
}

// UpdateAvailable reports whether latest is a strictly newer version than
// current. It returns false when either value is empty.
func UpdateAvailable(current, latest string) bool {
	if strings.TrimSpace(current) == "" || strings.TrimSpace(latest) == "" {
		return false
	}
	return CompareVersions(latest, current) > 0
}

// Render writes the What's New banner to w. current is the running version,
// latest is the newest available version (may be empty when unknown/offline),
// rel holds the highlights to display, and color enables devr-blue ANSI
// styling (use ColorForWriter to decide). When there is nothing to show
// (no version and no highlights) Render writes nothing.
func Render(w io.Writer, current string, latest string, rel Release, color bool) {
	current = strings.TrimSpace(current)
	if current == "" && len(rel.Highlights) == 0 {
		return
	}

	const rule = "────────────────────────────────────────────────────────────"
	frame := paint(color, devrBlue, "╭"+rule+"╮")
	_, _ = fmt.Fprintln(w, frame)

	header := "codeguard"
	if current != "" {
		header = "codeguard v" + NormalizeVersion(current)
	}
	if UpdateAvailable(current, latest) {
		notice := paint(color, devrBlueBold, header) +
			"  " + paint(color, devrBlue, "→ update available: v"+NormalizeVersion(latest))
		_, _ = fmt.Fprintf(w, "  %s\n", notice)
		hint := "upgrade: go install github.com/devr-tools/codeguard/cmd/codeguard@latest"
		_, _ = fmt.Fprintf(w, "  %s\n", paint(color, dim, hint))
	} else {
		_, _ = fmt.Fprintf(w, "  %s %s\n", paint(color, devrBlueBold, header), paint(color, dim, "(latest)"))
	}

	if len(rel.Highlights) > 0 {
		heading := "What's new"
		if rel.Version != "" {
			heading = "What's new in v" + NormalizeVersion(rel.Version)
		}
		if rel.Date != "" {
			heading += fmt.Sprintf(" (%s)", rel.Date)
		}
		_, _ = fmt.Fprintf(w, "  %s\n", paint(color, devrBlue, heading+":"))
		for _, h := range rel.Highlights {
			_, _ = fmt.Fprintf(w, "    %s %s\n", paint(color, devrBlue, "•"), h)
		}
	}

	_, _ = fmt.Fprintln(w, paint(color, devrBlue, "╰"+rule+"╯"))
}
