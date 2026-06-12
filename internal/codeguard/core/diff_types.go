package core

// ChangedLineRanges describes which lines of a file changed in a diff scan.
type ChangedLineRanges struct {
	// AllChanged marks files where every line counts as changed
	// (for example newly added files).
	AllChanged bool
	// Ranges holds inclusive [start, end] line ranges.
	Ranges [][2]int
}

// Contains reports whether the given 1-based line is part of the change.
func (c ChangedLineRanges) Contains(line int) bool {
	if c.AllChanged {
		return true
	}
	for _, r := range c.Ranges {
		if line >= r[0] && line <= r[1] {
			return true
		}
	}
	return false
}
