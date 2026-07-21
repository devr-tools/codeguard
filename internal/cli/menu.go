package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/devr-tools/codeguard/internal/whatsnew"
)

// menuItem is a single command row in the menu.
type menuItem struct {
	name string
	desc string
}

// menuGroup is a titled cluster of related commands.
type menuGroup struct {
	title string
	items []menuItem
}

// menuGroups defines the command menu layout, grouped by task.
var menuGroups = []menuGroup{
	{
		title: "Get started",
		items: []menuItem{
			{"init", "Create a codeguard config file"},
			{"validate", "Check that a config file is valid"},
			{"profiles", "List built-in policy profiles"},
		},
	},
	{
		title: "Scan & baseline",
		items: []menuItem{
			{"scan", "Scan the working tree or a diff for violations"},
			{"scan-history", "Scan git history for committed secrets"},
			{"validate-patch", "Scan a unified diff piped on stdin"},
			{"baseline", "Record current findings as an accepted baseline"},
		},
	},
	{
		title: "Fix & report",
		items: []menuItem{
			{"fix", "AI-assisted fix for a specific finding"},
			{"fix-batch", "Verify explicit deterministic fixes as one patch"},
			{"report", "Print the slop / performance history report"},
		},
	},
	{
		title: "Rules & policy",
		items: []menuItem{
			{"rules", "List all rules and their metadata"},
			{"explain", "Explain a rule by id"},
			{"owasp", "Show OWASP category coverage"},
		},
	},
	{
		title: "Serve & diagnose",
		items: []menuItem{
			{"serve", "Run the Model Context Protocol (MCP) server"},
			{"doctor", "Diagnose config and environment"},
			{"version", "Print the codeguard version"},
		},
	},
}

// writeMenu renders the command menu: a short tagline, task-grouped commands
// with aligned descriptions, and a footer pointing at per-command help. Command
// names are styled in devr blue when w is a color-capable terminal.
func writeMenu(w io.Writer) {
	color := whatsnew.ColorForWriter(w)

	_, _ = fmt.Fprintln(w, "codeguard — code quality, design, security, CI, and prompt-safety checks for your repository.")
	_, _ = fmt.Fprintf(w, "\nUsage: %s %s %s\n\n",
		whatsnew.BlueBold("codeguard", color),
		whatsnew.Blue("<command>", color),
		whatsnew.Faint("[flags]", color))

	width := menuNameWidth()
	for _, group := range menuGroups {
		_, _ = fmt.Fprintf(w, "  %s\n", whatsnew.Faint(strings.ToUpper(group.title), color))
		for _, item := range group.items {
			_, _ = fmt.Fprintf(w, "    %s  %s\n", whatsnew.Blue(pad(item.name, width), color), item.desc)
		}
		_, _ = fmt.Fprintln(w)
	}

	_, _ = fmt.Fprintf(w, "  %s\n", whatsnew.Faint("Common flags", color))
	_, _ = fmt.Fprintf(w, "    %s  %s\n", whatsnew.Blue(pad("-config", width), color), "Path to a config file or directory")
	_, _ = fmt.Fprintf(w, "    %s  %s\n", whatsnew.Blue(pad("-profile", width), color), "startup | strict | enterprise | ai-safe")
	_, _ = fmt.Fprintf(w, "    %s  %s\n\n", whatsnew.Blue(pad("-format", width), color), "text | json | sarif | github")

	_, _ = fmt.Fprintf(w, "Run %s to see all flags for a command.\n",
		whatsnew.Blue("codeguard <command> -h", color))
}

// menuNameWidth returns the column width for command/flag names: the longest
// name plus padding, so descriptions align.
func menuNameWidth() int {
	longest := len("-profile") // keep the Common flags column aligned too.
	for _, group := range menuGroups {
		for _, item := range group.items {
			if len(item.name) > longest {
				longest = len(item.name)
			}
		}
	}
	return longest
}

func pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
