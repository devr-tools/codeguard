package performance

import "strings"

func consumeBraceLoopLine(depth int, loops []int, line string, startsLoop bool) (int, []int) {
	next := depth + strings.Count(line, "{") - strings.Count(line, "}")
	if startsLoop && next > depth {
		loops = append(loops, depth)
	}
	for len(loops) > 0 && next <= loops[len(loops)-1] {
		loops = loops[:len(loops)-1]
	}
	return next, loops
}
