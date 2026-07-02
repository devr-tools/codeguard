package agentcontext

import "strings"

// fenceState tracks fenced code blocks while scanning a doc line by line.
// unreliable marks a command fence that switched directories or opened a
// heredoc, after which its remaining lines cannot be resolved safely.
type fenceState struct {
	open       bool
	info       string
	unreliable bool
}

// observe updates fence tracking for one line, returning true when the line
// is itself a fence delimiter.
func (f *fenceState) observe(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "```") && !strings.HasPrefix(trimmed, "~~~") {
		return false
	}
	if f.open {
		f.open, f.info, f.unreliable = false, "", false
		return true
	}
	f.open = true
	f.info = strings.ToLower(strings.TrimSpace(strings.Trim(trimmed, "`~")))
	if idx := strings.IndexAny(f.info, " \t"); idx >= 0 {
		f.info = f.info[:idx]
	}
	return true
}

func (f *fenceState) inFence() bool { return f.open }

// inCommandFence reports whether the scanner sits inside a still-reliable
// shell fence, the only fence kind whose lines are parsed as commands.
func (f *fenceState) inCommandFence() bool {
	if !f.open || f.unreliable {
		return false
	}
	switch f.info {
	case "bash", "sh", "shell", "zsh":
		return true
	default:
		return false
	}
}
